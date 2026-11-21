package tests

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/l5s1/health-registry/internal/router"
	"github.com/stretchr/testify/require"
)

func newTestRouter(t *testing.T, allowed []string) *httptest.Server {
	t.Helper()
	r := router.New(router.Deps{AllowedOrigins: allowed})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func TestCORSAcceptsAllowedOrigin(t *testing.T) {
	srv := newTestRouter(t, []string{"http://localhost:8080"})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/healthz", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "http://localhost:8080", res.Header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "true", res.Header.Get("Access-Control-Allow-Credentials"))
}

func TestCORSDeniesUnknownOrigin(t *testing.T) {
	srv := newTestRouter(t, []string{"http://localhost:8080"})

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Empty(t, res.Header.Get("Access-Control-Allow-Origin"))
	require.Empty(t, res.Header.Get("Access-Control-Allow-Credentials"))
}

func TestCORSPreflightAllowedOrigin(t *testing.T) {
	srv := newTestRouter(t, []string{"https://l5s1.com"})

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/auth/login/begin", nil)
	req.Header.Set("Origin", "https://l5s1.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusNoContent, res.StatusCode)
	require.Equal(t, "https://l5s1.com", res.Header.Get("Access-Control-Allow-Origin"))
	require.NotEmpty(t, res.Header.Get("Access-Control-Allow-Methods"))
}

func TestCORSPreflightUnknownOrigin(t *testing.T) {
	srv := newTestRouter(t, []string{"https://l5s1.com"})

	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/auth/login/begin", nil)
	req.Header.Set("Origin", "https://evil.example")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusNoContent, res.StatusCode)
	require.Empty(t, res.Header.Get("Access-Control-Allow-Origin"))
}

func TestSecurityHeadersPresent(t *testing.T) {
	srv := newTestRouter(t, []string{"http://localhost:8080"})

	res, err := http.Get(srv.URL + "/api/healthz")
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	require.Equal(t, "DENY", res.Header.Get("X-Frame-Options"))
	require.Contains(t, res.Header.Get("Content-Security-Policy"), "default-src 'self'")
	require.NotEmpty(t, res.Header.Get("Referrer-Policy"))
}
