# otrv.dev

My personal website. A static site generator in ~200 lines of Go.

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

## Building locally

```
go run -mod=readonly .
```

Output goes to `public/`.

## Amp orbs

`.agents/setup` installs the Go version from `go.mod` and downloads the modules
listed in `go.mod` and `go.sum`. It builds the site to warm the build cache.
Amp snapshots the installed toolchain and caches for later orbs.

Run `.agents/setup` again after changing dependencies. Repeated runs reuse the
installed toolchain and caches. `.agents/resume` does no work because the site
needs no services or authentication.

## Deploying

Push to main. GitHub Actions handles the rest.
