"""Defines the unprivileged user-facing CLI and translates commands into task requests.

Technical requirements:
- Keep privileged Firecracker operations behind controller interfaces.
- Require explicit review before applying returned patches.
- Clearly label any non-microVM backend as unsafe.
- Never accept provider secrets as command-line arguments because process lists expose them.
"""

from __future__ import annotations

import argparse
from collections.abc import Sequence


def build_parser() -> argparse.ArgumentParser:
    """Create the command-line parser without reading configuration or credentials."""
    pass


def main(argv: Sequence[str] | None = None) -> int:
    """Validate CLI input and dispatch to the host controller once it is implemented."""
    pass
