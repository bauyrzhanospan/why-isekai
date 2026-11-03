---
title: "Secrets store adapters: dotenvx, 1Password CLI, Vault, Infisical"
status: partial
labels: [secrets, later]
---
Spec §2.2.1. Each adapter shells out to the vendor CLI (no own crypto, no SDK network code), maps `scope/name` to the vendor's addressing, and returns the §2.5.1 error kinds (not found / denied / tool not installed with install command / no key). Tests use fake CLIs on PATH.

**Status (2026-11-03):** Presets for dotenvx, 1Password, Vault and Infisical plus a generic `command` store (argv templates, no shell), tested with a fake CLI. Left: verify each preset against a real account and adjust flags.
