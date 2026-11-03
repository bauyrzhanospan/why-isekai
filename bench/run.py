#!/usr/bin/env python3
"""why-isekai benchmark: does the journal save a fresh agent session work?

Each task is a small repository whose history ends with an open task and a
dead end that session 1 already hit. Session 2 is a fresh agent run with
the same prompt in two arms:

  plain    the same commits with subject lines only (no trailers)
  journal  the commits with Why/Tried/Next trailers, `isekai init --claude`
           (brief at session start) and the AGENTS.md line

Measured per run: success (the task's check command), tokens and turns
(when the agent CLI reports them), wall time, and whether the agent went
down the recorded dead end (patterns in its transcript or its diff).

  python3 bench/run.py --self-test
  python3 bench/run.py --agent 'claude -p {prompt} --output-format stream-json --verbose --allowedTools Bash,Edit,Write,Read' --runs 3
  python3 bench/analyze.py bench/results.jsonl > docs/benchmark-results.md

Running agents costs tokens: nothing runs an agent unless you pass --agent.
Standard library only.
"""
import argparse
import importlib.util
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time

try:
    import tomllib
except ImportError:  # Python < 3.11
    tomllib = None

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)


def load_toml(path):
    with open(path, "rb") as f:
        if tomllib:
            return tomllib.load(f)
    # minimal fallback: key = "string" | [list of strings]
    out = {}
    for line in open(path, encoding="utf-8"):
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        k, v = line.split("=", 1)
        out[k.strip()] = json.loads(v.strip())
    return out


def load_py(path):
    spec = importlib.util.spec_from_file_location(os.path.basename(path)[:-3], path)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


def tasks(selected):
    out = []
    for name in sorted(os.listdir(os.path.join(HERE, "tasks"))):
        d = os.path.join(HERE, "tasks", name)
        if selected and name not in selected:
            continue
        t = load_toml(os.path.join(d, "task.toml"))
        t["dir"] = d
        out.append(t)
    return out


def sh(cmd, cwd, env=None, check=True, timeout=None):
    r = subprocess.run(cmd, cwd=cwd, env=env, shell=isinstance(cmd, str), capture_output=True, text=True, timeout=timeout)
    if check and r.returncode != 0:
        raise RuntimeError("%s failed: %s%s" % (cmd, r.stdout, r.stderr))
    return r


def write_files(repo, files):
    for p, content in files.items():
        full = os.path.join(repo, p)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w", encoding="utf-8") as f:
            f.write(content)


def build_isekai():
    out = os.path.join(tempfile.mkdtemp(prefix="isekai-bench-bin-"), "isekai")
    sh(["go", "build", "-o", out, "./cmd/isekai"], cwd=ROOT)
    return os.path.dirname(out)


def make_repo(task, arm, env):
    repo = tempfile.mkdtemp(prefix="bench-%s-%s-" % (task["id"], arm))
    sh(["git", "init", "-q", "-b", "main"], repo, env)
    seed = load_py(os.path.join(task["dir"], "seed.py"))
    for files, subject, trailers in seed.COMMITS:
        write_files(repo, files)
        msg = subject
        if arm == "journal":
            msg += "\n\n" + "\n".join(trailers)
        sh(["git", "add", "-A"], repo, env)
        sh(["git", "commit", "-q", "--no-verify", "-m", msg], repo, env)
    if arm == "journal":
        sh(["isekai", "init", "--claude"], repo, env)
        sh(["git", "add", "-A"], repo, env)
        sh(["git", "commit", "-q", "--no-verify", "-m", "chore: isekai\n\nWhy: agents read the project memory at session start"], repo, env)
    base = sh(["git", "rev-parse", "HEAD"], repo, env).stdout.strip()
    with open(os.path.join(repo, ".git", "bench-base"), "w") as f:
        f.write(base)
    return repo


def bench_env(isekai_dir):
    env = dict(os.environ)
    env["PATH"] = isekai_dir + os.pathsep + env.get("PATH", "")
    env.update({"GIT_AUTHOR_NAME": "bench", "GIT_AUTHOR_EMAIL": "bench@example.com",
                "GIT_COMMITTER_NAME": "bench", "GIT_COMMITTER_EMAIL": "bench@example.com"})
    for k in ("ISEKAI_MODE",):
        env.pop(k, None)
    return env


