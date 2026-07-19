# RouteCause local product interface

Status: implementation contract for milestone 2

## Product posture

RouteCause should feel like a calm consumer utility: one obvious action, a visible connection path, plain language, and technical depth only when requested. It must never resemble a monitoring dashboard or imply certainty the evidence does not support.

The product contract is: “RouteCause measures each layer of your connection and reports what happened during the test. It does not claim to prove the underlying root cause.” The first screen contains one dominant Full 60-second Snapshot action, the five measured layers, and a small privacy statement. Demo mode uses a persistent amber banner and labels every result as a demonstrated observation pattern.

## Visual system

- Typography: system sans-serif with a compact serif accent for the RouteCause wordmark and result headline. Body text remains highly readable at 16–18 px.
- Canvas: warm off-white background, near-black text, white only for the focused result surface.
- Colour: deep ink and restrained teal as the primary action; green, amber, red, and grey appear only as semantic status colours.
- Layout: centered reading column, generous vertical rhythm, minimal borders, and no dashboard grid. The connection path is one responsive sequence, not a chart.
- Motion: a subtle testing pulse and status transition only. `prefers-reduced-motion` disables both.
- Controls: native semantic buttons, radio controls, disclosure elements, and visible keyboard focus. Minimum touch target is 44 px.

## Information states

During collection a layer may be waiting or testing. Final cards are Wi-Fi/Ethernet, Gateway, DNS, Internet and Tested destinations, and use only Healthy, Issue observed, Not measured or Insufficient data. There is no percentage because collectors have different schedules and completion counts.

The result hierarchy is:

1. factual Test summary;
2. five layer observations;
3. export action;
4. confidence, alternative explanations, evidence, robust statistics and limitations inside Advanced report.

History stores and displays only privacy-safe summary fields: session ID, timestamps, duration, diagnosis code/conclusion, confidence, demo flag, and final layer states. It never uses browser storage.

## Responsive and accessibility behavior

The connection path is horizontal when space permits and vertical on narrow screens. Content order remains identical. Status is conveyed by text and icon shape, never colour alone. Live progress uses a polite ARIA live region; completion and errors use assertive announcements. Focus moves only after explicit navigation, never as a side effect of SSE events.
