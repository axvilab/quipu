package advise

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/axvilab/quipu/internal/claudecode"
)

// ErrStale means the file no longer looks the way the finding saw it; the advisor has to run
// again before the fix can be applied.
var ErrStale = errors.New("the file changed since the finding was made")

// Edit is a mechanical change to one file. Exactly one of the forms is used:
//   - Old/New: replace the single occurrence of Old with New;
//   - Append: add text at the end of the file (created if missing), after a blank line;
//   - Create: write a new file with this content; refused if the file already has content;
//   - Delete: remove the file, which must still be exactly Old; folders it leaves empty are
//     removed up to, not including, Stop;
//   - JSONKey/JSONSet: in a JSON object file, set entries of the top-level object JSONKey
//     (created if missing), keeping all other keys and their order.
type Edit struct {
	Path    string
	Old     string
	New     string
	Append  string
	Create  string
	Delete  bool
	Stop    string
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
	if e.Delete {
		if string(current) != e.Old {
			return nil, ErrStale
		}
		return nil, nil // the caller removes the file
	}
	if e.Create != "" {
		if len(current) > 0 {
			return nil, ErrStale
		}
		return []byte(e.Create), nil
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
	raw, ok, err := claudecode.SettingsValue(current, key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStale, err)
	}
	inner := map[string]json.RawMessage{}
	if ok {
		if err := json.Unmarshal(raw, &inner); err != nil {
			return nil, fmt.Errorf("%w: %s is not an object", ErrStale, key)
		}
	}
	for k, v := range set {
		b, _ := json.Marshal(v)
		inner[k] = b
	}
	merged, err := json.Marshal(inner) // encoding/json sorts map keys: stable output
	if err != nil {
		return nil, err
	}
	return claudecode.SetSettingsValue(current, key, merged)
}
