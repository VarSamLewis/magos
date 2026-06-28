"""Defines stable error categories crossing Magos subsystem boundaries.

Technical requirements:
- Errors exposed to users must not include credentials or raw sensitive payloads.
- Security-policy failures must be distinguishable from infrastructure failures.
- Cleanup failures must never replace the original task failure; callers should retain both.
"""


class MagosError(Exception):
    """Base class for expected Magos failures."""


class ConfigurationError(MagosError):
    """Raised before task launch when configuration is invalid or unsafe."""


class PolicyDenied(MagosError):
    """Raised when a requested capability is outside the task policy."""


class ProtocolError(MagosError):
    """Raised for malformed, oversized, replayed, or unexpected guest messages."""


class RuntimeFailure(MagosError):
    """Raised when the microVM cannot be started, supervised, or destroyed safely."""
