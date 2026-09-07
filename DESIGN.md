# Design

## Theme

Operational internal-tool interface using Algérie Télécom blue with green for healthy states, amber for attention, and red only for destructive actions. Surfaces stay light (`brand-ground` page, white panels) with navy headings and muted secondary text.

## Colors

| Token | Role | Value |
| --- | --- | --- |
| `brand-blue` / `brand-blue-hover` | Primary actions, links | `#003da5` / `#002e80` |
| `brand-navy` | Headings, strong labels | `#172b4d` |
| `brand-ink` | Body text | `#182536` |
| `brand-muted` | Secondary copy | `#536274` |
| `brand-line` | Borders / dividers | `#dbe3ed` |
| `brand-ground` | Page background | `#f5f8fb` |
| `brand-green` / `brand-green-soft` | Healthy / live | `#087f44` / `#e7f6ed` |
| `brand-amber` / `brand-amber-soft` | Attention / expected-up | `#9a5500` / `#fff5dd` |
| `brand-red` / `brand-red-soft` | Destructive only | `#bd2d2d` / `#fff0f0` |

## Typography

Inter (system fallbacks) with a compact product scale. Monospace is reserved for ports, paths, hostnames, and slugs. Headings use slight negative tracking (`tracking-tight`) and `text-wrap: balance`.

## Layout

Desktop sidebar (250px) with hub content column (`max-width: 1360px`, `padding: 34px`) matching the prototype. Project list and detail use `web/static/hub.css` classes ported from `prototype/styles.css` (panels, toolbar, table, badges, state pills, hero, detail grid, tracking, archive).

## Components

Buttons (primary blue, secondary bordered, danger red), status badges with leading dots, lifecycle state pills, data tables, definition-list panels, forms, inline archive confirmation.

## Motion

150–200ms ease-out on color/opacity and archive confirm reveal. Respect `prefers-reduced-motion: reduce`.
