# Magos: New Architecture Plan

## 1. Product direction

Magos will pivot from being its own coding agent into a local, security-focused runner for existing CLI agents such as OpenCode.

Each agent task will run inside a disposable Firecracker microVM. The security model assumes that repository content may successfully prompt-inject or otherwise compromise the agent. Magos will not claim to prevent prompt injection; it will contain the consequences by limiting the compromised agent's credentials, network access, host access, resources, and ability to apply changes.

The intended outcome is:

> Even if an agent is fully controlled by malicious repository content, it cannot access the host, obtain long-lived credentials, or communicate with unauthorized systems.

## 2. Core security properties

- The guest receives no direct access to the host filesystem, home directory, environment, credential stores, SSH agent, or cloud metadata.
- The project is copied into an ephemeral guest disk. The host repository and its `.git` directory are never mounted into the guest.
- The guest has no unrestricted internet connection or DNS access.
- Long-lived provider, registry, Git, cloud, and MCP credentials remain on the host.
- Every task receives a short-lived, revocable capability token with a strict scope and budget.
- Communication between the guest and host uses Firecracker vsock and a narrow application protocol.
- CPU, memory, process count, disk, network-equivalent traffic, output, cost, and wall-clock time are limited.
- The guest is destroyed after the task.
- Agent changes are returned as a patch for review. Magos never automatically checks out a branch or merges changes into the user's repository.

## 3. High-level architecture

```text
User / CLI
    |
    v
Magos host controller
    |-- Task manager
    |-- Firecracker + jailer manager
    |-- Source exporter and patch importer
    |-- Capability-token service
    |-- Credential and network broker
    |     |-- LLM gateway ----------> Anthropic/OpenAI/etc.
    |     |-- Package proxy --------> npm/PyPI/Go/etc.
    |     `-- Approved MCP proxy ---> Selected MCP services
    `-- Audit and policy engine
              ^
              | Firecracker vsock
              v
Disposable microVM
    |-- Read-only versioned root image
    |-- Ephemeral project disk
    |-- Guest supervisor
    |-- Agent adapter
    |     |-- OpenCode ACP
    |     `-- OpenCode run --format json
    `-- Local gateway shim
```

## 4. Task lifecycle

1. The user chooses an agent, project, prompt, model policy, resource limits, and network/package policy.
2. The host identifies the repository state and exports the selected source into a new ephemeral project image. Git credentials and the host `.git` directory are excluded.
3. The host creates a task record and a cryptographically random capability token.
4. The token is bound to the task, VM identity, expiry, permitted operations, model, token/cost budget, package policy, and MCP policy.
5. Magos starts Firecracker through the production `jailer` using an unprivileged identity, namespaces, cgroups, seccomp, and configured resource limits.
6. The guest supervisor starts the selected agent adapter and communicates with the host over vsock.
7. All LLM, package, and MCP requests pass through the host broker. The guest cannot open arbitrary network connections.
8. The host records policy decisions, destinations, operation types, sizes, timings, and budget consumption.
9. On completion or policy violation, the guest returns its status, structured events, logs, and a patch or source artifact.
10. Magos destroys the VM, revokes the capability token, and deletes ephemeral storage according to the retention policy.
11. The host validates the returned changes in a separate trusted validation environment and presents the diff to the engineer.
12. Changes are applied only after explicit approval.

## 5. Credentials and capability tokens

The guest must not receive the real LLM provider API key. Passing a key at runtime prevents it being baked into an image but does not prevent a compromised process from reading it.

Magos should issue an opaque random token rather than relying on a purely stateless JWT. The broker stores the token record, allowing immediate revocation. If JWTs are later needed for interoperability, they must have a short expiry and a `jti` checked against broker-side task and revocation state.

Example capability record:

```yaml
task_id: task_7f19
vm_id: vm_c213
expires_at: 2026-06-28T14:30:00Z
operations:
  - model.complete
  - package.fetch
model_allowlist:
  - anthropic/claude-sonnet
maximum_model_tokens: 200000
maximum_cost_gbp: 2.00
package_policy: locked-dependencies
mcp_allowlist: []
status: active
```

Secret handling by category:

- **Git:** The host obtains source before VM launch. Git and SSH credentials never enter the guest.
- **LLM providers:** The host broker stores the provider key and injects it into authorized outbound requests.
- **Package registries:** Prefer anonymous downloads. Where authentication is necessary, use a host proxy with a read-only, narrowly scoped credential.
- **MCP and other services:** Keep credentials in a host-side proxy and expose only approved servers, tools, and argument policies.
- **Cloud and deployment systems:** Do not expose these credentials to ordinary coding tasks. Deployment must be a separate workflow with separate review and policy.
- **Signing keys:** Keep outside the agent environment and use only after review.

Revoking or expiring a task token does not require rotating the underlying source-system credentials.

