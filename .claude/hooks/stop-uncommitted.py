#!/usr/bin/env python3
"""Stop hook: blocks ending a turn with uncommitted tracked changes (DECISIONS 28).

Ported from the team board's hook, same behaviour. Exit 2 = block, stderr is fed back to the
model; stop_hook_active guards against loops. Outside a git repository there is nothing to
check and the hook allows. Any unexpected error is swallowed and treated as allow: a bug in
this hook must never trap a session.
"""
import json
import os
import subprocess
import sys


def main():
    try:
        payload = json.load(sys.stdin)
        if payload.get("stop_hook_active"):
            sys.exit(0)
        repo = os.environ.get("CLAUDE_PROJECT_DIR") or os.path.abspath(
            os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "..")
        )
        result = subprocess.run(
            ["git", "status", "--porcelain", "-uno"],
            cwd=repo, capture_output=True, text=True, timeout=10,
        )
        dirty = [l for l in result.stdout.splitlines() if l] if result.returncode == 0 else []
        if dirty:
            sys.stderr.write(
                f"Tracked files have uncommitted changes ({len(dirty)}). Run "
                "`GOTOOLCHAIN=local make check`; if green, commit with a conventional message "
                "(fix commits carry symptom -> cause -> fix) and update meta/state.md if focus "
                "changed - or say explicitly in your reply why the work stays uncommitted.\n"
            )
            sys.exit(2)
        sys.exit(0)
    except SystemExit:
        raise
    except Exception:
        sys.exit(0)


if __name__ == "__main__":
    main()
