# A11y Verification Report — Graph page (`/graph`)

This report compiles the compliance evidence for a given *Feature*, ensuring its development reached the "Certification Ready" definition.

> This report is a **versioned project record** — never add it to `.gitignore`. Evidence hidden from version control is not evidence: QA and leadership verify it before release, and audits read it after.

> **Marking legend (mandatory):**
> `[x]` verified, with the evidence described beside it · `[!]` verified and **failed** (fix it, or open an entry in `EXCEPTIONS.md`) · `[~]` partially verified, with what is missing written down · `[ ]` **not verified** — the reason MUST be written beside it.
> Marking `[x]` without reproducible evidence invalidates the whole report.

---

## 📌 Validation Context
- **Feature/Epic:** Graph page — WebGL relationship graph with out-/in-edge focus (`web/src/components/graph/*`, `web/src/app/graph/page.tsx`; shared: `CommunityRail.tsx` `Row`/`moveOptionFocus`, `theme.ts` focus + ToggleButton overrides, `AppShell.tsx` nav link)
- **Test Date:** 09/14/2026
- **Covers interface as of:** working tree on top of `de3c95f` (uncommitted change set that introduces `/graph`)
- **Compliance Status:** ⚠️ CONDITIONAL — screen-reader, voice-control, zoom/reflow and text-spacing checks are not yet run (see `[ ]` below)
- **Verification Independence:** fresh-context — *code audit by an `a11y-architect` subagent in a new context over the repo (Claude, same model family as the author); browser checks below were run by the authoring session (Claude Code + Playwright/Chromium) and are self-reported*

## 1. Technical Verification (Automated & Semantics)
- [~] **Axe-Core / Lighthouse:** `eslint` (with `jsx-a11y` via `eslint-config-next`) reports 0 problems on the change set. axe/Lighthouse were **not** run — no axe runner in the toolchain. *Owner: developer — run axe DevTools on `/graph` in both unfocused and focused states.*
- [x] **HTML Semantics:** Playwright accessibility snapshot of `/graph`: search is a `combobox "Find a node"`; filters are `group "Direction"` / `group "Relations"` of `button [pressed]`; neighbour rows are native `<button role="option">` inside `listbox "Out · N …"` / `listbox "In · N …"`; zoom controls are `button "Zoom in" | "Zoom out" | "Fit graph"`; the canvas is `figure` > `img` named by the summary sentence; links are `<a>`.
- [x] **Heading Hierarchy (H1-H6):** snapshot shows app `h1` (sidebar) → `h2 "Graph"` → `h3` focused node → `h4 "Out · N"` / `h4 "In · N"`; no skipped levels.

## 2. Tab Order and Focus Management
- [x] **Focus Indicator:** computed style on a keyboard-focused ToggleButton: `outline: solid 2px rgb(15, 15, 16)` (slab, 19.16:1 on white, 18.36:1 on `#FAFAFA`); neighbour rows use the global 2px `:focus-visible` ring (screenshot while arrowing the Out list); focused search input border is 2px ink (16.55:1).
- [~] **Logical Navigation:** verified search → Direction toggles by `Tab`; listbox is one tab stop with Arrow/Home/End roving (navigated); Enter on a row moves focus to the new node's `h3` (verified: `activeElement` = H3); Escape with focus in the panel returns focus to the search input (verified). Full Tab sweep of the page not recorded.
- [x] **Captured Focus (Modals/Overlays):** no modals. The combobox popup: Escape with the popup open closes only the popup (URL keeps `?node=`), a second Escape clears focus — verified.

## 3. Behavior and Task Return
- [ ] **Screen Reader Test:** not run — the authoring agent has no screen reader. *Owner: developer.* Task to run: search a paper, walk Out → In neighbours with arrows + Enter, clear with Escape.
  - Pair(s) used: — (suggested: NVDA + Firefox, VoiceOver + Safari)
  - Who ran it and when: —
- [~] **Voice Control:** names read against visible labels: toggle names equal their text; option names start with the visible title (`"<title>, similar to, cosine 0.25"`), satisfying SC 2.5.3; zoom IconButtons are named by their tooltip text (visible only on hover/focus), so voice users need "show names/numbers". No live voice tool run. *Owner: developer — macOS Voice Control.*
- [x] **Interactive states inventoried:**
  - Combobox: closed / open / option selected / cleared — navigated.
  - Direction toggle All / Out / In — navigated (In hides the Out list).
  - Relation toggles on/off — navigated (Nearest off: In 47 → 9).
  - Neighbour row rest / pointer hover / keyboard focus (edge highlighted on canvas) / activated (walk) — navigated.
  - "Show all N" collapsed / expanded — code-read only.
  - Canvas loading / settling / ready / WebGL-unavailable — navigated (settling under both motion preferences; WebGL forced off via `getContext` stub → fallback text, lists still work, no errors).
  - Unknown `?node=` ("Not in the current graph") and snapshot empty/error placeholders — code-read only.
