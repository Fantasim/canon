#!/usr/bin/env python3
"""PreToolUse hook (Edit|Write|MultiEdit|NotebookEdit): no file write outside this project.

Resource, Source and every sibling service are read-only from here (DECISIONS 29, DOCTRINE
section 8). Permission patterns cannot say "anywhere but this directory", so this hook does:
it allows a target inside the project, the system temp directory (the harness scratchpad) and
~/.claude (plans, memory), and blocks everything else, whatever its name.
Exit 2 = block (stderr fed back to the model); exit 0 = allow. A malformed payload allows: the
permission deny rules in settings.json remain the second line.
"""
import json
import os
import sys
import tempfile


def inside(path, root):
    root = os.path.realpath(root)
    return path == root or path.startswith(root + os.sep)


def main():
    try:
        payload = json.load(sys.stdin)
        tool_input = payload.get("tool_input", {})
        file_path = tool_input.get("file_path") or tool_input.get("notebook_path")
        if not file_path:
            sys.exit(0)
        project = os.path.realpath(os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd())
        target = os.path.realpath(os.path.join(project, os.path.expanduser(file_path)))
        allowed = (project, tempfile.gettempdir(), os.path.expanduser("~/.claude"))
        if any(inside(target, root) for root in allowed):
            sys.exit(0)
        sys.stderr.write(
            f"BLOCKED: {target} is outside this project. Resource, Source and sibling services "
            "are read-only from the Canon repository; write the need as a handoff in "
            "meta/handoff/ (see meta/handoff/README.md).\n"
        )
        sys.exit(2)
    except SystemExit:
        raise
    except Exception:
        sys.exit(0)


if __name__ == "__main__":
    main()
