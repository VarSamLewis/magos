"""Defines jailed Firecracker microVM creation, supervision, and destruction.

Technical requirements:
- Require Linux KVM, cgroup v2, production seccomp filters, jailer, and unique UID/GID.
- Attach only a read-only rootfs, fresh project disk, entropy device, and vsock by default.
- Do not attach TAP networking unless a fail-closed network policy explicitly requires it.
- Apply CPU, memory, PID, I/O, disk, output, idle, and wall-time limits before guest code.
- Partial startup and repeated cleanup must never affect another task's resources.
"""

from dataclasses import dataclass
from pathlib import Path
from typing import Protocol

from ..domain import ResourceLimits


@dataclass(frozen=True, slots=True)
class VMRequest:
    task_id: str
    vm_id: str
    uid: int
    gid: int
    rootfs: Path
    project_disk: Path
    jail_directory: Path
    limits: ResourceLimits


@dataclass(frozen=True, slots=True)
class VMHandle:
    vm_id: str
    process_id: int
    api_socket: Path
    vsock_socket: Path


class MicroVMRuntime(Protocol):
    def check_host(self) -> None:
        """Fail unless all required KVM, jailer, kernel, cgroup, and filesystem controls exist."""
        pass

    def start(self, request: VMRequest) -> VMHandle:
        """Start a fully constrained microVM or leave no runnable guest behind."""
        pass

    def wait(self, handle: VMHandle, timeout_seconds: int) -> int:
        """Wait for guest completion and return its exit status within the hard deadline."""
        pass

    def stop(self, handle: VMHandle) -> None:
        """Stop and reap the microVM idempotently."""
        pass

    def cleanup(self, vm_id: str) -> None:
        """Remove only cgroup, jail, socket, and disk resources bound to the VM identifier."""
        pass
