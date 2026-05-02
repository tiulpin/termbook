package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/tiulpin/termbook/internal/capture"
	"github.com/tiulpin/termbook/internal/config"
)

func newRecordCmd() *cobra.Command {
	var (
		id      string
		cat     string
		title   string
		desc    string
		width   int
		timeout time.Duration
		only    string
		all     bool
	)
	cmd := &cobra.Command{
		Use:   "record [command...]",
		Short: "Capture a command's output and add it to the manifest",
		Long: `Run a command in a PTY, capture its ANSI output, and upsert it into the
manifest. The first invocation scaffolds .termbook/termbook.yml.

With --only <id>, re-captures an existing entry's command and overwrites
its capture file without changing the manifest.

With --all, re-captures every entry in the manifest. Useful in CI before
running ` + "`termbook diff`" + `.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cc *cobra.Command, args []string) error {
			workdir, err := os.Getwd()
			if err != nil {
				return err
			}

			modes := 0
			if all {
				modes++
			}
			if only != "" {
				modes++
			}
			if len(args) > 0 {
				modes++
			}
			if modes > 1 {
				return errors.New("--all, --only, and a positional command are mutually exclusive")
			}
			if modes == 0 {
				return errors.New("provide a command to record, or use --only <id> or --all")
			}

			manifestPath := config.ManifestPath(workdir)
			defaultTitle := filepath.Base(workdir)
			m, _, err := config.LoadOrInit(manifestPath, defaultTitle)
			if err != nil {
				return err
			}

			if all {
				return recordAll(cc.Context(), cc.OutOrStdout(), workdir, m, captureOpts(width, m.Width, timeout))
			}

			var screen config.Screen
			if only != "" {
				existing := m.FindScreen(only)
				if existing == nil {
					return fmt.Errorf("no screen with id %q in %s", only, manifestPath)
				}
				screen = *existing
			} else {
				cmdStr := strings.Join(args, " ")
				if id == "" {
					id = config.DeriveID(cmdStr)
				}
				screen = config.Screen{
					ID:      id,
					Title:   cmp.Or(title, cmdStr),
					Desc:    desc,
					Command: cmdStr,
				}
			}

			res, err := recordOne(cc.Context(), workdir, screen, captureOpts(width, m.Width, timeout))
			if err != nil {
				return err
			}

			if only == "" {
				m.UpsertScreen(cat, screen)
				if err := m.Save(manifestPath); err != nil {
					return err
				}
			}

			fmt.Fprintf(cc.OutOrStdout(), "captured %s (%d bytes, exit %d)\n",
				screen.ID, len(res.Output), res.ExitCode)

			if res.ExitCode != 0 {
				os.Exit(res.ExitCode)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "screen id (defaults to a slug of the command)")
	cmd.Flags().StringVar(&cat, "cat", "Screens", "category name")
	cmd.Flags().StringVar(&title, "title", "", "screen title (defaults to the command)")
	cmd.Flags().StringVar(&desc, "desc", "", "screen description")
	cmd.Flags().IntVar(&width, "width", 0, "PTY columns (default: manifest width or 120)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "per-command timeout")
	cmd.Flags().StringVar(&only, "only", "", "re-capture an existing entry by id without modifying the manifest")
	cmd.Flags().BoolVar(&all, "all", false, "re-capture every entry in the manifest")
	return cmd
}

func captureOpts(flagWidth, manifestWidth int, timeout time.Duration) capture.Options {
	return capture.Options{Width: cmp.Or(flagWidth, manifestWidth), Timeout: timeout}
}

func recordOne(ctx context.Context, workdir string, screen config.Screen, opts capture.Options) (*capture.Result, error) {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	res, err := capture.Run(runCtx, []string{"sh", "-c", screen.Command}, opts)
	if err != nil {
		return nil, err
	}
	if res.TimedOut {
		return res, fmt.Errorf("timed out after %s: %s", opts.Timeout, screen.Command)
	}
	capPath := config.CapturePath(workdir, screen.ID)
	if err := os.MkdirAll(filepath.Dir(capPath), 0o755); err != nil {
		return res, err
	}
	if err := os.WriteFile(capPath, res.Output, 0o644); err != nil {
		return res, err
	}
	return res, nil
}

func recordAll(ctx context.Context, out io.Writer, workdir string, m *config.Manifest, opts capture.Options) error {
	total, ok, fail := 0, 0, 0
	for _, c := range m.Categories {
		for _, s := range c.Screens {
			total++
			res, err := recordOne(ctx, workdir, s, opts)
			switch {
			case err != nil:
				fail++
				fmt.Fprintf(out, "FAIL %s: %v\n", s.ID, err)
			case res.ExitCode != 0:
				fail++
				fmt.Fprintf(out, "FAIL %s (exit %d, %d bytes)\n", s.ID, res.ExitCode, len(res.Output))
			default:
				ok++
				fmt.Fprintf(out, "  ok %s (%d bytes)\n", s.ID, len(res.Output))
			}
		}
	}
	fmt.Fprintf(out, "\n%d captured, %d failed (of %d)\n", ok, fail, total)
	if fail > 0 {
		return fmt.Errorf("%d capture(s) failed", fail)
	}
	return nil
}

