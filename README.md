# termbook

Generate browsable HTML galleries from CLI terminal output. Like Storybook, but for command-line tools.

Captures ANSI-colored output and renders it as a static page with a grid layout, dark/light themes, click-to-expand terminals, and sidebar navigation.

<img width="3892" height="2196" alt="cli" src="https://github.com/user-attachments/assets/da17c2db-f739-400e-9dd3-d597020f00eb" />


## Usage

```go
book := termbook.New("mycli — screen gallery",
    termbook.WithGitHub("https://github.com/you/mycli"),
)

book.Category("Users", "users",
    termbook.Scr("user-list", "user list", "List all users",
        "mycli user list", capturedANSIOutput),
)

book.Generate("docs/gallery/index.html")
```

`Scr` takes an ANSI string — capture it however you want. There's also `Manual` for building output with a writer:

```go
termbook.Manual("login", "login", "Auth flow", "mycli login", func(w io.Writer) {
    fmt.Fprintf(w, "✓ Logged in as admin\n")
})
```

## Capture helper for Cobra

```go
func capture(rootCmd *cobra.Command, args ...string) string {
    var buf bytes.Buffer
    rootCmd.SetArgs(args)
    rootCmd.SetOut(&buf)
    rootCmd.SetErr(&buf)
    rootCmd.Execute()
    return buf.String()
}
```

## Options

```go
// Branding
termbook.WithGitHub("https://github.com/...")          // star link in header
termbook.WithIntro("Generated from real output")       // intro text below header

// Appearance
termbook.WithAccent("#FF6B6B")                         // accent color (both themes)
termbook.WithDefaultTheme("light")                     // "dark" (default) or "light"
termbook.WithFont("Fira Code",                         // custom monospace font
    "https://fonts.googleapis.com/css2?family=Fira+Code:wght@400;500;700&display=swap")
termbook.WithColumns(2)                                // grid columns (default 3)

// Advanced
termbook.WithCSS(".terminal pre { font-size: 14px }")  // inject extra CSS
termbook.WithTemplate(myHTML)                          // replace entire template
```

Zero config works out of the box. Each option layers on independently.

## Install

```
go get github.com/tiulpin/termbook
```

## See also

- [terminal-to-html](https://github.com/buildkite/terminal-to-html) by Buildkite — standalone CLI to convert ANSI terminal output to HTML
- [VHS](https://github.com/charmbracelet/vhs) by Charm — record terminal sessions as GIFs/MP4s from tape files
