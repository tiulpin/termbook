# termbook

Generate browsable HTML galleries from CLI terminal output. Like Storybook, but for command-line tools.

Captures ANSI in a real PTY and renders a static page with sidebar nav, dark/light themes, and a fuzzy filter. The same captures power `termbook diff`, which re-records on every PR, applies redaction, and posts a unified diff straight into PR review — [live example](https://github.com/tiulpin/termbook/pull/1#issuecomment-4363705991).

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
| `record --all` | Re-capture every entry in the manifest |
| `build` | `-o <path>` (default `gallery.html`), `--only <id>` to render a single screen |
| `diff` | `--baseline <dir>`, `--report-md <path>`, `--report-html <path>`, `--quiet`. Exits 1 when any screen changed |

The PTY is started with `TERM=xterm-256color`, `COLORTERM=truecolor`, and a fixed `COLUMNS`, so tools that disable color outside a terminal still emit ANSI.

## Diff and redaction

`termbook diff` re-runs nothing on its own — call `termbook record --all` first to refresh captures, then compare against the baseline (git HEAD by default). Volatile bits like timestamps drown out real changes, so the manifest carries a top-level `redact:` list of regex rules applied during comparison only:

```yaml
redact:
  - pattern: '\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}'
    replace: TIMESTAMP
  - pattern: '\b[a-f0-9]{40}\b'
    replace: COMMIT_HASH
```

Reports are written either as Markdown (`--report-md` — designed for `gh pr comment --body-file`) or self-contained HTML (`--report-html`).

For a real-world example, see the [comment posted on PR #1 of this repo](https://github.com/tiulpin/termbook/pull/1#issuecomment-4363705991): the diff workflow caught every change v0.3 made to the CLI's help text and the eza manifest's redact rules, with no hand-written content.

## GitHub Action

Two modes. `mode: build` renders a gallery; `mode: diff` re-records, compares, and writes a Markdown report you can post on the PR.

Build:

```yaml
- uses: tiulpin/termbook@v0.3.0
  with:
    output: site/index.html
- uses: actions/upload-pages-artifact@v3
  with: { path: site }
```

Diff with a sticky PR comment:

```yaml
on: pull_request

jobs:
  diff:
    runs-on: ubuntu-latest
    permissions: { contents: read, pull-requests: write }
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - id: diff
        uses: tiulpin/termbook@v0.3.0
        with:
          mode: diff
          baseline-ref: origin/${{ github.event.pull_request.base.ref }}
          fail-on-change: 'false'
      - if: steps.diff.outputs.changed == 'true'
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          path: ${{ steps.diff.outputs.report-md }}
          header: termbook
```

`baseline-ref` is what makes the comparison meaningful on a PR — it diffs the freshly recorded captures against whatever was committed on the base branch, not against the PR's own tip. The `header` keeps the bot's comment sticky across pushes; if you have several galleries to diff, give each its own header.

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
