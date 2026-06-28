"""Builds and verifies immutable guest images and fresh task disks.

Technical requirements:
- Verify pinned hashes or signatures for kernels, root filesystems, tools, and agents.
- Maintain manifests, provenance, dependency versions, and an SBOM.
- Never reuse writable task storage or snapshots containing task data or capabilities.
- Sanitize archives and reject devices, sockets, and links that escape the staging root.
"""

from dataclasses import dataclass
from pathlib import Path
from typing import Protocol


@dataclass(frozen=True, slots=True)
class VerifiedImage:
    path: Path
    sha256: str
    manifest_id: str


class ImageStore(Protocol):
    def verify_image(self, path: Path, expected_sha256: str) -> VerifiedImage:
        """Verify image integrity and return immutable provenance metadata."""
        pass

    def get_rootfs(self, profile: str) -> VerifiedImage:
        """Return a verified read-only root filesystem for an approved profile."""
        pass

    def create_project_disk(self, task_id: str, source_directory: Path) -> Path:
        """Create a bounded fresh filesystem image containing sanitized project input."""
        pass

    def destroy_project_disk(self, task_id: str, disk_path: Path) -> None:
        """Delete only the ephemeral disk registered to the supplied task."""
        pass
