# Irmik Blog example

Reference app for **Irmik Phase 1**: file-based routes, Markdown content, ISR, and a React/Vite island.

## Routes

| Path | Mode | Notes |
|------|------|-------|
| `/` | SSR | Home + Counter island |
| `/about` | SSG | Pre-rendered at build |
| `/blog` | SSR | Lists `content/posts` |
| `/blog/:slug` | ISR | Revalidate every 60s |

## Run (dev)

From this directory:

```bash
# optional: islands HMR
npm install
npm run dev          # Vite on :5173

# in another terminal
go run .             # Gin on :8080
```

Or use the CLI from the repo root after `go install` / `go run ./cmd/irmik`:

```bash
cd examples/blog
go run ../../cmd/irmik dev -c irmik.yaml
```

> Loaders in this example are registered in `main.go`. Plain `irmik dev` mounts pages without those loaders — prefer `go run .` for the full demo.

## Build

```bash
npm install && npm run build   # islands → public/islands
go run ../../cmd/irmik build -c irmik.yaml
```

SSG/ISR HTML lands in `out/` with `sitemap.xml` and `robots.txt`. Content-driven paths expand `/blog/:slug` from `content/posts`.

## Layout

```text
app/                 # pages + _meta.yaml
content/posts/       # Markdown + frontmatter
islands/Counter.tsx  # React island
public/style.css
irmik.yaml
main.go              # loaders + seoHead helper
```