## 6. Controlled external communication

The preferred design gives the VM no TAP network interface. The guest communicates only through Firecracker vsock.

### LLM gateway

OpenCode or another agent is configured to use a guest-local provider endpoint or HTTP shim. The shim forwards a structured request through vsock. The host gateway:

1. authenticates the task capability;
2. validates the requested provider, model, method, size, and remaining budget;
3. adds the real provider credential;
4. sends the HTTPS request;
5. streams the response back to the guest; and
6. records usage against the task.

The selected model provider will necessarily receive code included in legitimate model requests. Magos prevents transmission to unauthorized destinations, not transmission to the explicitly selected provider.

### Package gateway

Package managers are configured to use guest-local mirrors backed by the vsock package proxy, for example:

```text
PIP_INDEX_URL=http://127.0.0.1:<port>/pypi
NPM_CONFIG_REGISTRY=http://127.0.0.1:<port>/npm
GOPROXY=http://127.0.0.1:<port>/go
```

The package proxy should enforce:

- download-only methods;
- approved upstream registries;
- package, namespace, and version policy;
- lockfile-only mode where requested;
- maximum artifact and aggregate download sizes;
- request and bandwidth limits;
- hash or signature checks where the ecosystem supports them;
- caching and optional vulnerability/malware scanning; and
- no credential forwarding into the guest.

Downloaded dependencies may still contain malicious code, but that code remains inside the disposable VM. Common toolchains should be supplied in pinned, versioned base images to reduce downloads.

### MCP gateway

MCP access must be brokered by server and tool. A task policy should list allowed MCP servers, tools, argument constraints, call counts, and response-size limits. The guest must never receive the upstream MCP credential.

### Restricted-network fallback

An early prototype may use a TAP interface with host firewall rules and a mandatory proxy. This is weaker and operationally more complex because of DNS, TLS, redirects, CDNs, and covert channels. The target architecture remains vsock-only communication with application-level gateways.

## 7. Monitoring and prompt-injection response

Prompt and tool-call monitoring is defence-in-depth, not the primary security boundary. A classifier can be evaded through encoding, indirection, split instructions, or apparently legitimate commands.

Deterministic policy controls must make unauthorized actions impossible. Monitoring can then detect and respond to suspicious behaviour:

- scan prompts, retrieved content, and tool requests for suspicious instructions;
- detect encoded or unusually large outbound payloads;
- track request volume, destinations, models, tools, and costs;
- use canary files or synthetic secrets that should never be accessed;
- flag unexpected attempts to enumerate credentials, network state, or system metadata;
- terminate and quarantine tasks that exceed policy thresholds; and
- require enhanced review for suspicious outputs.

Avoid turning the audit system into a new data leak. Raw prompts and source content should be encrypted, access-controlled, retained briefly, and optionally disabled. Prefer structured metadata, hashes, sizes, and policy decisions where raw content is unnecessary.

## 8. Agent adapter model

Magos should define a runner-neutral adapter interface rather than embedding one agent's behaviour into the controller.

An adapter should declare:

- executable and pinned version;
- supported protocol;
- command-line arguments;
- required environment variables;
- provider endpoint configuration;
- interactive or one-shot capability;
- working-directory behaviour;
- event and output format;
- graceful cancellation procedure; and
- required broker capabilities.

OpenCode is the first target:

- use `opencode acp` for interactive sessions through structured JSON-RPC over stdio; and
- use `opencode run --format json` for simpler one-shot tasks.

PTY passthrough can be added later for agents that expose only an interactive terminal, but structured protocols are preferable for auditing, cancellation, and reliable result handling.

## 9. Source and patch handling

The current host worktree approach must not be used as a security boundary.

For each task:

- export the desired commit and explicitly selected uncommitted files into an isolated image;
- exclude `.git`, `.magos`, credentials, editor state, sockets, and unrelated files;
- record the exact input manifest and hashes;
- let the agent modify only its ephemeral copy;
- produce a unified patch and an output-file manifest;
- reject paths outside the project, absolute paths, special files, links escaping the project, and oversized outputs;
- validate the patch applies cleanly to the recorded base;
- run configured tests and static checks outside the agent VM; and
- show the engineer the diff before applying it.

The engineer remains responsible for reviewing whether an edit is correct or malicious. Magos is responsible for never silently applying it.

## 10. Resource and runtime controls

Every task must define hard limits for:

- vCPU count and CPU time;
- memory;
- process count;
- writable disk and file count;
- LLM tokens and monetary spend;
- package download size;
- broker request rate and response size;
- stdout, stderr, structured-event, and audit-log size;
- idle timeout and total wall-clock time; and
- maximum patch size and changed-file count.

Cancellation should terminate the guest, revoke the token, close broker channels, and clean up storage. Do not rely only on killing a shell process inside the guest.

