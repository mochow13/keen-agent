---
description: Convert a finished trip plan into a polished, self-contained HTML page using the design system.
name: render-html-page
---

# Render HTML page

Activate when the trip plan is complete and the user needs a file they can open in a browser.

Save via `write_file` as `<Destination>_trip.html` (e.g. `Amsterdam_trip.html`).

## Hard constraints

- One file. All CSS inline in `<style>`, all JS inline (or no JS at all). No external CSS/JS files, no build step, no CDN libraries, no icon webfonts.
- Must open by double-click (`file://`): no `fetch()` of local files, no module imports, no web-only APIs for core content.
- The only external URLs allowed are content links: booking deep links, official ticket shops, operator timetables, Google Maps queries. Never a stylesheet, script, font, or image host.
- Never truncate the plan: every section from the output format (§5 of `prompts/system.md`) appears in the page.

## Design system

Tokens first, never hardcoded values outside `:root`. Standard palette — Tailwind slate neutrals + blue primary + amber warning (the most widely deployed web token system; all text pairs clear WCAG AA):

```css
:root{
  --ink:#0f172a; --muted:#64748b; --secondary:#475569; --bg:#f8fafc; --card:#ffffff;
  --primary:#2563eb; --primary-dark:#1d4ed8; --primary-tint:#eff6ff;
  --warn-bar:#d97706; --warn-bg:#fffbeb; --warn-line:#fde68a; --warn-ink:#92400e;
  --line:#e2e8f0;
}
```

- Backgrounds: light neutral `var(--bg)` page, white `var(--card)` content cards. No cream/paper tints, no dark-mode-only styling.
- Accent discipline: blue = links/buttons/headings accents, slate = headings/body/borders, amber = warnings (`notice` box) only. Green/red strictly for success/error status; never decorative.
- Typography (system stacks only, so the file works offline):
  - Headings: `-apple-system, "Segoe UI", Inter, Roboto, sans-serif`, tight `letter-spacing:-.02em`, `h1 clamp(32px,5vw,54px)`, `h2 24px`, `h3` card titles.
  - Body: same stack at browser default size, `line-height:1.6`.
  - Data/labels/prices/badges: `ui-monospace, "SF Mono", "Cascadia Code", Consolas, monospace` with `tabular-nums` — all prices, dates, and status labels are mono.
- Spacing/shape: content column `max-width:1020px` centered; cards `border:1px solid var(--line)`, `border-radius:var(--radius)`, `box-shadow:var(--shadow)`, `padding:22px`.
- Icons: inline Unicode symbols only (✈ 🏨 🗺️ 🎟️ 🚆 🍽️ ⚠️). No emoji walls, no external icon library. Mark decorative icons `aria-hidden="true"`.

## Page skeleton

1. Hero: deep neutral gradient (`#0f172a → #1e3a8a`, slate-900 → blue-900), eyebrow pill (trip dates), `h1` destination, one-line sub (route · party · budget), meta pills (dates, route, party, budget, assumptions).
2. Sticky section nav: anchor links to `#flights #stays #itinerary #tickets #notes #sources`.
3. Amber `notice` box: key caveat (e.g. unverified prices, winter hours) — always present, one sentence.
4. `#flights` / `#stays`: cards with price (mono, 22px, slate-900), party/date scope on every price ("€210/night, 2 adults, 12–15 Jun"), trade-offs list, booking-link button (`btn` blue, `ghost` variant for secondary).
5. `#itinerary`: vertical timeline — slate rail with blue numbered day nodes, one card per day (theme, sights + opening times, food areas, transfers with times).
6. `#tickets`: table or cards per sight — attraction, where to buy, price, slot advice, cancellation note. Links go to official shops verified via Exa.
7. `#notes` / `#sources`: winter/visa notes, then sources (URL + retrieved date per claim) and a re-check checklist.
8. Footer: "Plan generated <date> — re-check prices before paying."

## Responsive + print

- One breakpoint at `760px`: two-column card grids collapse to one column, nav wraps, hero padding shrinks.
- `@media print`: hide nav, remove hero gradient (dark ink on white), `break-inside:avoid` on cards, `body{font-size:10.5pt}`.

## Quality gates

- Valid `<!DOCTYPE html>` with `charset utf-8` + `viewport` meta; `<html lang="en">`.
- No `var(--undefined)` tokens, no bare hex/rgb outside `:root`, no `font-family`/`font-size` overrides outside the type roles above.
- Mobile readable at 360px, no horizontal overflow; print preview shows all content without clipped cards.
- Prices carry currency + party scope; every flight/stay/ticket has exactly one openable URL.
