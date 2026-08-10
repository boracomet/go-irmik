---
title: Rendering modes
description: SSR, SSG, ISR, Static, and CSR at a glance.
date: 2026-08-05T12:00:00Z
---

## Pick a mode per route

| Mode | When to use |
|------|-------------|
| **SSR** | Personalized or always-fresh pages |
| **SSG** | Marketing / docs that change rarely |
| **ISR** | Content that can be cached then revalidated |
| **Static** | Pure files from `public/` or `out/` |
| **CSR** | Shell + client islands only |

Set the mode in `_meta.yaml` next to `page.html`.
