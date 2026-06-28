"""Defines immutable task, policy, resource, and result models shared by host services.

Technical requirements:
- Models crossing trust boundaries must have explicit schemas and bounded fields.
- Task and VM identifiers must be unguessable and unique.
- Policies are immutable after launch; changes require a new task capability.
- Never place provider credentials or other long-lived secrets in these models.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from enum import StrEnum
from pathlib import Path


class TaskState(StrEnum):
    CREATED = "created"
    PREPARING = "preparing"
    RUNNING = "running"
    VALIDATING = "validating"
    AWAITING_REVIEW = "awaiting_review"
    COMPLETED = "completed"
    FAILED = "failed"
    CANCELLED = "cancelled"
    QUARANTINED = "quarantined"


@dataclass(frozen=True, slots=True)
class ResourceLimits:
    vcpus: int = 1
    memory_mib: int = 1024
    process_count: int = 256
    writable_disk_mib: int = 4096
    wall_time_seconds: int = 1800
    idle_time_seconds: int = 300
    output_bytes: int = 10_000_000
    changed_files: int = 250
    patch_bytes: int = 5_000_000


@dataclass(frozen=True, slots=True)
class ModelPolicy:
    allowed_models: tuple[str, ...]
    maximum_tokens: int
    maximum_cost_minor_units: int
    currency: str = "GBP"


@dataclass(frozen=True, slots=True)
class PackagePolicy:
    allowed_registries: tuple[str, ...] = ()
    allowed_namespaces: tuple[str, ...] = ()
    lockfile_only: bool = True
    maximum_download_bytes: int = 0


@dataclass(frozen=True, slots=True)
class TaskPolicy:
    resources: ResourceLimits
    models: ModelPolicy
    packages: PackagePolicy = field(default_factory=PackagePolicy)
    allowed_mcp_tools: tuple[str, ...] = ()


@dataclass(frozen=True, slots=True)
class TaskRequest:
    task_id: str
    agent: str
    prompt: str
    project_root: Path
    base_revision: str
    created_at: datetime
    policy: TaskPolicy


@dataclass(frozen=True, slots=True)
class TaskResult:
    task_id: str
    state: TaskState
    exit_code: int | None
    patch_path: Path | None
    audit_id: str
    summary: str
