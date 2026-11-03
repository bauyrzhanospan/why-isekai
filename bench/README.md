# Benchmark: with the journal vs without

Does a fresh agent session do better when the history carries Why / Tried / Next? This harness measures it. The method and published results live in [docs/benchmark.md](../docs/benchmark.md).

```sh
python3 bench/run.py --self-test        # builds isekai, checks every task's seed and reference solution
python3 bench/run.py --runs 3 \
  --agent 'claude -p {prompt} --output-format stream-json --verbose --allowedTools Bash,Edit,Write,Read'
python3 bench/analyze.py bench/results.jsonl
```

Running agents costs tokens; nothing runs an agent unless you pass `--agent`. `{prompt}` is replaced by the task prompt, already shell-quoted. Any agent CLI works; token and turn counts are read from Claude Code's JSON output when present, otherwise left empty.

## Tasks

Each folder in `tasks/` has:

- `seed.py`: the history of "session 1". `COMMITS` is a list of `(files, subject, trailers)`. The plain arm gets subjects only; the journal arm also gets the trailers and `isekai init --claude`.
- `task.toml`: `prompt` (identical in both arms), `check` (a shell command that succeeds when the task is done right), `dead_end` (the text of the recorded `Tried`) and `dead_end_patterns` (regexes that show the agent went down that path, matched against its transcript and diff).
- `solution.py`: a reference solution, so `--self-test` can prove the check is passable and fails before the work.

Good tasks have a dead end that looks like the natural first move and fails or breaks a rule the check enforces. Aim for 10–20 tasks before publishing numbers.
