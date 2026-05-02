package diff

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stripANSI(b []byte) []byte {
	return ansiRE.ReplaceAll(b, nil)
}

// WriteMarkdown renders a Markdown report of the changes to outputPath.
// ANSI escape codes are stripped so GitHub renders the content cleanly.
func WriteMarkdown(changes []Change, outputPath string) error {
	var buf bytes.Buffer
	s := Summarize(changes)
	total := s.Unchanged + s.Added + s.Removed + s.Modified

	buf.WriteString("## termbook diff\n\n")
	if !HasChanges(changes) {
		fmt.Fprintf(&buf, "No changes detected (%d screens).\n", total)
		return write(outputPath, buf.Bytes())
	}

	parts := []string{}
	if s.Modified > 0 {
		parts = append(parts, fmt.Sprintf("**%d modified**", s.Modified))
	}
	if s.Added > 0 {
		parts = append(parts, fmt.Sprintf("**%d added**", s.Added))
	}
	if s.Removed > 0 {
		parts = append(parts, fmt.Sprintf("**%d removed**", s.Removed))
	}
	parts = append(parts, fmt.Sprintf("%d unchanged", s.Unchanged))
	fmt.Fprintf(&buf, "%s · %d total\n\n", strings.Join(parts, " · "), total)

	for _, c := range changes {
		if c.Status == StatusUnchanged {
			continue
		}
		fmt.Fprintf(&buf, "### `%s` · %s\n", c.ID, statusLabel(c.Status))
		if c.Title != "" && c.Title != c.ID {
			fmt.Fprintf(&buf, "_%s_\n\n", c.Title)
		} else {
			buf.WriteString("\n")
		}
		if c.Command != "" {
			fmt.Fprintf(&buf, "```sh\n$ %s\n```\n\n", c.Command)
		}
		writeDiffBlock(&buf, c)
		buf.WriteString("\n")
	}
	return write(outputPath, buf.Bytes())
}

func writeDiffBlock(buf *bytes.Buffer, c Change) {
	switch c.Status {
	case StatusAdded:
		buf.WriteString("```diff\n")
		for _, h := range c.Hunks {
			line := string(stripANSI([]byte(h.Line)))
			fmt.Fprintf(buf, "+%s\n", line)
		}
		buf.WriteString("```\n")
	case StatusRemoved:
		buf.WriteString("```diff\n")
		for _, h := range c.Hunks {
			line := string(stripANSI([]byte(h.Line)))
			fmt.Fprintf(buf, "-%s\n", line)
		}
		buf.WriteString("```\n")
	default:
		buf.WriteString("```diff\n")
		for _, h := range c.Hunks {
			line := string(stripANSI([]byte(h.Line)))
			fmt.Fprintf(buf, "%c%s\n", h.Op, line)
		}
		buf.WriteString("```\n")
	}
}

func statusLabel(s Status) string {
	switch s {
	case StatusAdded:
		return "added"
	case StatusRemoved:
		return "removed"
	case StatusModified:
		return "modified"
	default:
		return string(s)
	}
}

func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
