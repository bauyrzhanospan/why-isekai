---
title: "pre-commit.com hook, lefthook and husky recipes"
status: done
labels: [integration, phase-2]
---
Many teams install hooks only through these managers; `isekai init` writing into `.git/hooks` is invisible to them.

- [ ] `.pre-commit-hooks.yaml` in this repo with `commit-msg` and `prepare-commit-msg` stages (language: golang).
- [ ] lefthook and husky snippets in README.
- [ ] Test: a repo using pre-commit gets the same rejections as `isekai init`.

**Status (2026-11-03):** Done in 285d21e: .pre-commit-hooks.yaml (verified with pre-commit 4.7 try-repo), lefthook and husky recipes in README.
