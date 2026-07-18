// Package packs defines condition-oriented tag packs for the check-in picker.
// Packs only control which catalog keys appear; HealthLog.Tags stay CSV keys.
package packs

import (
	"sort"
	"strings"
)

// Pack keys (stable API / storage ids).
const (
	PackGeneral    = "general"
	PackStenosis   = "stenosis"
	PackDiabetes   = "diabetes"
	PackUC         = "uc"
	PackHeart      = "heart"
	PackSleepApnea = "sleep-apnea"
)

// DefaultEnabledPacks is applied for new users and empty DB defaults.
// Stenosis matches the product focus; other condition packs are opt-in.
const DefaultEnabledPacks = PackStenosis

// Pack is a named group of catalog tag keys.
type Pack struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	AlwaysOn    bool     `json:"always_on"`
	TagKeys     []string `json:"tag_keys"`
}

// Catalog is the built-in pack set.
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
		{
			Key:         PackUC,
			Label:       "UC / IBD",
			Description: "Ulcerative colitis and gut flare tags",
			AlwaysOn:    false,
			TagKeys: []string{
				"uc-flare",
				"abdominal-pain", "urgency", "diarrhea", "blood-stool",
				"bloating", "nausea", "mucus", "night-stools", "bathroom-trips",
			},
		},
		{
			Key:         PackHeart,
			Label:       "Heart",
			Description: "Blood pressure, chest, rhythm, and heart symptoms",
			AlwaysOn:    false,
			TagKeys: []string{
				"bp-high", "bp-ok",
				"chest-pain", "chest-tightness", "palpitations", "heart-racing",
				"shortness-of-breath", "dizziness", "ankle-swelling",
			},
		},
		{
			Key:         PackSleepApnea,
			Label:       "Sleep apnea",
			Description: "Sleep quality, headaches, tiredness, and apnea signs",
			AlwaysOn:    false,
			TagKeys: []string{
				"morning-headache", "headache", "daytime-tired", "unrefreshing-sleep",
				"snoring", "gasping", "dry-mouth", "brain-fog",
				"restless-sleep", "naps", "insomnia",
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

// TagKeysForPack returns the tag key set for one pack (empty if unknown).
func TagKeysForPack(packKey string) map[string]struct{} {
	p, ok := ByKey(packKey)
	if !ok {
		return nil
	}
	m := make(map[string]struct{}, len(p.TagKeys))
	for _, tk := range p.TagKeys {
		m[tk] = struct{}{}
	}
	return m
}

// LogHasAnyTag reports whether CSV tags include any key in the set.
// Empty set or empty tags → false (untagged never matches a pack filter).
func LogHasAnyTag(tagsCSV string, keys map[string]struct{}) bool {
	if len(keys) == 0 {
		return false
	}
	for _, t := range strings.Split(tagsCSV, ",") {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := keys[t]; ok {
			return true
		}
	}
	return false
}

// FilterPackOptions returns optional (non-always-on) packs for summary filters.
// If enabled is non-nil, only those keys (plus nothing always-on) are returned.
func FilterPackOptions(enabled []string) []Pack {
	enSet := map[string]struct{}{}
	for _, k := range NormalizeEnabled(enabled) {
		enSet[k] = struct{}{}
	}
	// If empty enabled list, still allow filtering by any optional pack that exists
	// (summary uses patient's enabled packs; pass them in).
	var out []Pack
	for _, p := range Catalog() {
		if p.AlwaysOn {
			continue
		}
		if len(enSet) > 0 {
			if _, on := enSet[p.Key]; !on {
				continue
			}
		}
		out = append(out, p)
	}
	return out
}