## 11. Host and image hardening

- Run on a dedicated, supported Linux host with KVM. Firecracker does not run directly on Windows.
- Use the production Firecracker binary and matching `jailer`.
- Keep the host kernel, guest kernel, microcode, Firecracker, and base images patched.
- Run each VM under a unique unprivileged UID/GID and cgroup.
- Use minimal guest kernels and root filesystems.
- Make the base root filesystem read-only and use a disposable writable data disk.
- Disable unnecessary devices and metadata services.
- Pin image contents and verify hashes/signatures before launch.
- Maintain an SBOM and reproducible image-build process.
- Never reuse a writable disk between untrusted tasks.
- Treat snapshots as sensitive because they may contain task data or temporary capabilities.

## 12. Migration from the current code

The current code is useful as a UI and workflow prototype, but its execution and Git paths should be replaced rather than incrementally described as isolated.

Remove or redesign:

- direct host `bash -c` execution;
- inherited host environment variables;
- claims that a worktree is an isolated VM;
- automatic checkout and merge into `main`;
- force-deletion of worktrees containing uncommitted changes;
- validation that falls through to merge on setup or execution errors;
- embedded Anthropic-specific agent logic in the main workflow; and
- unbounded command output and background contexts without cancellation.

Potentially retain and evolve:

- Bubble Tea UI concepts;
- SQLite task and audit storage, after preserving structured fields correctly;
- validation result data structures;
- project discovery; and
- session display and event streaming.

## 13. Delivery phases

### Phase 0: Specification and threat model

- Define trusted and untrusted components.
- Document attacker capabilities and protected assets.
- Define task, capability, broker, adapter, event, and patch schemas.
- Decide supported Linux distributions and CPU architectures.
- Create security acceptance tests before implementation.

### Phase 1: Safe local runner abstraction

- Remove the embedded LLM-agent workflow.
- Add an agent adapter interface and OpenCode one-shot adapter.
- Add task IDs, cancellation, bounded streaming output, and explicit patch review.
- Keep execution clearly labelled unsafe until the microVM backend exists.

### Phase 2: Minimal Firecracker execution

- Build reproducible guest kernel and root image.
- Start Firecracker through `jailer`.
- Copy source into an ephemeral project disk.
- Run a fixed test command with no network or host secrets.
- Return logs, status, and a patch over vsock.
- Destroy all task resources reliably.

### Phase 3: Capability and LLM broker

- Implement opaque task tokens, expiry, revocation, budgets, and audit records.
- Add the guest-local gateway shim and host vsock broker.
- Add one provider integration while keeping the real provider key on the host.
- Run OpenCode through the broker without general guest networking.

### Phase 4: Package and MCP gateways

- Add read-only package proxies, caching, limits, and lockfile policy.
- Add MCP server/tool allowlists and host-side credentials.
- Add base-image profiles for common language ecosystems.

### Phase 5: Monitoring and hardening

- Add behavioural alerts, canaries, quarantine, and policy reporting.
- Add fuzzing and adversarial prompt-injection test repositories.
- Test VM escape assumptions, broker authorization, token replay, covert channels, archive/path traversal, symlink handling, and cleanup failures.
- Obtain an independent security review before describing the system as hardened.

### Phase 6: Multiple agents and production UX

- Add further CLI-agent adapters.
- Add ACP interactive sessions and optional PTY support.
- Add signed image/update distribution.
- Add user-facing policy profiles such as `offline`, `model-only`, `packages`, and `approved-mcp`.

## 14. Initial acceptance criteria

The first security-relevant release is complete only when all of the following are demonstrated:

- A malicious guest command cannot read any host file or host environment variable.
- The guest does not contain a long-lived provider, Git, registry, MCP, cloud, or signing credential.
- The guest cannot reach the internet, LAN, host services, DNS, or cloud metadata except through explicitly exposed broker operations.
- A task token cannot be used by another VM, after expiry, after revocation, or beyond its budget and operation scope.
- The LLM gateway cannot be redirected to an attacker-selected endpoint.
- The package gateway cannot publish, upload arbitrary content, or access an unapproved registry.
- Resource exhaustion terminates only the task and does not destabilize the host controller.
- VM termination revokes capabilities and removes ephemeral resources.
- Returned patches cannot escape the project root or modify files without review.
- Validation errors, missing validators, broker failures, and cleanup failures are fail-closed and visibly reported.
- No code is automatically merged or applied to the user's checkout.
- Adversarial prompt-injection fixtures are included in automated end-to-end tests.

## 15. Product wording

Recommended description:

> Magos runs CLI coding agents inside disposable local Firecracker microVMs, with brokered credentials, controlled external access, resource limits, and reviewable patch output.

Avoid claiming that Magos eliminates prompt injection. The defensible claim is that it is designed to contain a compromised agent and minimize the authority available to it.
