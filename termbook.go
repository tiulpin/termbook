// Package termbook generates browsable HTML galleries of CLI terminal output.
//
// It captures ANSI-colored command output and renders it as a static HTML page
// with a sidebar-driven reading layout, dark/light themes, a fuzzy filter,
// and responsive breakpoints down to phone — like Storybook, but for CLI
// tools.
//
// Zero-config produces a polished gallery. A handful of options set colors,
// fonts, and the GitHub link. Everything else — brand mark, crumbs, kicker,
// facts, notes callout, footer, attribution, bloom — lives on one [Decor]
// struct so the option surface stays small:
//
//	book := termbook.New("My CLI",
//	    termbook.WithAccent("#FF6B6B"),
//	    termbook.WithGitHub("https://github.com/you/mycli"),
//	    termbook.WithDecor(termbook.Decor{
//	        BrandName:    "my cli",
//	        BrandVersion: "v1.0.0",
//	        Kicker:       "Internal · Screen reference",
//	        Lede:         "Generated from real output.",
//	        Facts:        []termbook.Fact{{"120", "screens"}, {"12", "groups"}},
//	    }),
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
	ID           string
	Title        string
	Desc         string
	Command      string
	HTML         template.HTML
	Experimental bool
	// NavLabel is the text shown in the sidebar nav. Defaults to Title.
	NavLabel string
}

// SearchKey returns a lowercased blob used by the in-page filter.
// Exported so the template can embed it in a data attribute.
func (s Screen) SearchKey() string {
	return strings.ToLower(s.ID + " " + s.Title + " " + s.Desc + " " + s.Command)
}

// Category groups related screens.
type Category struct {
	Name    string
	ID      string
	Blurb   string
	Screens []Screen
}

// Fact is a stat line displayed in the masthead.
type Fact struct {
	Value string
	Label string
}

// Notes is a callout card rendered between the masthead and the first section.
// Body is raw HTML so callers can embed emphasis, kbd hints, or links.
// If Title is empty the card is not rendered.
type Notes struct {
	Title string
	Body  template.HTML
}

// Decor bundles all textual and visual chrome around the gallery: brand
// strip, topbar crumbs, masthead copy, below-masthead callout, page footer,
// and masthead bloom.
//
// Zero-valued fields keep the template's sensible defaults. The struct
// exists so new decoration slots can be added as fields rather than as new
// Option functions, which keeps the library's API surface small.
type Decor struct {
	// Sidebar brand row. BrandName falls back to the book's title.
	// BrandMark injects raw HTML for the icon slot; if empty, a small
	// lettered square (first letter of BrandName) is rendered.
	BrandName    string
	BrandVersion string
	BrandMark    template.HTML

	// Crumbs replaces the topbar breadcrumb. The final segment is styled
	// as the current location. If empty, the breadcrumb is
	// `<BrandName> / gallery`.
	Crumbs []string

	// Kicker is a short uppercase label above the h1
	// (e.g. "Internal · Design discussion draft").
	Kicker string
	// Lede is a single-paragraph intro below the h1. Also settable via
	// [WithIntro].
	Lede string
	// Facts populates the stats column on the right of the masthead.
	Facts []Fact

	// Notes is the accent-bordered callout between masthead and sections.
	Notes Notes

	// Footer is the left-hand text on the page footer. The right-hand
	// side always shows the generation timestamp and screen count.
	Footer string
	// Attribution is an optional HTML band below the footer for
	// wordmark / tagline / copyright.
	Attribution template.HTML

	// BloomDark / BloomLight override the soft bloom painted behind the
	// masthead. Pass a full CSS background value (typically a
	// `radial-gradient(...)`); if both are empty, an accent-derived glow
	// is used.
	BloomDark  string
	BloomLight string
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
	sansFontName string
	sansFontURL  string
	defaultTheme string
	customCSS    string
	templateHTML string
	decor        Decor
}

// Option configures a [Book].
type Option func(*options)

// WithGitHub adds a GitHub link to the sidebar footer.
func WithGitHub(url string) Option {
	return func(o *options) { o.githubURL = url }
}

// WithAccent sets the accent color (hex or any CSS color). Drives the
// masthead slash, sidebar active link, search focus ring, and the bloom
// behind the title. Works in both dark and light themes.
func WithAccent(hex string) Option {
	return func(o *options) { o.accent = hex }
}

// WithFont sets the monospace font used for terminal bodies, IDs, and
// the h1. Default is JetBrains Mono.
func WithFont(name, cssURL string) Option {
	return func(o *options) { o.fontName = name; o.fontURL = cssURL }
}

// WithSans sets the sans-serif font used for prose (lede, descriptions,
// blurbs). Default is Inter.
func WithSans(name, cssURL string) Option {
	return func(o *options) { o.sansFontName = name; o.sansFontURL = cssURL }
}

