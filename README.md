# otrv.dev

My personal website, built with a Go static site generator.

## Adding a post

Create a markdown file in `posts/`:

```markdown
---
title: Your Title
date: 2025-12-29
description: A short summary.
---

Content goes here.
```

Use lowercase letters, digits, and single hyphens in filenames, such as
`your-title.md`. The filename becomes the post URL. Descriptions are plain text.
For a cover image, add `cover: images/cover.png` and place the image at
`static/images/cover.png`.

The generator rejects missing titles, invalid dates, missing covers, and output
filename collisions. It accepts up to 256 posts of 1 MiB each and up to 1,024
entries in `posts/`. Social image titles must fit within three lines at the
minimum font size.

The static tree accepts only regular files and directories, with no symlinks.
It allows up to 4,096 entries, eight nested directory levels, 16 MiB per file,
and 128 MiB total. The social image font has a separate 8 MiB read limit.
Exceeding a limit fails the build without replacing the previous site.

## Building locally

Use the Go version specified in `go.mod`.

```sh
go run -mod=readonly .
```

Output goes to `public/`, including nested files from `static/`. A successful
build replaces the previous output, so removed posts leave no stale pages or
social images. A failed render leaves the previous site unchanged.

## Checking changes

Run from the repository root:

```sh
gofmt -w *.go
go vet ./...
go test -race ./...
go run -mod=readonly .
```

Tests use temporary sites with real templates, Markdown, fonts, and files. They
check generated HTML, XML, JSON, PNG dimensions, input limits, repeated builds,
and failed builds. A source check rejects Go comments other than tool directives.

## Amp orbs

`.agents/setup` installs the Go version from `go.mod` and downloads the modules
listed in `go.mod` and `go.sum`. It builds the site to warm the build cache.
Amp snapshots the installed toolchain and caches for later orbs.

Run `.agents/setup` again after changing dependencies. Repeated runs reuse the
installed toolchain and caches. `.agents/resume` does no work because the site
needs no services or authentication.

## Deploying

Push to main. GitHub Actions checks formatting, runs vet and tests, scans for
reachable vulnerabilities, and builds the site before deploying to GitHub Pages.
Pull requests run the checks without deploying.
