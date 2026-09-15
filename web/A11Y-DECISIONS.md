# A11Y Decisions Log (Pattern Memory)

> **Purpose:** cross-turn memory of choices between **equally conformant alternatives**. Two implementations can both pass `A11Y.md` and axe and still diverge (different `role`, focus pattern, announcement wording) — twenty compliant modals, zero coherence. This file prevents that.

## Rules for the AI

1. **Record only what is not derivable** from `A11Y.md` rules or from the code itself. Landmarks, headings, and alt presence are machine-verifiable — they do **NOT** belong here.
2. **Index by pattern, never by screen.** ✅ *"Destructive confirmation modal → `alertdialog`"* · ❌ *"The dispute modal does X"*.
3. **One line per decision:** pattern → choice → short why.
4. **Read before building:** before generating any interactive component, check this log and reuse the recorded pattern (see *Component Reuse* in the AI Behavior Contract).
5. **Never fork silently:** if a new requirement contradicts a recorded decision, ask the user — do not create a parallel variant.
6. **Stay lean:** tens of lines, not hundreds. This file shares the context budget with Lazy Loading; if it grows past ~40 entries, consolidate.
7. **Versioned, never gitignored:** this is *shared* memory — across turns, agents and developers. A local-only copy per developer forks the patterns and defeats the file's entire purpose.

## Decisions

<!-- Append entries below. Format: - **Pattern** → choice — why. (date) -->

- **Interactive canvas visualisation (WebGL graph)** → canvas is one `role="img"` named by a visible summary sentence; every canvas action has a DOM twin (combobox search, neighbour listboxes, zoom/fit buttons, Escape) — a canvas has no per-item semantics, and the listboxes *are* the data alternative. (2026-09-14)
- **Previewing an item of a list on a linked visual** → preview on hover **and** keyboard focus of the option (`Row onHighlight`); commit on Enter/click, then move focus to the destination heading (`tabIndex={-1}`) — keyboard users get the same preview without committing. (2026-09-14)
- **Grouped options inside one listbox** → single listbox per list, visual group headers `aria-hidden`, relation carried in each option's accessible name — one tab stop and continuous arrow navigation instead of one listbox per group. (2026-09-14)
- **Edge direction encoding** → arrowhead shape (away / toward / none) carries direction; colour (ink #1D1F20 out, accent #006399 in, ink3 #676C6F mutual) only repeats it — out vs in colours are 2.56:1 to each other, but each is ≥5:1 on #FAFAFA and the Out/In lists repeat the split in text. (2026-09-14)
- **Context marks behind a focused subset** → focused edges/nodes ≥3:1 (ink/accent/slab); non-focused nodes drop to `line` #E3E5E6 as de-emphasis only; unfocused overview edges ink3 @ 75% (#8C9092, 3.09:1) — relationships stay perceivable at 3:1 when nothing is focused. (2026-09-14)
- **Default mark colour on #FAFAFA** → ink3 #676C6F (5.09:1), never ink4 #959A9D (2.72:1 fails SC 1.4.11). (2026-09-14)
- **Physics layout motion** → bounded 3 s run (under SC 2.2.2's 5 s), and under `prefers-reduced-motion` the canvas stays hidden with a text status until it stops; camera moves use `setState` instead of tweens. (2026-09-14)
- **MUI focus ring** → `MuiButtonBase` `.Mui-focusVisible` gets the global 2px slab outline and focused inputs a 2px border — MUI's `outline: 0` lives in the `mui` layer and beats the base `:focus-visible` rule. (2026-09-14)
- **Selected state of segmented toggles** → slab fill + ink-inverse text (as selected rail rows), not MUI's 8% tint — the tint is ~1.2:1 and hides which option is on. (2026-09-14)
