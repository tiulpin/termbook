# termbook

Generate browsable HTML galleries from CLI terminal output. Like Storybook, but for command-line tools.

Captures ANSI-colored output and renders it as a static page with a grid layout, dark/light themes, click-to-expand terminals, and sidebar navigation.

<img width="1543" height="1136" alt="image" src="https://github.com/user-attachments/assets/f8935e1c-2898-4227-9307-8683fd12f07e" />


## Examples

- [bat – cat with wings](https://htmlpreview.github.io/?https://raw.githubusercontent.com/tiulpin/termbook/main/examples/bat/gallery.html)
- [eza – modern ls](https://htmlpreview.github.io/?https://raw.githubusercontent.com/tiulpin/termbook/main/examples/eza/gallery.html)
- [ripgrep – fast recursive search](https://htmlpreview.github.io/?https://raw.githubusercontent.com/tiulpin/termbook/main/examples/ripgrep/gallery.html)
- [teamcity cli](https://jetbrains.github.io/teamcity-cli/)

Manifests live under [`examples/`](examples/); each one is a `.termbook/termbook.yml` plus committed `*.ansi` captures.

## Install

```sh
brew install tiulpin/tap/termbook
# or
go install github.com/tiulpin/termbook/cmd/termbook@latest
```

Linux and macOS. PTY capture uses `creack/pty`; Windows is not supported yet.

## Quickstart

```sh
termbook record "kubectl get pods" --id get-pods --cat Pods
termbook record "kubectl describe pod web-0" --id describe-pod --cat Pods
termbook build
```

The first `record` scaffolds `.termbook/termbook.yml`. Captures land in `.termbook/captures/<id>.ansi`. Commit them or don't – `build` works either way, so a teammate without access to the tool you're documenting can still rebuild the page.

## Manifest

`.termbook/termbook.yml`:

```yaml
title: kubectl screens
accent: "#326CE5"
github: https://github.com/kubernetes/kubectl
width: 120
categories:
  - name: Pods
    id: pods
    screens:
      - id: get-pods
        title: list pods
        command: kubectl get pods
      - id: describe-pod
        title: pod details
        command: kubectl describe pod web-0
```

`record` upserts entries by `id`. To edit titles, descriptions, or grouping, change the YAML directly and re-run `termbook record --only <id>` to refresh the capture.

## Commands

| Verb | Flags |
|------|-------|
| `record <cmd...>` | `--id`, `--cat`, `--title`, `--desc`, `--width` (default 120), `--timeout` (default 30s) |
| `record --only <id>` | Re-capture an existing entry without touching the manifest |
| `build` | `-o <path>` (default `gallery.html`), `--only <id>` to render a single screen |

The PTY is started with `TERM=xterm-256color`, `COLORTERM=truecolor`, and a fixed `COLUMNS`, so tools that disable color outside a terminal still emit ANSI.

## GitHub Action

```yaml
- uses: tiulpin/termbook@v0.2.0
  with:
    output: site/index.html
- uses: actions/upload-pages-artifact@v3
  with: { path: site }
```

The action installs the released binary and runs `build` against committed captures. To re-record commands in CI, run `termbook record --only <id>` for each entry before the action.

## Library API

For programmatic capture in Go:

```go
book := termbook.New("mycli – screen gallery",
    termbook.WithGitHub("https://github.com/you/mycli"),
)

book.Category("Users", "users",
    termbook.Scr("user-list", "user list", "List all users",
        "mycli user list", capturedANSIOutput),
)

book.Generate("docs/gallery/index.html")
```

`Scr` takes an ANSI string. `Manual` takes a writer:

```go
termbook.Manual("login", "login", "Auth flow", "mycli login", func(w io.Writer) {
    fmt.Fprintf(w, "✓ Logged in as admin\n")
})
```

Options:

```go
termbook.WithGitHub("https://github.com/...")
termbook.WithAccent("#FF6B6B")
termbook.WithDefaultTheme("light")               // default "dark"
termbook.WithFont("Fira Code", "https://fonts.googleapis.com/css2?family=Fira+Code:wght@400;500;700&display=swap")
termbook.WithCSS(".terminal pre { font-size: 14px }")
termbook.WithTemplate(myHTML)
```

## See also

- [terminal-to-html](https://github.com/buildkite/terminal-to-html) by Buildkite – standalone CLI to convert ANSI terminal output to HTML
- [VHS](https://github.com/charmbracelet/vhs) by Charm – record terminal sessions as GIFs/MP4s from tape files
