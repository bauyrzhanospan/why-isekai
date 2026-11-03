---
title: "Homebrew tap, Scoop bucket, winget manifest"
status: partial
labels: [packaging, phase-2]
---
- [ ] Repo `homebrew-tap`, GoReleaser `brews`/`homebrew_casks` section, token secret.
- [ ] Repo `scoop-bucket`, GoReleaser `scoops` section.
- [ ] winget manifest (GoReleaser `wingets`) — needs a fork of microsoft/winget-pkgs.
- [ ] Later: submit to homebrew-core once there are users.

**Status (2026-11-03):** GoReleaser config done in 25fe372 (formula and manifest generated in a snapshot). Left: create the tap/bucket repositories and tokens; winget after the first release.
