package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Meta is the JSON sidecar that lives alongside <slug>.db.
// It captures display-only state that doesn't belong inside the SQLite file.
type Meta struct {
	Name       string    `json:"name"`
	NameSource string    `json:"name_source"` // "auto" | "manual"
	UpdatedAt  time.Time `json:"updated_at"`
}

const (
	NameSourceAuto   = "auto"
	NameSourceManual = "manual"
	maxNameLen       = 100
)

// MetaPath returns the sidecar path for the given slug.
func MetaPath(dataDir, slug string) string {
	return filepath.Join(dataDir, "repos", slug+".meta.json")
}

// ReadMeta loads the sidecar. Missing file returns a zero Meta (not an error).
func ReadMeta(dataDir, slug string) (Meta, error) {
	data, err := os.ReadFile(MetaPath(dataDir, slug))
	if errors.Is(err, os.ErrNotExist) {
		return Meta{}, nil
	}
	if err != nil {
		return Meta{}, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return Meta{}, fmt.Errorf("parse meta: %w", err)
	}
	return m, nil
}

// WriteMeta persists the sidecar atomically.
func WriteMeta(dataDir, slug string, m Meta) error {
	if err := os.MkdirAll(filepath.Join(dataDir, "repos"), 0o755); err != nil {
		return err
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Join(dataDir, "repos"), slug+"-*.meta.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, MetaPath(dataDir, slug))
}

// ApplyPushedName updates the sidecar with a name received via push.
// Precedence rule: an existing manual name wins over an incoming auto name.
// Returns true if the meta was written, false if the existing manual name was preserved.
func ApplyPushedName(dataDir, slug, name, source string) (bool, error) {
	name = sanitizeName(name)
	if name == "" {
		return false, nil
	}
	if source != NameSourceManual {
		source = NameSourceAuto
	}

	current, err := ReadMeta(dataDir, slug)
	if err != nil {
		return false, err
	}

	// Existing manual name is sticky against an incoming auto name.
	if current.NameSource == NameSourceManual && source == NameSourceAuto {
		return false, nil
	}

	// Skip the write when nothing changed (avoid touching mtime needlessly).
	if current.Name == name && current.NameSource == source {
		return false, nil
	}

	if err := WriteMeta(dataDir, slug, Meta{
		Name:       name,
		NameSource: source,
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		return false, err
	}
	return true, nil
}

// SetManualName forces a manual rename (overrides any prior state).
func SetManualName(dataDir, slug, name string) error {
	name = sanitizeName(name)
	if name == "" {
		return errors.New("name cannot be empty after trimming")
	}
	return WriteMeta(dataDir, slug, Meta{
		Name:       name,
		NameSource: NameSourceManual,
		UpdatedAt:  time.Now().UTC(),
	})
}

// sanitizeName trims whitespace, removes control chars, enforces max length.
func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	cleaned := make([]rune, 0, len(name))
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			continue
		}
		cleaned = append(cleaned, r)
	}
	out := string(cleaned)
	if len(out) > maxNameLen {
		out = out[:maxNameLen]
	}
	return out
}
