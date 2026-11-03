---
title: "Release v0.1.0: push, green CI on 3 OS, register package names"
labels: [release, phase-0]
---
Nothing else matters until people can install it.

**Steps**
- [ ] Push `main`; CI matrix passes on ubuntu, macos, windows (fix what breaks; Windows Git Bash hooks are untested).
- [ ] Register `why-isekai` on npm (+ the 6 `why-isekai-<os>-<cpu>` names) and PyPI; set `NPM_TOKEN`, PyPI trusted publishing (environment `pypi`), repo variables `PUBLISH_NPM=true`, `PUBLISH_PYPI=true`.
- [ ] Tag `v0.1.0`; check the release has 6 archives + `checksums.txt`.
- [ ] Verify each install path from README on a clean machine: `go install`, `install.sh`, `install.ps1`, `npm i -g`, `pipx install`, `uvx why-isekai`.

**Done when** every row of the README install table works on a clean machine.
