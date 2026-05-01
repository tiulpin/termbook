package main

import (
	"errors"
	"fmt"
	"html/template"
	"os"

	"github.com/spf13/cobra"

	"github.com/tiulpin/termbook"
	"github.com/tiulpin/termbook/internal/config"
)

func newBuildCmd() *cobra.Command {
	var (
		output string
		only   string
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Render gallery.html from the manifest and captures",
		Args:  cobra.NoArgs,
		RunE: func(cc *cobra.Command, _ []string) error {
			workdir, err := os.Getwd()
			if err != nil {
				return err
			}
			manifestPath := config.ManifestPath(workdir)
			m, err := config.Load(manifestPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("no manifest at %s — run `termbook record` first", manifestPath)
				}
				return err
			}

			var opts []termbook.Option
			if m.Accent != "" {
				opts = append(opts, termbook.WithAccent(m.Accent))
			}
			if m.GitHub != "" {
				opts = append(opts, termbook.WithGitHub(m.GitHub))
			}
			if decor, ok := buildDecor(m); ok {
				opts = append(opts, termbook.WithDecor(decor))
			}
			book := termbook.New(m.Title, opts...)

			screensEmitted := 0
			missing := 0
			for _, c := range m.Categories {
				var screens []termbook.Screen
				for _, s := range c.Screens {
					if only != "" && s.ID != only {
						continue
					}
					ansi, err := os.ReadFile(config.CapturePath(workdir, s.ID))
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: missing capture for %q (%s) — skipping\n", s.ID, err)
						missing++
						continue
					}
					screens = append(screens, termbook.Scr(s.ID, s.Title, s.Desc, s.Command, string(ansi)))
				}
				if len(screens) == 0 {
					continue
				}
				catID := c.ID
				if catID == "" {
					catID = s2id(c.Name)
				}
				if c.Blurb != "" {
					book.CategoryWithBlurb(c.Name, catID, c.Blurb, screens...)
				} else {
					book.Category(c.Name, catID, screens...)
				}
				screensEmitted += len(screens)
			}

			if screensEmitted == 0 {
				return errors.New("no screens to render — run `termbook record` first")
			}
			if err := book.Generate(output); err != nil {
				return err
			}
			fmt.Fprintf(cc.OutOrStdout(), "wrote %s (%d screens", output, screensEmitted)
			if missing > 0 {
				fmt.Fprintf(cc.OutOrStdout(), ", %d missing", missing)
			}
			fmt.Fprintln(cc.OutOrStdout(), ")")
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "gallery.html", "output HTML path")
	cmd.Flags().StringVar(&only, "only", "", "render only the screen with this id")
	return cmd
}

func s2id(s string) string {
	return config.DeriveID(s)
}

func buildDecor(m *config.Manifest) (termbook.Decor, bool) {
	d := termbook.Decor{
		Kicker: m.Kicker,
		Lede:   m.Intro,
		Footer: m.Footer,
	}
	if m.Notes != nil && (m.Notes.Title != "" || m.Notes.Body != "") {
		d.Notes = termbook.Notes{
			Title: m.Notes.Title,
			Body:  template.HTML(m.Notes.Body), //nolint:gosec // trusted manifest input
		}
	}
	for _, f := range m.Facts {
		d.Facts = append(d.Facts, termbook.Fact{Value: f.Value, Label: f.Label})
	}
	any := d.Kicker != "" || d.Lede != "" || d.Footer != "" || d.Notes.Title != "" || d.Notes.Body != "" || len(d.Facts) > 0
	return d, any
}
