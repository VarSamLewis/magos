"""Defines the bounded, versioned protocol transported between host and guest over vsock.

Technical requirements:
- Use length-prefixed messages with a small hard maximum; never read until EOF.
- Authenticate every guest-initiated request with its task capability.
- Include protocol version, task ID, VM ID, request ID, and monotonic sequence number.
- Reject duplicate request IDs, sequence regressions, unknown message types, and extra fields.
- Stream large logs and patches as hashed chunks with explicit totals and size limits.
- The protocol must expose operations, never arbitrary host paths, URLs, sockets, or commands.
"""

from dataclasses import dataclass
from enum import StrEnum
from typing import Any, BinaryIO, Mapping


PROTOCOL_VERSION = 1
MAX_MESSAGE_BYTES = 1_048_576


class Operation(StrEnum):
    MODEL_COMPLETE = "model.complete"
    PACKAGE_FETCH = "package.fetch"
    MCP_CALL = "mcp.call"
    EVENT_EMIT = "event.emit"
    PATCH_SUBMIT = "patch.submit"
    TASK_COMPLETE = "task.complete"


@dataclass(frozen=True, slots=True)
class Envelope:
    version: int
    task_id: str
    vm_id: str
    request_id: str
    sequence: int
    operation: Operation
    capability: str
    payload: Mapping[str, Any]


@dataclass(frozen=True, slots=True)
class ResponseEnvelope:
    request_id: str
    accepted: bool
    payload: Mapping[str, Any]
    error_code: str | None = None


def encode_envelope(envelope: Envelope) -> bytes:
    """Serialize one request into the canonical bounded wire representation."""
    pass


def decode_envelope(data: bytes) -> Envelope:
    """Parse and strictly validate one untrusted request envelope."""
    pass


def write_frame(stream: BinaryIO, payload: bytes) -> None:
    """Write one length-prefixed frame while enforcing the protocol size limit."""
    pass


def read_frame(stream: BinaryIO) -> bytes:
    """Read exactly one bounded length-prefixed frame without waiting for EOF."""
    pass
