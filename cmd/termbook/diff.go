package main

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/tiulpin/termbook/internal/config"
	"github.com/tiulpin/termbook/internal/diff"
)

func newDiffCmd() *cobra.Command {
	var (
		baselineDir string
		baselineRef string
		reportHTML  string
		reportMD    string
		quiet       bool
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare current captures against a baseline",
		Long: `Compare the captures under .termbook/captures/ to a baseline and
report what changed. Defaults to comparing against the captures committed
at git HEAD; use --baseline to point at any directory containing
<id>.ansi files instead.

Manifest-level redact: rules are applied before comparison, so volatile
bits like timestamps don't drown out real diffs.

Exits 1 when any screen has changed, added, or been removed.`,
		Args: cobra.NoArgs,
		RunE: func(cc *cobra.Command, _ []string) error {
			workdir, err := os.Getwd()
			if err != nil {
				return err
			}
			m, err := config.Load(config.ManifestPath(workdir))
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("no manifest at %s", config.ManifestPath(workdir))
				}
				return err
			}

			reader, err := resolveBaselineReader(workdir, baselineDir, baselineRef)
			if err != nil {
				return err
			}

			changes, err := diff.Compare(m, workdir, reader)
			if err != nil {
				return err
			}

			printDiff(cc.OutOrStdout(), changes, quiet)

			if reportHTML != "" {
				if err := diff.WriteReport(changes, reportHTML); err != nil {
					return err
				}
				fmt.Fprintf(cc.OutOrStdout(), "wrote %s\n", reportHTML)
			}
			if reportMD != "" {
				if err := diff.WriteMarkdown(changes, reportMD); err != nil {
					return err
				}
				fmt.Fprintf(cc.OutOrStdout(), "wrote %s\n", reportMD)
			}

			if diff.HasChanges(changes) {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&baselineDir, "baseline", "", "directory of <id>.ansi files (default: git HEAD)")
	cmd.Flags().StringVar(&baselineRef, "baseline-ref", "", "git ref to use as baseline (e.g. main); default HEAD")
	cmd.Flags().StringVar(&reportHTML, "report-html", "", "also write an HTML report to this path")
	cmd.Flags().StringVar(&reportMD, "report-md", "", "also write a Markdown report (suitable for PR comments) to this path")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "print summary only, no per-line hunks")
	return cmd
}

func resolveBaselineReader(workdir, baselineDir, baselineRef string) (diff.BaselineReader, error) {
	if baselineDir != "" {
		if baselineRef != "" {
			return nil, errors.New("--baseline and --baseline-ref are mutually exclusive")
		}
		return diff.DirReader(baselineDir), nil
	}
	repoRoot, err := diff.GitRepoRoot(workdir)
	if err != nil {
		return nil, fmt.Errorf("not in a git repository (use --baseline <dir>): %w", err)
	}
	relWork, err := filepath.Rel(repoRoot, workdir)
	if err != nil {
		return nil, err
	}
	captureRelDir := filepath.Join(relWork, config.Dir, config.CapturesDir)
	ref := cmp.Or(baselineRef, "HEAD")
	return diff.GitRefReader(repoRoot, ref, captureRelDir), nil
}

func printDiff(out io.Writer, changes []diff.Change, quiet bool) {
	for _, c := range changes {
		if c.Status == diff.StatusUnchanged {
			continue
		}
		fmt.Fprintf(out, "%s  %s  %s\n", padStatus(c.Status), c.ID, c.Title)
		if quiet {
			continue
		}
		for _, h := range c.Hunks {
			if h.Op == ' ' {
				continue
			}
			fmt.Fprintf(out, "  %c %s\n", h.Op, h.Line)
		}
		fmt.Fprintln(out)
	}
	s := diff.Summarize(changes)
	total := s.Unchanged + s.Added + s.Removed + s.Modified
	changed := s.Added + s.Removed + s.Modified
	if changed == 0 {
		fmt.Fprintf(out, "no changes (%d screens)\n", total)
		return
	}
	fmt.Fprintf(out, "%d changed (%d modified, %d added, %d removed) of %d screens\n",
		changed, s.Modified, s.Added, s.Removed, total)
}

func padStatus(s diff.Status) string {
	switch s {
	case diff.StatusModified:
		return "modified"
	case diff.StatusAdded:
		return "   added"
	case diff.StatusRemoved:
		return " removed"
	default:
		return string(s)
	}
}
