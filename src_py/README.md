# Magos Python Runner

This directory contains the Python implementation scaffold for the architecture in [`../new_plan.md`](../new_plan.md).

The package is deliberately split along security boundaries:

- `controller.py` owns the host-side task lifecycle.
- `firecracker/` owns jailed microVM creation and destruction.
- `broker/` owns temporary capabilities and approved external communication.
- `adapters/` translates agent-specific protocols into Magos events.
- `workspace/` exports source and safely imports reviewed patches.
- `guest/` contains the minimal process that runs inside the microVM.
- `protocol.py` defines the only messages allowed across vsock.

The current files define contracts and technical requirements. Operational Firecracker, networking, and credential code remains intentionally unimplemented until the threat model and protocol tests are in place.

## Development

```powershell
cd src_py
python -m magos_runner --help
python -m unittest discover -s tests
```
