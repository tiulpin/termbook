package main

import (
	"context"
	"errors"
	"fmt"
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
	)
	cmd := &cobra.Command{
		Use:   "record [command...]",
		Short: "Capture a command's output and add it to the manifest",
		Long: `Run a command in a PTY, capture its ANSI output, and upsert it into the
manifest. The first invocation scaffolds .termbook/termbook.yml.

With --only <id>, re-captures an existing entry's command and overwrites
its capture file without changing the manifest.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cc *cobra.Command, args []string) error {
			workdir, err := os.Getwd()
			if err != nil {
				return err
			}
			if only != "" && len(args) > 0 {
				return errors.New("--only is mutually exclusive with a positional command")
			}
			if only == "" && len(args) == 0 {
				return errors.New("provide a command to record, or use --only <id>")
			}

			manifestPath := config.ManifestPath(workdir)
			defaultTitle := filepath.Base(workdir)
			m, _, err := config.LoadOrInit(manifestPath, defaultTitle)
			if err != nil {
				return err
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
					Title:   firstNonEmpty(title, cmdStr),
					Desc:    desc,
					Command: cmdStr,
				}
			}

			effectiveWidth := width
			if effectiveWidth <= 0 {
				effectiveWidth = m.Width
			}

			ctx, cancel := context.WithCancel(cc.Context())
			defer cancel()
			res, err := capture.Run(ctx, []string{"sh", "-c", screen.Command}, capture.Options{
				Width:   effectiveWidth,
				Timeout: timeout,
			})
			if err != nil {
				return err
			}
			if res.TimedOut {
				return fmt.Errorf("timed out after %s: %s", timeout, screen.Command)
			}

			capPath := config.CapturePath(workdir, screen.ID)
			if err := os.MkdirAll(filepath.Dir(capPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(capPath, res.Output, 0o644); err != nil {
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
	return cmd
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
