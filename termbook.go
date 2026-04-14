// Package termbook generates browsable HTML galleries of CLI terminal output.
//
// It captures ANSI-colored command output and renders it as a static HTML page
// with a grid layout, dark/light themes, click-to-expand terminal blocks,
// and a sidebar navigation — like Storybook, but for CLI tools.
//
// Zero-config produces a polished gallery. Options customize branding,
// colors, typography, and layout progressively:
//
//	book := termbook.New("My CLI",
//	    termbook.WithAccent("#FF6B6B"),
//	    termbook.WithGitHub("https://github.com/you/mycli"),
//	)
//	book.Category("Commands", "commands",
//	    termbook.Scr("list", "list users", "List all users", "mycli list", capturedANSI),
//	)
//	book.Generate("docs/gallery/index.html")
package termbook

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	terminal "github.com/buildkite/terminal-to-html/v3"
)

//go:embed gallery.html
var defaultTemplate string

// Screen represents a single CLI screen capture.
type Screen struct {
	ID      string
	Title   string
	Desc    string
	Command string
	HTML    template.HTML
}

// Category groups related screens.
type Category struct {
	Name    string
	ID      string
	Screens []Screen
}

// Book holds the gallery configuration and screens.
type Book struct {
	title      string
	categories []Category
	opts       options
}

type options struct {
	githubURL    string
	accent       string
	fontName     string
	fontURL      string
	intro        string
	defaultTheme string
	columns      int
	customCSS    string
	templateHTML string
}

// Option configures a [Book].
type Option func(*options)

// WithGitHub adds a GitHub star link to the gallery header.
func WithGitHub(url string) Option {
	return func(o *options) { o.githubURL = url }
}

// WithAccent sets the accent color (hex, e.g. "#FF6B6B").
// Drives sidebar highlights, category headers, hover states, and header title.
// Works in both dark and light themes.
func WithAccent(hex string) Option {
	return func(o *options) { o.accent = hex }
}

// WithFont sets the monospace font. Provide a name and a CSS import URL
// (typically Google Fonts). Default is JetBrains Mono.
//
//	termbook.WithFont("Fira Code", "https://fonts.googleapis.com/css2?family=Fira+Code:wght@400;500;700&display=swap")
func WithFont(name, cssURL string) Option {
	return func(o *options) { o.fontName = name; o.fontURL = cssURL }
}

// WithIntro sets a short intro paragraph below the header.
func WithIntro(text string) Option {
	return func(o *options) { o.intro = text }
}

// WithDefaultTheme sets which theme loads first: "dark" (default) or "light".
func WithDefaultTheme(theme string) Option {
	return func(o *options) { o.defaultTheme = theme }
}

// WithColumns sets the grid column count. Default is 3.
// Falls back to 2 on narrow viewports and 1 on mobile regardless.
func WithColumns(n int) Option {
	return func(o *options) { o.columns = n }
}

// WithCSS injects additional CSS rules after the default styles.
// Use for targeted overrides without replacing the entire template.
//
//	termbook.WithCSS(".terminal pre { font-size: 14px; }")
func WithCSS(css string) Option {
	return func(o *options) { o.customCSS = css }
}

// WithTemplate replaces the entire HTML template.
// The template receives a [TemplateData] struct. Use as an escape hatch
// when the other options aren't enough.
func WithTemplate(html string) Option {
	return func(o *options) { o.templateHTML = html }
}

// New creates a new gallery book with the given title.
func New(title string, opts ...Option) *Book {
	b := &Book{title: title}
	for _, o := range opts {
		o(&b.opts)
	}
	return b
}

// Category adds a named category with screens.
func (b *Book) Category(name, id string, screens ...Screen) {
	b.categories = append(b.categories, Category{Name: name, ID: id, Screens: screens})
}

// Scr creates a screen from pre-captured ANSI output.
func Scr(id, title, desc, command, ansiOutput string) Screen {
	return Screen{
		ID:      id,
		Title:   title,
		Desc:    desc,
		Command: command,
		HTML:    template.HTML(terminal.Render([]byte(ansiOutput))), //nolint:gosec // trusted
	}
}

// Manual creates a screen by calling a builder function that writes ANSI output.
func Manual(id, title, desc, command string, fn func(w io.Writer)) Screen {
	var buf bytes.Buffer
	fn(&buf)
	return Scr(id, title, desc, command, buf.String())
}

// TemplateData is passed to the HTML template.
// Exported so custom templates (via [WithTemplate]) can reference the type.
type TemplateData struct {
	Title        string
	Generated    string
	ScreenCount  int
	GitHubURL    string
	Intro        string
	DefaultTheme string
	FontName     string
	FontURL      string
	Columns      int
	CustomCSS    template.CSS
	Categories   []Category
}

func (b *Book) buildCustomCSS() string {
	var css strings.Builder
	o := &b.opts
	if o.accent != "" {
		// Override accent + derive glow
		fmt.Fprintf(&css, ":root,[data-theme=\"dark\"]{--accent:%s;--accent-glow:%s14;--cmd-color:%s}\n", o.accent, o.accent, o.accent)
		fmt.Fprintf(&css, "[data-theme=\"light\"]{--accent:%s;--accent-glow:%s0f;--cmd-color:%s}\n", o.accent, o.accent, o.accent)
	}
	if o.fontName != "" {
		fmt.Fprintf(&css, "body,.terminal pre,.screen-cmd,.theme-toggle{font-family:'%s',monospace}\n", o.fontName)
	}
	if o.columns > 0 && o.columns != 3 {
		fmt.Fprintf(&css, ".screens{grid-template-columns:repeat(%d,1fr)}\n", o.columns)
	}
	if o.customCSS != "" {
		css.WriteString(o.customCSS)
		css.WriteByte('\n')
	}
	return css.String()
}

// Generate renders the gallery HTML to the given file path.
func (b *Book) Generate(outputPath string) error {
	tmpl, err := template.New("termbook").Parse(cmp.Or(b.opts.templateHTML, defaultTemplate))
	if err != nil {
		return fmt.Errorf("termbook: parse template: %w", err)
	}

	n := 0
	for _, c := range b.categories {
		n += len(c.Screens)
	}

	data := TemplateData{
		Title:        b.title,
		Generated:    time.Now().Format("2006-01-02 15:04"),
		ScreenCount:  n,
		GitHubURL:    b.opts.githubURL,
		Intro:        b.opts.intro,
		DefaultTheme: cmp.Or(b.opts.defaultTheme, "dark"),
		FontURL:      cmp.Or(b.opts.fontURL, "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;700&display=swap"),
		Columns:      b.opts.columns,
		CustomCSS:    template.CSS(b.buildCustomCSS()), //nolint:gosec // trusted
		Categories:   b.categories,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("termbook: execute template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("termbook: create directory: %w", err)
	}

	if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("termbook: write file: %w", err)
	}

	return nil
}
