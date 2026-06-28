"""Records append-only security events and policy decisions with controlled retention.

Technical requirements:
- Preserve structured attributes instead of flattening them into log strings.
- Never record provider keys, capability tokens, authorization headers, or raw secrets.
- Raw source and prompts require explicit encrypted-content retention configuration.
- Events must include task, VM, operation, decision, byte counts, duration, and correlation IDs.
- Support tamper-evident export and deterministic retention deletion.
"""

from dataclasses import dataclass
from datetime import datetime
from typing import Mapping, Protocol


@dataclass(frozen=True, slots=True)
class AuditEvent:
    event_id: str
    occurred_at: datetime
    task_id: str
    vm_id: str | None
    category: str
    decision: str
    attributes: Mapping[str, str | int | bool]


class AuditSink(Protocol):
    def append(self, event: AuditEvent) -> None:
        """Persist one redacted event atomically."""
        pass

    def list_for_task(self, task_id: str) -> tuple[AuditEvent, ...]:
        """Return redacted events for one task in deterministic sequence order."""
        pass


def redact_attributes(attributes: Mapping[str, object]) -> Mapping[str, str | int | bool]:
    """Remove credentials and sensitive values before an event reaches an audit sink."""
    pass
