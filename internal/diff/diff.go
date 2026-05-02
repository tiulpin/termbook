// Package diff compares current termbook captures against a baseline,
// applying manifest-level redaction rules before comparison.
package diff

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tiulpin/termbook/internal/config"
)

type Status string

const (
	StatusUnchanged Status = "unchanged"
	StatusAdded     Status = "added"
	StatusRemoved   Status = "removed"
	StatusModified  Status = "modified"
)

type Change struct {
	ID       string
	Title    string
	Category string
	Command  string
	Status   Status

	// Raw bytes pre-redaction (for rendering the actual ANSI in reports).
	BaselineRaw []byte
	CurrentRaw  []byte

	// Hunks are computed from redacted line streams. Each hunk's Op is
	// ' ' for context, '+' for added, '-' for removed.
	Hunks []Hunk
}

type Hunk struct {
	Op   byte
	Line string
}

// BaselineReader returns the baseline capture bytes for a screen id, or
// os.ErrNotExist when the screen wasn't present in the baseline.
type BaselineReader func(id string) ([]byte, error)

// Compare walks the manifest and returns one Change per screen id that
// existed in either side. Unchanged screens are included so callers can
// summarize totals.
func Compare(m *config.Manifest, workdir string, baseline BaselineReader) ([]Change, error) {
	red, err := NewRedactor(m.Redact)
	if err != nil {
		return nil, err
	}

	var changes []Change
	for _, c := range m.Categories {
		for _, s := range c.Screens {
			cur, curErr := os.ReadFile(config.CapturePath(workdir, s.ID))
			base, baseErr := baseline(s.ID)

			currentExists := curErr == nil
			baselineExists := baseErr == nil

			if !currentExists && !baselineExists {
				continue
			}

			ch := Change{
				ID:          s.ID,
				Title:       s.Title,
				Category:    c.Name,
				Command:     s.Command,
				BaselineRaw: base,
				CurrentRaw:  cur,
			}

			switch {
			case !baselineExists:
				ch.Status = StatusAdded
				ch.Hunks = DiffLines(nil, splitLines(red.Apply(cur)))
			case !currentExists:
				ch.Status = StatusRemoved
				ch.Hunks = DiffLines(splitLines(red.Apply(base)), nil)
			default:
				redBase := red.Apply(base)
				redCur := red.Apply(cur)
				if bytes.Equal(redBase, redCur) {
					ch.Status = StatusUnchanged
				} else {
					ch.Status = StatusModified
					ch.Hunks = DiffLines(splitLines(redBase), splitLines(redCur))
				}
			}
			changes = append(changes, ch)
		}
	}
	return changes, nil
}

// DirReader reads baseline captures from a directory. The directory is
// expected to contain `<id>.ansi` files at its root.
func DirReader(root string) BaselineReader {
	return func(id string) ([]byte, error) {
		return os.ReadFile(filepath.Join(root, id+".ansi"))
	}
}

// GitHEADReader reads baseline captures via `git show HEAD:<rel>` from
// the repo root. captureRelDir is the path of the captures directory
// relative to the repo root (e.g. `examples/eza/.termbook/captures`).
func GitHEADReader(repoRoot, captureRelDir string) BaselineReader {
	return func(id string) ([]byte, error) {
		rel := filepath.ToSlash(filepath.Join(captureRelDir, id+".ansi"))
		cmd := exec.Command("git", "show", "HEAD:"+rel)
		cmd.Dir = repoRoot
		out, err := cmd.Output()
		if err != nil {
			if _, ok := errors.AsType[*exec.ExitError](err); ok {
				return nil, os.ErrNotExist
			}
			return nil, err
		}
		return out, nil
	}
}

// GitRepoRoot resolves the repo root from a path inside the working tree.
// Returns the absolute path or an error if not in a git repo.
func GitRepoRoot(path string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DiffLines is a line-level LCS diff. Op space=context, +=added, -=removed.
func DiffLines(a, b []string) []Hunk {
	m, n := len(a), len(b)
	if m == 0 && n == 0 {
		return nil
	}
	if m == 0 {
		out := make([]Hunk, n)
		for i, line := range b {
			out[i] = Hunk{'+', line}
		}
		return out
	}
	if n == 0 {
		out := make([]Hunk, m)
		for i, line := range a {
			out[i] = Hunk{'-', line}
		}
		return out
	}

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	hunks := make([]Hunk, 0, m+n)
	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && a[i-1] == b[j-1]:
			hunks = append(hunks, Hunk{' ', a[i-1]})
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			hunks = append(hunks, Hunk{'+', b[j-1]})
			j--
		default:
			hunks = append(hunks, Hunk{'-', a[i-1]})
			i--
		}
	}
	slices.Reverse(hunks)
	return hunks
}

func splitLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	s := strings.ReplaceAll(string(b), "\r\n", "\n")
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// Summary aggregates change counts.
type Summary struct {
	Unchanged, Added, Removed, Modified int
}

func Summarize(changes []Change) Summary {
	var s Summary
	for _, c := range changes {
		switch c.Status {
		case StatusUnchanged:
			s.Unchanged++
		case StatusAdded:
			s.Added++
		case StatusRemoved:
			s.Removed++
		case StatusModified:
			s.Modified++
		}
	}
	return s
}

// HasChanges returns true if any change is not StatusUnchanged.
func HasChanges(changes []Change) bool {
	for _, c := range changes {
		if c.Status != StatusUnchanged {
			return true
		}
	}
	return false
}
