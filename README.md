# otrv.github.io

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
go run main.go
```

Output goes to `public/`.

## Deploying

Push to main. GitHub Actions handles the rest.