- [~] **Status Change (`aria-live`):** snapshot confirms `role="status"` content updates (`"<title>: 11 out, 10 in"`, search match count). Actual announcement, and whether it collides with focus moving to the `h3`, not heard. *Owner: developer, with the screen-reader run.*
- [x] **Form Filling:** the only input (MUI `TextField` in Autocomplete) exposes name "Find a node" via its associated label (accessibility snapshot).

## 4. Visual Perception and Comprehension
- [x] **Text & UI Contrast:** computed with the WCAG relative-luminance formula (canvas ground `#FAFAFA`):

  | Pair | Ratio |
  |---|---|
  | out edge / concept node ink `#1D1F20` | 15.85:1 |
  | in edge accent `#006399` | 6.20:1 |
  | mutual edge, default paper node ink3 `#676C6F` | 5.09:1 |
  | unfocused overview edges ink3 @ 75% (composited `#8C9092`) | 3.09:1 |
  | focused node / hovered edge slab `#0F0F10` | 18.36:1 |
  | legend/caption text ink2 `#4E5355` | 7.48:1 |
  | selected ToggleButton: ink-inverse `#E4E6E7` on slab | 15.3:1 |
  | non-neighbour nodes while focused, line `#E3E5E6` | 1.21:1 — de-emphasis by design, recorded in `A11Y-DECISIONS.md` |

- [x] **Redundancy:** direction is encoded by arrowhead shape (away / toward / none) and repeated as text in the Out/In lists and legend; concepts use mono caps labels, not only a darker fill; the inline "Graph data (JSON)" link is permanently underlined.
- [ ] **Scale / Zoom:** 200% text resize and 320 CSS px reflow not verified. The graph toolbar wraps and the grid collapses to one column below `lg`, but the pre-existing `AppShell` sidebar is a fixed 240px, which likely blocks 320px reflow app-wide. *Owner: developer — confirm, and decide on an app-shell fix or an `EXCEPTIONS.md` entry.*

## 5. Time-Based Media and Motion
*No video or audio. The only motion is the force-layout settling and camera moves.*
- [x] **Classification:** N/A — no time-based media.
- [x] **Alternatives:** N/A — no time-based media.
- [x] **Autoplay and Moving Content:** layout motion is bounded to 3 s (`LAYOUT_MS`, under SC 2.2.2's 5 s); no audio; nothing flashes.
- [x] **Reduced Motion:** `page.emulateMedia({ reducedMotion: "reduce" })`: while settling the canvas computes `visibility: hidden` and "Arranging N nodes…" is shown; after the run the canvas becomes visible. Camera moves use `setState` (no tween) and inertia is 0.
- [x] **Text over Media:** N/A — no media.

## 6. Cognitive Load and Flow
*Single-screen exploration: no multi-step flow, authentication or time limit.*
- [x] **Nothing to remember:** focused node stays in the heading, and the URL (`?node=`) restores it on reload — verified.
- [x] **Nothing to retype:** N/A — no multi-step process.
- [x] **Help in the same place:** N/A — no help mechanism.
- [ ] **Text spacing:** SC 1.4.12 override not tested. *Owner: developer — apply a text-spacing bookmarklet on `/graph`; canvas-drawn labels are exempt, but panel rows truncate with ellipsis and should be checked.*
- [x] **Timing:** no time limits.
- [x] **Conflicting needs:** dimming non-neighbour nodes (low contrast) serves focus for low-vision and cognitive users at the cost of their visibility; the full data stays available in the lists and at ≥3:1 when nothing is focused — recorded in `A11Y-DECISIONS.md`.

---
## 📝 Assessment Notes or Known Blockers

- **Note 1 — fixed from the fresh-context audit:** V1 unfocused edges at 1.21:1 → ink3 @ 75% (3.09:1); V2 ToggleButton selected state was an 8% tint → slab fill with inverse text; V3 inline data link now permanently underlined. V4 (neighbour *node* role by colour) accepted as low: the connecting edge's arrowhead and the Out/In lists carry the same information without colour.
- **Note 2 — open, needs a human:** screen-reader pass (live-region announcement overlapping focus moving to the `h3`; `role="option"` on native buttons in VoiceOver), voice control on the tooltip-named zoom buttons, 200% zoom / 320px reflow (pre-existing fixed sidebar), and text spacing. The status stays CONDITIONAL until these are run; no `EXCEPTIONS.md` entry is opened because no violation has been knowingly accepted yet.
