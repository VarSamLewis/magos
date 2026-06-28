"""Orchestrates the fail-closed host lifecycle for agent tasks.

Technical requirements:
- Allocate unique task, VM, capability, workspace, cgroup, UID/GID, and vsock identities.
- Persist state transitions before the corresponding external action.
- Revoke capabilities before cleanup and clean up on every terminal path.
- Treat policy denial, validation failure, and uncertain cleanup as failure or quarantine.
- Never apply or merge a returned patch; send it to explicit engineering review.
"""

from pathlib import Path
from typing import Protocol

from .domain import TaskRequest, TaskResult


class TaskController(Protocol):
    def create_task(self, request: TaskRequest) -> str:
        """Persist a validated task request and return its immutable task identifier."""
        pass

    def run_task(self, task_id: str) -> TaskResult:
        """Run preparation, microVM execution, validation, and review handoff."""
        pass

    def cancel_task(self, task_id: str) -> None:
        """Revoke task authority and terminate its microVM idempotently."""
        pass

    def approve_patch(self, task_id: str, destination: Path) -> None:
        """Apply an engineer-approved patch only after base and path checks succeed."""
        pass


def recover_incomplete_tasks(controller: TaskController) -> None:
    """Quarantine or clean up tasks left incomplete by a previous host process."""
    pass