// WithIntro is a shortcut for [Decor.Lede] — a single-paragraph intro
// shown under the title. Keep for callers that just want a one-liner
// without reaching for [WithDecor].
func WithIntro(text string) Option {
	return func(o *options) { o.decor.Lede = text }
}

// WithDefaultTheme sets which theme loads first: "dark" (default) or "light".
func WithDefaultTheme(theme string) Option {
	return func(o *options) { o.defaultTheme = theme }
}

// WithColumns is retained for backward compatibility with earlier termbook
// releases that used a grid layout. The current layout is single-column
// reading flow, so this option is ignored.
//
// Deprecated: no-op in the current layout.
func WithColumns(int) Option { return func(*options) {} }

// WithCSS injects additional CSS after the default styles. Use for targeted
// overrides without replacing the entire template.
//
//	termbook.WithCSS(".term-body { font-size: 14px; }")
func WithCSS(css string) Option {
	return func(o *options) { o.customCSS = css }
}

// WithTemplate replaces the entire HTML template. The template receives a
// [TemplateData] struct. Use as an escape hatch when the other options
// aren't enough.
func WithTemplate(html string) Option {
	return func(o *options) { o.templateHTML = html }
}

// WithDecor configures the chrome around the gallery — brand row, crumbs,
// masthead, notes callout, footer, and bloom.
//
// WithDecor replaces the entire [Decor] configuration, including anything
// set by a prior [WithIntro]. Call it once with a fully-populated struct;
// for the "just a one-line intro" case, [WithIntro] alone is shorter.
func WithDecor(d Decor) Option {
	return func(o *options) { o.decor = d }
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

// CategoryWithBlurb adds a named category that displays a short blurb in its
// section header.
func (b *Book) CategoryWithBlurb(name, id, blurb string, screens ...Screen) {
	b.categories = append(b.categories, Category{Name: name, ID: id, Blurb: blurb, Screens: screens})
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

// TemplateData is passed to the HTML template. Exported so custom templates
// (via [WithTemplate]) can reference the type.
type TemplateData struct {
	Title        string
	Generated    string
	ScreenCount  int
	GitHubURL    string
	DefaultTheme string
	FontName     string
	FontURL      string
	SansFontName string
	SansFontURL  string
	CustomCSS    template.CSS
	Categories   []Category

	// Decor holds all chrome content set via [WithDecor] / [WithIntro].
	Decor Decor
	// BrandName is the effective sidebar brand text: [Decor.BrandName] if set,
	// otherwise the book title.
	BrandName string
}

func (b *Book) buildCustomCSS() string {
	var css strings.Builder
	o := &b.opts
	if o.accent != "" {
		fmt.Fprintf(&css, ":root,[data-theme=\"dark\"]{--accent:%s}\n", o.accent)
		fmt.Fprintf(&css, "[data-theme=\"light\"]{--accent:%s}\n", o.accent)
	}
	if o.decor.BloomDark != "" {
		fmt.Fprintf(&css, ":root,[data-theme=\"dark\"]{--bloom:%s}\n", o.decor.BloomDark)
	}
	if o.decor.BloomLight != "" {
		fmt.Fprintf(&css, "[data-theme=\"light\"]{--bloom:%s}\n", o.decor.BloomLight)
	}
	if o.customCSS != "" {
		css.WriteString(o.customCSS)
		css.WriteByte('\n')
	}
	return css.String()
}

// normalizeScreens fills in derived defaults (NavLabel) so the template
// doesn't have to branch on empty strings.
func normalizeScreens(cats []Category) []Category {
	out := make([]Category, len(cats))
	for i, c := range cats {
		screens := make([]Screen, len(c.Screens))
		for j, s := range c.Screens {
			if s.NavLabel == "" {
				s.NavLabel = s.Title
			}
			screens[j] = s
		}
		c.Screens = screens
		out[i] = c
	}
	return out
}

// Generate renders the gallery HTML to the given file path.
func (b *Book) Generate(outputPath string) error {
	funcs := template.FuncMap{
		"inc": func(i int) int { return i + 1 },
		"sub": func(a, b int) int { return a - b },
	}
	tmpl, err := template.New("termbook").Funcs(funcs).Parse(cmp.Or(b.opts.templateHTML, defaultTemplate))
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
		DefaultTheme: cmp.Or(b.opts.defaultTheme, "dark"),
		FontName:     cmp.Or(b.opts.fontName, "JetBrains Mono"),
		FontURL:      cmp.Or(b.opts.fontURL, "https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;700&display=swap"),
		SansFontName: cmp.Or(b.opts.sansFontName, "Inter"),
		SansFontURL:  cmp.Or(b.opts.sansFontURL, "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap"),
		CustomCSS:    template.CSS(b.buildCustomCSS()), //nolint:gosec // trusted
		Categories:   normalizeScreens(b.categories),
		Decor:        b.opts.decor,
		BrandName:    cmp.Or(b.opts.decor.BrandName, b.title),
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
