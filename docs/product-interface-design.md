# RouteCause local product interface

Status: implementation contract for milestone 2

## Product posture

RouteCause should feel like a calm consumer utility: one obvious action, a visible connection path, plain language, and technical depth only when requested. It must never resemble a monitoring dashboard or imply certainty the evidence does not support.

The primary promise is: “Find out why your internet is slow—not merely how slow it is.” The first screen contains one dominant action, a short duration choice, the connection path, and a small privacy statement. Demo mode uses a persistent amber banner and labels every demo result; it cannot visually pass as a real diagnostic.

## Visual system

- Typography: system sans-serif with a compact serif accent for the RouteCause wordmark and result headline. Body text remains highly readable at 16–18 px.
- Canvas: warm off-white background, near-black text, white only for the focused result surface.
- Colour: deep ink and restrained teal as the primary action; green, amber, red, and grey appear only as semantic status colours.
- Layout: centered reading column, generous vertical rhythm, minimal borders, and no dashboard grid. The connection path is one responsive sequence, not a chart.
- Motion: a subtle testing pulse and status transition only. `prefers-reduced-motion` disables both.
- Controls: native semantic buttons, radio controls, disclosure elements, and visible keyboard focus. Minimum touch target is 44 px.

## Information states

Each connection layer is one of: waiting, testing, healthy, degraded, unavailable, or inconclusive. There is no percentage because collectors have different schedules and completion counts. During a run the interface shows elapsed wall time and a cancel action.

The result hierarchy is:

1. likely cause and confidence;
2. supporting and contradicting evidence;
3. recommended actions selected only from the typed diagnosis code;
4. alternatives and limitations;
5. export action;
6. advanced raw measurements inside a native disclosure panel.

History stores and displays only privacy-safe summary fields: session ID, timestamps, duration, diagnosis code/conclusion, confidence, demo flag, and final layer states. It never uses browser storage.

## Responsive and accessibility behavior

The connection path is horizontal when space permits and vertical on narrow screens. Content order remains identical. Status is conveyed by text and icon shape, never colour alone. Live progress uses a polite ARIA live region; completion and errors use assertive announcements. Focus moves only after explicit navigation, never as a side effect of SSE events.
