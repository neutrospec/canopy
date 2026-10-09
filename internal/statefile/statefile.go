// Package statefile patches known JSON fields against the latest state under
// a file lock. Raw JSON retains unknown fields and exact numbers at every
// object level. The caller owns the wiki lease (shared, or exclusive for a
// larger mutation); this package never recursively acquires it.
package statefile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/neutrospec/canopy/internal/safefile"
	"github.com/neutrospec/canopy/internal/wikilock"
)

// Decode rejects null/arrays as state roots instead of silently resetting them.
func Decode(data []byte, dst any) error {
	if _, ok := object(data); !ok {
		return fmt.Errorf("state must be a JSON object")
	}
	return json.Unmarshal(data, dst)
}

// Snapshot records the known representation at load time. All callers pass
// JSON-only state structs, so a marshal failure is a programming error.
func Snapshot(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Patch applies only changes since before. Disjoint changes merge; a stale
// conflicting leaf is refused before publication, preserving the input state.
// validate checks the latest document, not merely the caller's old snapshot.
func Patch(path string, before []byte, next any, validate func([]byte) error) error {
	g, err := wikilock.AcquireFile(context.Background(), path)
	if err != nil {
		return err
	}
	defer g.Unlock()
	current, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		current = []byte("{}")
	} else if err != nil {
		return err
	}
	if _, ok := object(current); !ok {
		return fmt.Errorf("%s: state must be a JSON object", path)
	}
	if validate != nil && string(current) != "{}" {
		if err := validate(current); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if len(before) == 0 {
		before = []byte("{}")
	}
	after, err := json.Marshal(next)
	if err != nil {
		return err
	}
	patched, err := merge(current, before, after, "$")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if equal(current, patched) {
		return nil
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, patched, "", "  "); err != nil {
		return err
	}
	indented.WriteByte('\n')
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return safefile.WriteFile(path, indented.Bytes(), 0644)
}

func object(b []byte) (map[string]json.RawMessage, bool) {
	if !bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
		return nil, false
	}
	var m map[string]json.RawMessage
	err := json.Unmarshal(b, &m)
	return m, err == nil && m != nil
}

func equal(a, b []byte) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	decode := func(raw []byte) (any, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		err := d.Decode(&v)
		return v, err
	}
	x, ex := decode(a)
	y, ey := decode(b)
	return ex == nil && ey == nil && reflect.DeepEqual(x, y)
}

func merge(current, before, next json.RawMessage, path string) (json.RawMessage, error) {
	if equal(before, next) || equal(current, next) {
		return current, nil
	}
	cm, co := object(current)
	bm, bo := object(before)
	nm, no := object(next)
	if len(before) == 0 || (no && bytes.Equal(bytes.TrimSpace(before), []byte("null"))) {
		bm = map[string]json.RawMessage{}
		bo = true
	}
	if len(current) == 0 || (no && bytes.Equal(bytes.TrimSpace(current), []byte("null"))) {
		cm = map[string]json.RawMessage{}
		co = true
	}
	if co && bo && no {
		keys := map[string]bool{}
		for k := range bm {
			keys[k] = true
		}
		for k := range nm {
			keys[k] = true
		}
		for k := range keys {
			v, err := merge(cm[k], bm[k], nm[k], path+"."+k)
			if err != nil {
				return nil, err
			}
			if len(v) == 0 {
				delete(cm, k)
			} else {
				cm[k] = v
			}
		}
		return json.Marshal(cm)
	}
	// Append-only histories can merge independent new entries, while keeping
	// the current raw prefix (including unknown fields) intact.
	var ca, ba, na []json.RawMessage
	if json.Unmarshal(current, &ca) == nil && json.Unmarshal(before, &ba) == nil && json.Unmarshal(next, &na) == nil && len(na) > len(ba) && len(ca) >= len(ba) {
		prefix := true
		for i := range ba {
			if !equal(ba[i], na[i]) || !equal(ba[i], ca[i]) {
				prefix = false
				break
			}
		}
		if prefix {
			return json.Marshal(append(ca, na[len(ba):]...))
		}
	}
	if len(current) == 0 && bytes.Equal(bytes.TrimSpace(before), []byte("null")) {
		return next, nil
	}
	if !equal(current, before) {
		return nil, fmt.Errorf("state changed at %s; reload before retrying", path)
	}
	return next, nil
}
