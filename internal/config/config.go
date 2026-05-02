package config

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	Dir          = ".termbook"
	ManifestFile = "termbook.yml"
	CapturesDir  = "captures"
)

func ManifestPath(workdir string) string {
	return filepath.Join(workdir, Dir, ManifestFile)
}

func CapturePath(workdir, id string) string {
	return filepath.Join(workdir, Dir, CapturesDir, id+".ansi")
}

type Manifest struct {
	Title      string       `yaml:"title"`
	Accent     string       `yaml:"accent,omitempty"`
	GitHub     string       `yaml:"github,omitempty"`
	Width      int          `yaml:"width,omitempty"`
	Intro      string       `yaml:"intro,omitempty"`
	Kicker     string       `yaml:"kicker,omitempty"`
	Footer     string       `yaml:"footer,omitempty"`
	Notes      *Notes       `yaml:"notes,omitempty"`
	Facts      []Fact       `yaml:"facts,omitempty"`
	Redact     []RedactRule `yaml:"redact,omitempty"`
	Categories []Category   `yaml:"categories,omitempty"`
}

// RedactRule is applied during diff comparison only. Pattern is a Go
// regexp; Replace is the literal replacement (with $1, $2 backreferences).
type RedactRule struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`
}

type Notes struct {
	Title string `yaml:"title"`
	Body  string `yaml:"body"`
}

type Fact struct {
	Value string `yaml:"value"`
	Label string `yaml:"label"`
}

type Category struct {
	Name    string   `yaml:"name"`
	ID      string   `yaml:"id,omitempty"`
	Blurb   string   `yaml:"blurb,omitempty"`
	Screens []Screen `yaml:"screens,omitempty"`
}

type Screen struct {
	ID      string `yaml:"id"`
	Title   string `yaml:"title,omitempty"`
	Desc    string `yaml:"desc,omitempty"`
	Command string `yaml:"command"`
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &m, nil
}

func (m *Manifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

func (m *Manifest) UpsertScreen(catName string, s Screen) {
	catName = cmp.Or(catName, "Screens")
	for ci, c := range m.Categories {
		if c.Name == catName {
			for si, existing := range c.Screens {
				if existing.ID == s.ID {
					m.Categories[ci].Screens[si] = mergeScreen(existing, s)
					return
				}
			}
			m.Categories[ci].Screens = append(m.Categories[ci].Screens, s)
			return
		}
	}
	m.Categories = append(m.Categories, Category{
		Name:    catName,
		ID:      slugify(catName),
		Screens: []Screen{s},
	})
}

func (m *Manifest) FindScreen(id string) *Screen {
	for _, c := range m.Categories {
		for i := range c.Screens {
			if c.Screens[i].ID == id {
				return &c.Screens[i]
			}
		}
	}
	return nil
}

// Re-record with --id keeps prior title/desc unless the caller passes new ones.
func mergeScreen(existing, incoming Screen) Screen {
	incoming.Title = cmp.Or(incoming.Title, existing.Title)
	incoming.Desc = cmp.Or(incoming.Desc, existing.Desc)
	incoming.Command = cmp.Or(incoming.Command, existing.Command)
	return incoming
}

func DeriveID(command string) string {
	return slugify(command)
}

func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case b.Len() > 0 && !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return cmp.Or(strings.Trim(b.String(), "-"), "screen")
}

func LoadOrInit(path, defaultTitle string) (*Manifest, bool, error) {
	m, err := Load(path)
	if err == nil {
		return m, false, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	return &Manifest{Title: defaultTitle}, true, nil
}
