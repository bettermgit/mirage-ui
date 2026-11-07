// Package camouflage serves a static cover website that Xray fallbacks steer
// unmatched TLS connections to (anti-censorship "fronting", like v2ray-agent).
package camouflage

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed all:templates
var templatesFS embed.FS

// MarkerFile lands in the deploy dir so we never wipe a hand-built site by accident.
const MarkerFile = ".3x-ui-camouflage"

type Template struct {
	Key  string `json:"key" example:"blog"`
	Name string `json:"name" example:"Personal Blog"`
}

var templateNames = map[string]string{
	"blog":    "Personal Blog",
	"company": "Corporate Site",
	"gallery": "Photo Gallery",
}

// List returns the bundled templates, sorted by key.
func List() []Template {
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		return nil
	}
	out := make([]Template, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := templateNames[e.Name()]
		if name == "" {
			name = e.Name()
		}
		out = append(out, Template{Key: e.Name(), Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// Valid reports whether key is a bundled template.
func Valid(key string) bool {
	for _, t := range List() {
		if t.Key == key {
			return true
		}
	}
	return false
}

// Deploy extracts the template into dir, replacing a previous deployment only
// when dir was created by an earlier deploy (marker file) or is empty.
func Deploy(key, dir string) error {
	if !Valid(key) {
		return fmt.Errorf("unknown camouflage template %q", key)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	marker := filepath.Join(dir, MarkerFile)
	if err := checkDirClean(dir, marker); err != nil {
		return err
	}
	src := "templates/" + key
	entries, err := fs.ReadDir(templatesFS, src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := templatesFS.ReadFile(src + "/" + e.Name())
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0o644); err != nil {
			return err
		}
	}
	return os.WriteFile(marker, []byte(key+"\n"), 0o644)
}

func checkDirClean(dir, marker string) error {
	if _, err := os.Stat(marker); err == nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		return fmt.Errorf("camouflage dir %s is not empty and was not deployed by the panel; pick another port/dir or clear it first", dir)
	}
	return nil
}
