---
title: "Decide: implementation choices not fixed by the spec"
labels: [standard, decision]
---
Maintainer decision needed; then update SPEC.md and SPEC.ru.md.

1. Built-in age store format: JSON with readable names, each value encrypted separately (`.isekai/secrets.age.json`). Put it in the spec?
2. Wiki markers: English `(proposed)` / `(draft)`; Russian `(предложено)` / `(черновик)` also accepted. Spec says `(предложено)` only.
3. Agent env vars: our own `ISEKAI_AGENT`, and `AGENT=amp|goose|1` with value matching. Mention in the spec, or keep implementation-only?
