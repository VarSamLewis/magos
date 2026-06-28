"""Magos host and guest components for running CLI agents in disposable microVMs.

Technical requirements:
- Keep the public API small and versioned.
- Do not perform I/O, read credentials, or initialize global services on import.
- Treat all guest messages, agent output, repository content, and returned patches as untrusted.
"""

__version__ = "0.1.0"
