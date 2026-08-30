# Official provider alignment

Reference snapshot: `router-for-me/CLIProxyAPI` main at `f0de1d008fe8881dcb7431cf97b147295874c2b2`.

The built-in Kimi provider is the closest architectural reference because it combines OAuth, token refresh, OpenAI-compatible chat completions, reasoning, tools, and streaming. Codex and Claude remain secondary references for status handling and protocol translation. This external plugin must depend only on the public `sdk/pluginabi` and `sdk/pluginapi` contracts; it must not import CLIProxyAPI `internal/*` packages.

| Area | Official provider pattern | WorkBuddy fork status | Next action |
|---|---|---|---|
| Auth expiry | RFC3339 expiry metadata plus five-minute refresh lead | Aligned in v0.2.0 | Add refresh concurrency deduplication |
| Auth identity | Unique file and auth ID per account | Fixed `workbuddy.json` and `workbuddy` ID | Introduce backward-compatible multi-account IDs |
| Request context | `http.NewRequestWithContext` using the host request context | Plugin lifecycle context for async streams | Move upstream I/O to host HTTP callbacks |
| Proxy policy | `NewProxyAwareHTTPClient` with global and per-auth proxy settings | Environment proxy only | Use `host.http.do` and `host.http.do_stream` |
| HTTP errors | Typed status error preserves upstream status | Aligned for synchronous and stream bootstrap errors | Parse CodeBuddy business codes into stable error categories |
| Stream bootstrap | Open upstream and validate status before returning stream | Aligned | Move stream transport to host HTTP callbacks |
| Stream cancellation | Request context and channel select | Plugin lifecycle cancel plus host stream emit failure | Bind host HTTP stream lifetime to callback ID |
| Response headers | Forward upstream headers | Aligned | Add regression tests against a fake upstream |
| Usage and logging | Host request log, usage reporter, request IDs | No structured usage observer | Use host callbacks and optional usage capability |
| Translation | Central CLIProxyAPI translators and thinking pipeline | Host translates to/from chat completions; plugin applies provider quirks | Keep provider rewrites narrow and testable |
| Tool calls | Normalizes tool links and preserves streamed fragments | Non-stream aggregation appends fragments | Merge tool calls by index and concatenate arguments |
| Models | Catalog and provider-specific capability metadata | Hard-coded verified list | Move to a versioned manifest with last-verified evidence |
| Configuration | Typed config fields and reconfigure handling | No plugin-specific config | Add model manifest path and default-off prompt rewriting |
| Tests | Unit, race, transport, refresh, and stream tests | Baseline model, refresh, stream-error, and lifecycle tests | Add fake upstream integration and concurrent refresh tests |

## Alignment rule

Prefer the official provider behavior whenever the public plugin ABI can express it. When the public ABI cannot preserve a required capability, document the missing contract and propose a focused CLIProxyAPI upstream change instead of importing internal code or maintaining a CLIProxyAPI core fork.
