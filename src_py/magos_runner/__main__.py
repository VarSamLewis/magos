"""Package entry point for ``python -m magos_runner``.

Technical requirements:
- Delegate argument parsing and execution to ``cli.main``.
- Keep startup side-effect free until the selected command has validated its configuration.
"""

from .cli import main


if __name__ == "__main__":
    raise SystemExit(main())
