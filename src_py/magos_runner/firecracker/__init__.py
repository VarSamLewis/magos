"""Firecracker runtime and immutable image-management boundaries.

Technical requirements:
- Production execution must use a matching Firecracker and jailer release.
- This package must not depend on agent-specific implementations.
- Resource creation, inspection, stopping, and cleanup must be task-scoped and idempotent.
"""
