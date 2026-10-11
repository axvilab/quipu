package claudecode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// UserSettings is the profile's settings.json.
func UserSettings(configDir string) string {
	return filepath.Join(configDir, "settings.json")
}

// SettingsValue returns the raw value of a top-level key of a settings file.
func SettingsValue(settings []byte, key string) (json.RawMessage, bool, error) {
	if len(bytes.TrimSpace(settings)) == 0 {
		return nil, false, nil
	}
	_, values, err := topLevel(settings)
	if err != nil {
		return nil, false, err
	}
	v, ok := values[key]
	return v, ok, nil
}

// SetSettingsValue sets (or with a nil value removes) a top-level key of a settings file, keeping
// the other keys and their order. The file is re-indented with two spaces.
func SetSettingsValue(settings []byte, key string, value json.RawMessage) ([]byte, error) {
	if len(bytes.TrimSpace(settings)) == 0 {
		settings = []byte("{}")
	}
	keys, values, err := topLevel(settings)
	if err != nil {
		return nil, err
	}
	if _, ok := values[key]; !ok && value != nil {
		keys = append(keys, key)
	}
	values[key] = value

	var out bytes.Buffer
	out.WriteString("{\n")
	var kept []string
	for _, k := range keys {
		if values[k] != nil {
			kept = append(kept, k)
		}
	}
	for i, k := range kept {
		var v bytes.Buffer
		if err := json.Indent(&v, values[k], "  ", "  "); err != nil {
			return nil, err
		}
		name, _ := json.Marshal(k)
		fmt.Fprintf(&out, "  %s: %s", name, v.Bytes())
		if i < len(kept)-1 {
			out.WriteString(",")
		}
		out.WriteString("\n")
	}
	out.WriteString("}\n")
	return out.Bytes(), nil
}

// topLevel reads a JSON object and returns its keys in file order with their raw values.
func topLevel(data []byte) ([]string, map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, nil, errors.New("not a JSON object")
	}
	var keys []string
	values := map[string]json.RawMessage{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		k, _ := tok.(string)
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, err
		}
		if _, dup := values[k]; !dup {
			keys = append(keys, k)
		}
		values[k] = raw
	}
	return keys, values, nil
}

// hookGroup is one entry of settings.hooks.<Event>; unknown fields of other tools' groups are kept
// because groups are handled as raw JSON, this type only reads them.
type hookGroup struct {
	Matcher string `json:"matcher,omitempty"`
	Hooks   []struct {
		Type    string `json:"type"`
		Command string `json:"command"`
		Timeout int    `json:"timeout,omitempty"`
	} `json:"hooks"`
}

// HasHook reports whether any hook command in the settings contains marker.
func HasHook(settings []byte, marker string) bool {
	events, err := hookEvents(settings)
	if err != nil {
		return false
	}
	for _, groups := range events {
		for _, g := range groups {
			if groupHas(g, marker) {
				return true
			}
		}
	}
	return false
}

// WithHook adds command as a hook for each event (after removing earlier entries with marker),
// keeping every other hook and setting.
func WithHook(settings []byte, events []string, command, marker string) ([]byte, error) {
	all, err := hookEvents(settings)
	if err != nil {
		return nil, err
	}
	removeMarked(all, marker)
	group, _ := json.Marshal(map[string]any{
		"hooks": []map[string]any{{"type": "command", "command": command, "timeout": 5}},
	})
	for _, e := range events {
		all[e] = append(all[e], group)
	}
	return setHooks(settings, all)
}

// WithoutHook removes every hook group whose command contains marker.
func WithoutHook(settings []byte, marker string) ([]byte, error) {
	all, err := hookEvents(settings)
	if err != nil {
		return nil, err
	}
	removeMarked(all, marker)
	return setHooks(settings, all)
}

func hookEvents(settings []byte) (map[string][]json.RawMessage, error) {
	raw, ok, err := SettingsValue(settings, "hooks")
	if err != nil {
		return nil, err
	}
	all := map[string][]json.RawMessage{}
	if ok {
		if err := json.Unmarshal(raw, &all); err != nil {
			return nil, fmt.Errorf("settings.hooks: %w", err)
		}
	}
	return all, nil
}

func removeMarked(all map[string][]json.RawMessage, marker string) {
	for e, groups := range all {
		var kept []json.RawMessage
		for _, g := range groups {
			if !groupHas(g, marker) {
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 {
			delete(all, e)
		} else {
			all[e] = kept
		}
	}
}

func groupHas(raw json.RawMessage, marker string) bool {
	var g hookGroup
	if json.Unmarshal(raw, &g) != nil {
		return false
	}
	for _, h := range g.Hooks {
		if strings.Contains(h.Command, marker) {
			return true
		}
	}
	return false
}

func setHooks(settings []byte, all map[string][]json.RawMessage) ([]byte, error) {
	if len(all) == 0 {
		return SetSettingsValue(settings, "hooks", nil)
	}
	b, err := json.Marshal(all) // events sorted by name
	if err != nil {
		return nil, err
	}
	return SetSettingsValue(settings, "hooks", b)
}
