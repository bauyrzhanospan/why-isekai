---
title: "GitHub Action `why-isekai/check` in the Marketplace"
status: partial
labels: [integration, ci, phase-2]
---
`uses: bauyrzhanospan/why-isekai@v1` that downloads the release binary (no Go needed), runs `isekai check` over the PR range and writes a job summary listing commits without Why. Inputs: `mode`, `strict`, `range`.

**Done when** published in the Marketplace and this repo's own CI uses it.

**Status (2026-11-03):** Action done in 005bdeb (composite, job summary, used by this repo's CI). Left: publish to the Marketplace after the first release and tag v0.
