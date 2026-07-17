// Package codes handles 8-digit invite / device-link codes (display: xxxx-xxxx).
package codes

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

// DigitLen is the canonical stored length (no hyphen).
const DigitLen = 8

// Normalize strips spaces/hyphens and keeps digits only.
func Normalize(raw string) string {
	var b strings.Builder
	b.Grow(DigitLen)
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Valid reports whether raw normalizes to exactly 8 digits.
func Valid(raw string) bool {
	return len(Normalize(raw)) == DigitLen
}

// FormatDisplay returns "xxxx-xxxx" for storage form, or the original trimmed string if invalid.
func FormatDisplay(raw string) string {
	n := Normalize(raw)
	if len(n) != DigitLen {
		return strings.TrimSpace(raw)
	}
	return n[:4] + "-" + n[4:]
}

// Generate returns a cryptographically random 8-digit code (leading zeros allowed).
func Generate() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}