def check(task, repo, env):
    return sh(task["check"], repo, env, check=False, timeout=300).returncode == 0


def dead_end_hit(task, text):
    return any(re.search(re.escape(p), text, re.I) for p in task.get("dead_end_patterns", []))


def usage_from(output):
    """Best-effort token and turn counts from Claude Code JSON or stream-json output."""
    tokens = turns = cost = None
    for line in output.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            o = json.loads(line)
        except ValueError:
            continue
        if o.get("type") == "result" or "usage" in o and "num_turns" in o:
            u = o.get("usage") or {}
            tokens = sum(u.get(k, 0) or 0 for k in ("input_tokens", "output_tokens", "cache_creation_input_tokens", "cache_read_input_tokens")) or None
            turns = o.get("num_turns")
            cost = o.get("total_cost_usd")
    return tokens, turns, cost


def run_agent(task, arm, agent, env, timeout):
    repo = make_repo(task, arm, env)
    cmd = agent.replace("{prompt}", shlex.quote(task["prompt"]))
    t0 = time.time()
    try:
        r = subprocess.run(cmd, cwd=repo, env=env, shell=True, capture_output=True, text=True, timeout=timeout)
        out = r.stdout + "\n" + r.stderr
    except subprocess.TimeoutExpired as e:
        out = (e.stdout or "") if isinstance(e.stdout, str) else ""
    wall = round(time.time() - t0, 1)
    base = open(os.path.join(repo, ".git", "bench-base")).read().strip()
    diff = sh(["git", "diff", base], repo, env, check=False).stdout + sh(["git", "status", "--porcelain"], repo, env, check=False).stdout
    tokens, turns, cost = usage_from(out)
    res = {
        "task": task["id"], "arm": arm, "success": check(task, repo, env),
        "dead_end": dead_end_hit(task, out + "\n" + diff),
        "tokens": tokens, "turns": turns, "cost_usd": cost, "wall_s": wall,
    }
    shutil.rmtree(repo, ignore_errors=True)
    return res


def self_test(ts, env):
    for t in ts:
        for arm in ("plain", "journal"):
            repo = make_repo(t, arm, env)
            brief = sh(["isekai", "brief"], repo, env).stdout
            has = t["dead_end"].lower() in brief.lower()
            assert has == (arm == "journal"), "%s/%s: dead end in brief = %s" % (t["id"], arm, has)
            assert not check(t, repo, env), "%s/%s: check passes before any work" % (t["id"], arm)
            write_files(repo, load_py(os.path.join(t["dir"], "solution.py")).FILES)
            assert check(t, repo, env), "%s/%s: reference solution fails the check" % (t["id"], arm)
            shutil.rmtree(repo, ignore_errors=True)
        print("ok", t["id"])
    print("bench self-test ok: %d tasks" % len(ts))


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--agent", help="agent command; {prompt} is replaced by the quoted task prompt")
    ap.add_argument("--runs", type=int, default=3)
    ap.add_argument("--tasks", nargs="*", help="task ids (default: all)")
    ap.add_argument("--arms", nargs="*", default=["plain", "journal"])
    ap.add_argument("--timeout", type=int, default=900)
    ap.add_argument("--out", default=os.path.join(HERE, "results.jsonl"))
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args()
    env = bench_env(build_isekai())
    ts = tasks(a.tasks)
    if a.self_test:
        self_test(ts, env)
        return
    if not a.agent:
        ap.error("--agent is required (or --self-test); running agents costs tokens")
    with open(a.out, "a") as f:
        for i in range(a.runs):
            for t in ts:
                for arm in a.arms:
                    res = run_agent(t, arm, a.agent, env, a.timeout)
                    res["run"] = i + 1
                    f.write(json.dumps(res) + "\n")
                    f.flush()
                    print(json.dumps(res))


if __name__ == "__main__":
    main()
