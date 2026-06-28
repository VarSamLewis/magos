"""Loads and validates host configuration without exposing secrets to task models.

Technical requirements:
- Configuration precedence must be explicit: CLI, config file, then safe defaults.
- Secret values must be referenced by provider/key identifier, not copied into logs or tasks.
- Resolve and validate all filesystem paths before privileged runtime operations.
- Reject debug Firecracker builds and unsafe production defaults.
"""

from dataclasses import dataclass
from pathlib import Path

@dataclass(frozen=True, slots=True)
class HostConfig:
    firecracker_binary: Path
    jailer_binary: Path
    kernel_image: Path
    rootfs_image: Path
    state_directory: Path
    audit_database: Path
    credential_store: str

    def validate(self) -> None:
        """Validate static configuration before a task can allocate resources."""
        pass


def load_config(config_path: Path | None = None) -> HostConfig:
    """Load safe host defaults and optional file overrides without loading task secrets."""
    pass


def validate_runtime_paths(config: HostConfig) -> None:
    """Verify runtime paths are absolute, trusted, correctly typed, and not guest-controlled."""
    pass
