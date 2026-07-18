// Package packs defines condition-oriented tag packs for the check-in picker.
// Packs only control which catalog keys appear; HealthLog.Tags stay CSV keys.
package packs

import (
	"sort"
	"strings"
)

// Pack keys (stable API / storage ids).
const (
	PackGeneral  = "general"
	PackStenosis = "stenosis"
	PackDiabetes = "diabetes"
)

// DefaultEnabledPacks is applied for new users and empty DB defaults.
// Stenosis matches the product focus; diabetes is opt-in.
const DefaultEnabledPacks = PackStenosis

// Pack is a named group of catalog tag keys.
type Pack struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	AlwaysOn    bool     `json:"always_on"`
	TagKeys     []string `json:"tag_keys"`
}

// Catalog is the built-in pack set for v1.
func Catalog() []Pack {
	return []Pack{
		{
			Key:         PackGeneral,
			Label:       "General",
			Description: "Always on — left/right and shared basics",
			AlwaysOn:    true,
			TagKeys: []string{
				"left", "right", "both-sides",
			},
		},
		{
			Key:         PackStenosis,
			Label:       "Stenosis / spine",
			Description: "Body regions, nerve sensations, walking and stenosis",
			AlwaysOn:    false,
			TagKeys: []string{
				"lower-back", "hips", "glute", "leg", "thigh", "calf", "foot",
				"numbing", "pins-needles", "tingling", "burning",
				"sharp-pain", "dull-ache", "radiating", "cramping",
				"weakness", "stiffness", "limping",
				"stenosis",
			},
		},
		{
			Key:         PackDiabetes,
			Label:       "Diabetes",
			Description: "Glucose high/low check-in tags",
			AlwaysOn:    false,
			TagKeys: []string{
				"glucose-high", "glucose-low",
			},
		},
	}
}

// ByKey returns a pack definition or false.
func ByKey(key string) (Pack, bool) {
	for _, p := range Catalog() {
		if p.Key == key {
			return p, true
		}
	}
	return Pack{}, false
}

// OptionalKeys returns non-always-on pack keys.
func OptionalKeys() []string {
	var out []string
	for _, p := range Catalog() {
		if !p.AlwaysOn {
			out = append(out, p.Key)
		}
	}
	return out
}

// NormalizeEnabled cleans a user-supplied pack list (CSV or slice).
// Always-on packs are stripped from storage (they're implied).
// Unknown keys are dropped. Empty result is valid (general only).
func NormalizeEnabled(raw []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, k := range raw {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || k == PackGeneral {
			continue
		}
		p, ok := ByKey(k)
		if !ok || p.AlwaysOn {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ParseEnabledCSV parses user.EnabledPacks storage.
func ParseEnabledCSV(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	return NormalizeEnabled(parts)
}

// FormatEnabledCSV stores optional packs as CSV.
func FormatEnabledCSV(keys []string) string {
	return strings.Join(NormalizeEnabled(keys), ",")
}

// EffectiveKeys returns the union of tag keys for always-on + enabled packs.
func EffectiveKeys(enabled []string) map[string]struct{} {
	enabled = NormalizeEnabled(enabled)
	enSet := map[string]struct{}{}
	for _, k := range enabled {
		enSet[k] = struct{}{}
	}
	out := map[string]struct{}{}
	for _, p := range Catalog() {
		if p.AlwaysOn {
			for _, tk := range p.TagKeys {
				out[tk] = struct{}{}
			}
			continue
		}
		if _, on := enSet[p.Key]; on {
			for _, tk := range p.TagKeys {
				out[tk] = struct{}{}
			}
		}
	}
	return out
}

// TagKeyPacks maps a tag key → pack keys that include it (for admin later).
func TagKeyPacks() map[string][]string {
	m := map[string][]string{}
	for _, p := range Catalog() {
		for _, tk := range p.TagKeys {
			m[tk] = append(m[tk], p.Key)
		}
	}
	return m
}

// AssignedSystemKeys is every system catalog key claimed by a pack.
func AssignedSystemKeys() map[string]struct{} {
	m := map[string]struct{}{}
	for _, p := range Catalog() {
		for _, tk := range p.TagKeys {
			m[tk] = struct{}{}
		}
	}
	return m
}
