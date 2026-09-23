#!/usr/bin/env python3
"""SessionStart hook: prints a short orientation block (stdout is added to the context).

Adapted from the team board's hook (itself from admin's): branch, uncommitted count, recent
commits, the head of meta/state.md. Tolerates a directory that is not a git repository yet
(DECISIONS 28: configlang/ becomes a local repository) and a repository with no commits.
Never blocks: any failure is swallowed and the hook exits 0.
"""
import os
import subprocess
import sys

STATE_HEAD_LINES = 30


def run(args, cwd):
    try:
        result = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=10)
        if result.returncode != 0:
            return None
        return result.stdout.splitlines()
    except Exception:
        return None


def repo_dir():
    repo = os.environ.get("CLAUDE_PROJECT_DIR")
    if repo:
        return repo
    return os.path.abspath(os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", ".."))


def git_lines(repo):
    branch = run(["git", "rev-parse", "--abbrev-ref", "HEAD"], repo)
    if branch is None and run(["git", "rev-parse", "--git-dir"], repo) is None:
        return ["Not a git repository yet (DECISIONS 28): `git init` is an open Louis-call."]
    dirty = run(["git", "status", "--porcelain", "-uno"], repo) or []
    out = [f"Branch: {branch[0] if branch else '(none)'} | Uncommitted tracked changes: {len(dirty)}"]
    worktrees = run(["git", "worktree", "list"], repo) or []
    if len(worktrees) > 1:
        out.append(f"{len(worktrees)} worktrees active: check claims before editing "
                   "(orchestration.md, Worktrees).")
    commits = run(["git", "log", "--oneline", "-5"], repo)
    if commits:
        out.append("Recent commits:")
        out.extend(f"  {c}" for c in commits)
    else:
        out.append("No commits yet.")
    return out


def state_head(repo):
    path = os.path.join(repo, "meta", "state.md")
    if not os.path.isfile(path):
        return []
    with open(path, "r", encoding="utf-8", errors="replace") as f:
        lines = [line.rstrip("\n") for _, line in zip(range(STATE_HEAD_LINES), f)]
    return ["---"] + lines


def main():
    try:
        repo = repo_dir()
        out = ["## Session orientation (auto-generated)"]
        out.extend(git_lines(repo))
        out.extend(state_head(repo))
        sys.stdout.write("\n".join(out) + "\n")
    except Exception:
        pass
    sys.exit(0)


if __name__ == "__main__":
    main()
