#!/usr/bin/env python3
"""PreToolUse hook (Edit|Write|MultiEdit): durable documents never land in scratch directories.

Adapted from the team board's hook. The durable homes here are meta/ (state, plan, ADRs,
handoffs), DOCTRINE.md, CLAUDE.md, and - for Louis only - DECISIONS.md, SPEC.md, CLI.md and
spec/. Session scratch belongs in the harness scratchpad outside the repository.
Exit 2 = block (stderr fed back to the model); exit 0 = allow; any exception = allow.
"""
import json
import os
import sys

BLOCKED_EXTENSIONS = {".md", ".markdown", ".txt", ".rst", ".adoc", ".html"}
SCRATCH_PREFIXES = ("tmp/", "temp/", "scratch/", "notes/")


def main():
    try:
        payload = json.load(sys.stdin)
        file_path = payload.get("tool_input", {}).get("file_path")
        if not file_path:
            sys.exit(0)
        project = os.path.realpath(os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd())
        target = os.path.realpath(os.path.join(project, file_path))
        if not target.startswith(project + os.sep):
            sys.exit(0)  # outside the repository: guard-outside.py decides
        rel = target[len(project) + 1:].replace(os.sep, "/").lower()
        if os.path.splitext(rel)[1] in BLOCKED_EXTENSIONS and rel.startswith(SCRATCH_PREFIXES):
            sys.stderr.write(
                "BLOCKED: scratch directories are disposable; documents never go there. "
                "Plans, state, ADRs, handoffs -> meta/ (see meta/README.md); project law -> "
                "DOCTRINE.md. Session scratch -> the harness scratchpad directory.\n"
            )
            sys.exit(2)
        sys.exit(0)
    except SystemExit:
        raise
    except Exception:
        sys.exit(0)


if __name__ == "__main__":
    main()
