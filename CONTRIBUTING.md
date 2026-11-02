# Contributing

Agents and humans are both welcome. This repository eats its own dog food.

## Setup

```sh
go build -o "$(go env GOPATH)/bin/isekai" ./cmd/isekai   # put isekai on PATH
isekai init --mode block --claude                         # hooks for this clone
isekai brief                                              # what happened before you
```

## Rules

- Commit messages: [Conventional Commits](https://www.conventionalcommits.org) subject, then why-isekai trailers. `Why:` is required except for `style:`, `docs:` and `ci:` commits. Agents add `Agent: <model id>`. Close open items with `Done: <exact Next text>`.
- Code, README, comments and commit messages are in English. The spec has an English and a Russian version; keep both in sync.
- No telemetry and no network calls. The program must stay deterministic and never call an LLM.
- Do not change the meaning of the journal keys (`Why`, `Tried`, `Next`, `Done`, `Agent`) without agreement from the maintainer; they are the core of the standard.
- Tests run real git in temporary repositories, not mocks: `go test ./...`. CI runs them on Linux, macOS and Windows.
- Hook scripts must stay POSIX `sh` (they also run under Git Bash on Windows and dash on Debian).
- Packaging self-tests: `node packaging/npm/build.mjs --self-test`, `python3 packaging/pypi/build_wheels.py --self-test`.

## Releasing

Tag `vX.Y.Z` and push. The release workflow builds every platform with GoReleaser and, if the repository variables `PUBLISH_NPM` / `PUBLISH_PYPI` are `true`, publishes the npm packages (secret `NPM_TOKEN`) and PyPI wheels (trusted publishing, environment `pypi`).
