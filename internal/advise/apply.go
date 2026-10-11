package advise

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrStale means the file no longer looks the way the finding saw it; the advisor has to run
// again before the fix can be applied.
var ErrStale = errors.New("the file changed since the finding was made")

// Edit is a mechanical change to one file. Exactly one of the three forms is used:
//   - Old/New: replace the single occurrence of Old with New;
//   - Append: add text at the end of the file (created if missing), after a blank line;
//   - JSONKey/JSONSet: in a JSON object file, set entries of the top-level object JSONKey
//     (created if missing), keeping all other keys and their order.
type Edit struct {
	Path    string
	Old     string
	New     string
	Append  string
	JSONKey string
	JSONSet map[string]string
}

// ID identifies a finding across runs, as long as its rule, title and file stay the same.
func (f Finding) ID() string {
	sum := sha256.Sum256([]byte(f.Rule + "\x00" + f.Title + "\x00" + f.Path))
	return hex.EncodeToString(sum[:8])
}

// Apply returns the new content of the file for this edit, given its current content.
// It fails with ErrStale when the current content does not match what the edit expects.
func (e Edit) Apply(current []byte) ([]byte, error) {
	if e.JSONKey != "" {
		return setJSONObject(current, e.JSONKey, e.JSONSet)
	}
	if e.Append != "" {
		s := strings.TrimRight(string(current), "\n")
		if s != "" {
			s += "\n\n"
		}
		return []byte(s + e.Append), nil
	}
	if strings.Count(string(current), e.Old) != 1 {
		return nil, ErrStale
	}
	return []byte(strings.Replace(string(current), e.Old, e.New, 1)), nil
}

// setJSONObject merges set into the object under key at the top level of a JSON object.
// Top-level key order is preserved; the file is re-indented with two spaces.
func setJSONObject(current []byte, key string, set map[string]string) ([]byte, error) {
	if len(bytes.TrimSpace(current)) == 0 {
		current = []byte("{}")
	}
	keys, values, err := topLevel(current)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStale, err)
	}

	inner := map[string]json.RawMessage{}
	if raw, ok := values[key]; ok {
		if err := json.Unmarshal(raw, &inner); err != nil {
			return nil, fmt.Errorf("%w: %s is not an object", ErrStale, key)
		}
	} else {
		keys = append(keys, key)
	}
	for k, v := range set {
		b, _ := json.Marshal(v)
		inner[k] = b
	}
	merged, err := json.Marshal(inner) // encoding/json sorts map keys: stable output
	if err != nil {
		return nil, err
	}
	values[key] = merged

	var out bytes.Buffer
	out.WriteString("{\n")
	for i, k := range keys {
		var v bytes.Buffer
		if err := json.Indent(&v, values[k], "  ", "  "); err != nil {
			return nil, err
		}
		name, _ := json.Marshal(k)
		fmt.Fprintf(&out, "  %s: %s", name, v.Bytes())
		if i < len(keys)-1 {
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
