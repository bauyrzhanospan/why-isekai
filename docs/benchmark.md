# Benchmark: does the journal help?

**Status: harness ready, not run yet.** No numbers are published, and the dashboard shows "tokens saved: not measured yet" until they are.

## Method

- Tasks are small repositories whose history ends with an open task (a `Next`) and a dead end that a previous session already hit (a `Tried`). See [bench/README.md](../bench/README.md).
- A fresh agent session gets the same prompt in two arms:
  - **plain**: the same commits with subject lines only;
  - **journal**: the commits with Why/Tried/Next trailers, plus `isekai init --claude`, so the brief arrives at session start.
- Measured per run: success (the task's check), tokens, turns and time, and whether the agent repeated the recorded dead end.
- At least 3 runs per task per arm, at least 10 tasks, with the agent CLI version and the model recorded with the results.

## Results

Not run yet. To produce them:

```sh
python3 bench/run.py --runs 3 --agent '<agent command with {prompt}>'
python3 bench/analyze.py bench/results.jsonl
```

## How "tokens saved" will be estimated

`tokens saved ≈ briefs served in a clone × (mean tokens without the journal − mean tokens with it)`. Both means come from this benchmark. The estimate is labelled as an estimate, links here, and is only shown once results exist.
