# Islands scaffold (Vite + React)

Starter files for Irmik React/Vite islands. Copy into an app root (or start from `templates/scaffold/`).

## Layout

```
app/                 # html/template pages (Go)
islands/
  _hydrate.tsx       # shared helper — NOT a Vite entry (underscore prefix)
  Counter.tsx        # island entry: data-island="Counter"
vite.config.ts
package.json
public/islands/      # Vite outDir → manifest.json + hashed assets
```

## Template usage

```html
<head>
  {{ islandRuntime }}
</head>
<body>
  {{ island "Counter" (dict "initial" 0) }}
</body>
```

`island` emits:

```html
<div data-island="Counter" data-props='{"initial":0}'></div>
<script type="module" src="..."></script>
```

- **Dev:** `src` = `{Islands.DevServer}/islands/Counter.tsx` (default `http://localhost:5173`)
- **Prod:** Vite `manifest.json` under `Islands.OutDir` (`public/islands` or `public/islands/.vite/`)

## Vite multi-entry

`vite.config.ts` globs `islands/*.{tsx,jsx}` (skip `_*.tsx`) as Rollup inputs with `build.manifest: true` and `outDir: "public/islands"`.

## Expected manifest shape

```json
{
  "islands/Counter.tsx": {
    "file": "assets/Counter-abc123.js",
    "src": "islands/Counter.tsx",
    "isEntry": true,
    "css": ["assets/Counter-def456.css"],
    "imports": ["_shared-xyz.js"]
  },
  "_shared-xyz.js": {
    "file": "assets/shared-xyz.js"
  }
}
```

Go resolves island `"Counter"` to `islands/Counter.tsx` (or a named input key) and emits `/islands/{file}` plus CSS links.
