# Quiet Ink for YourMove Realize

**Not a copy of the full YourMove product shell.** This is a focused, editorial-feeling standalone demonstration borrowing the recognizable Quiet Ink palette and restraint. It needs to be memorable through clarity, pacing and typographic confidence—not gradients and AI dashboard noise.

## Tokens

Source of truth for this pack: `design/quiet-ink-tokens.css`.

| Element | Light | Dark |
| --- | --- | --- |
| Canvas | `#F5F4F1` | `#191C22` |
| Surface | `#FFFFFF` | `#22262E` |
| Text | `#242933` | `#EDF0F5` |
| Muted | `#606773` | `#AEB6C2` |
| Border | `#D9DDE3` | `#343A45` |
| Action | `#3459A5` | `#AAC5FF` |
| Action hover | `#284984` | `#C1D5FF` |
| Positive | `#2C694D` | `#A3D7B9` |

No Meta co-branding in the interface. Text wordmark **YourMove / Realize** until the official logo is supplied. Keep its replacement slot uncomplicated and preserve the baseline alignment.

## Layout

- Primary canvas with one deliberate page-level content width (~1120px desktop); no navigation sidebar.
- Headline 56–72px desktop, ~38–46px mobile, tracking slightly tight; headline uses a restrained serif for editorial character only if it doesn't conflict with existing branding (a well-set system sans is a safe default).
- Body 16–18px with relaxed line height; wide generous spacing; each screen has one dominant action.
- Small uppercase eyebrow text for orientation; prefer typography over badges.
- Upload area is large, inviting, and unmistakable; drag/drop affordance and native file picker are both usable.
- Conversation should feel like interviewing one capable human: one question prominently in focus, previous turns visible but secondary. Avoid animated fake typing or 'thinking' theatre.
- Results page should feel like an editorial revelation: a large insight headline, small source-based evidence, clear limitations, and a simple evidence reveal. No percentages, fake confidence meters or skill gamification.
- Evidence panel is supportive, not the visual centerpiece. Exact candidate wording has a subtle quoted-paper treatment.
- Responsive: on narrow screens, stack panels and keep the primary action within reach; avoid horizontal overflow.

## Theme behavior

- Honor `prefers-color-scheme` for initial appearance.
- Allow immediate light/dark override via a simple toggle; persist that **non-sensitive preference** in `localStorage` if useful.
- Both themes are first-class: check action contrasts, border separation, focus states and subtle cards independently.
- Reduced-motion users get minimal/disabled transitions. Motion should be gentle (150–220 ms), never block navigation or delay data display.

## Build guidance

`design/visual-reference.html` demonstrates the intended visual weight and rhythm as a static page. Treat it as **visual reference**, not functional application code to copy wholesale. The Go app should render its own actual state. A striking results screen is more valuable than an extra feature.
