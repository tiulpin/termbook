package diff

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"time"

	terminal "github.com/buildkite/terminal-to-html/v3"
)

//go:embed report.html
var reportTmpl string

type reportChange struct {
	Change
	BaselineHTML template.HTML
	CurrentHTML  template.HTML
}

type reportData struct {
	Changes   []reportChange
	S         Summary
	Total     int
	Changed   bool
	Generated string
}

// WriteReport renders an HTML report of the changes to outputPath.
func WriteReport(changes []Change, outputPath string) error {
	tmpl, err := template.New("diff").Parse(reportTmpl)
	if err != nil {
		return fmt.Errorf("diff: parse report template: %w", err)
	}

	rcs := make([]reportChange, 0, len(changes))
	for _, c := range changes {
		rc := reportChange{Change: c}
		if len(c.BaselineRaw) > 0 {
			rc.BaselineHTML = template.HTML(terminal.Render(c.BaselineRaw)) //nolint:gosec // trusted
		}
		if len(c.CurrentRaw) > 0 {
			rc.CurrentHTML = template.HTML(terminal.Render(c.CurrentRaw)) //nolint:gosec // trusted
		}
		rcs = append(rcs, rc)
	}

	s := Summarize(changes)
	data := reportData{
		Changes:   rcs,
		S:         s,
		Total:     s.Unchanged + s.Added + s.Removed + s.Modified,
		Changed:   HasChanges(changes),
		Generated: time.Now().Format("2006-01-02 15:04"),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("diff: execute report template: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, buf.Bytes(), 0o644)
}
