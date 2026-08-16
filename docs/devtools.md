# Dev overlay

When `cfg.IsDev()`, `irmik.New` injects a bottom-left badge into HTML responses.
Click it for:

- Template / window / unhandled-promise errors
- Discovered file routes
- Listen address and environment

`irmik dev` watches `app/` and `templates/`. A successful reload tells the
browser to refresh. A template parse error does not refresh; it appears in the
panel instead.

Island (Vite) compile errors use Vite’s own overlay. Production
(`env: production` / `irmik start`) does not mount `/_irmik/dev/*`.
