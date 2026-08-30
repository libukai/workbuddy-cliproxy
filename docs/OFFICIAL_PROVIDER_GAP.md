# Official provider alignment

Reference snapshot: `router-for-me/CLIProxyAPI` main at `f0de1d008fe8881dcb7431cf97b147295874c2b2`.

The built-in Kimi provider is the closest architectural reference because it combines OAuth, token refresh, OpenAI-compatible chat completions, reasoning, tools, and streaming. Codex and Claude remain secondary references for status handling and protocol translation. This external plugin must depend only on the public `sdk/pluginabi` and `sdk/pluginapi` contracts; it must not import CLIProxyAPI `internal/*` packages.

| Area | Official provider pattern | WorkBuddy fork status | Next action |
|---|---|---|---|
| Auth expiry | RFC3339 expiry metadata, five-minute refresh lead, and concurrent refresh deduplication | Aligned in v0.2.0 | Add a non-rotating refresh-token integration fixture |
| Auth identity | Unique file and auth ID per account | New logins use an opaque account hash; legacy `workbuddy.json` remains unchanged | Verify two live accounts and host round-robin routing |
| Request context | `http.NewRequestWithContext` using the host request context | Refresh and execution use callback-scoped host HTTP; plugin lifecycle still guards asynchronous pumps | Keep login cookie flow isolated and add callback cancellation tests |
| Proxy policy | `NewProxyAwareHTTPClient` with global and per-auth proxy settings | Refresh and execution use `host.http.do` / `host.http.do_stream`; login still uses an isolated local client | Track public ABI support for auth-specific host HTTP callbacks |
| HTTP errors | Typed status error preserves upstream status | Aligned for synchronous and stream bootstrap errors | Parse CodeBuddy business codes into stable error categories |
| Stream bootstrap | Open upstream and validate status before returning stream | Aligned through `host.http.do_stream` / `stream_read` / `stream_close` | Add partial-chunk SSE framing regression coverage |
| Stream cancellation | Request context, channel select, and `plugin.quiesce` before unload | Host HTTP stream lifetime is bound to callback ID; shutdown and quiesce close active host streams | CLIProxyAPI 7.2.130 lacks quiesce and can hit its API-server shutdown deadline; verify graceful drain against the latest stable host |
| Response headers | Forward upstream headers | Aligned | Add regression tests against a fake upstream |
| Usage and logging | Host request log, usage reporter, request IDs | Host HTTP callbacks capture request/response transport; no structured usage observer | Add optional usage capability without duplicating host accounting |
| Translation | Central CLIProxyAPI translators and thinking pipeline | Host translates to/from chat completions; plugin applies provider quirks | Keep provider rewrites narrow and testable |
| Tool calls | Normalizes tool links and preserves streamed fragments | Aligned for indexed fragment aggregation and named OpenAI tool choice | Add multi-turn tool-result and parallel-tool integration coverage |
| Models | Catalog and provider-specific capability metadata | Hard-coded verified list | Move to a versioned manifest with last-verified evidence |
| Configuration | Typed config fields and reconfigure handling | No plugin-specific config | Add model manifest path and default-off prompt rewriting |
| Tests | Unit, race, transport, refresh, and stream tests | Baseline model, refresh, stream-error, and lifecycle tests | Add fake upstream integration and concurrent refresh tests |

## Alignment rule

Prefer the official provider behavior whenever the public plugin ABI can express it. When the public ABI cannot preserve a required capability, document the missing contract and propose a focused CLIProxyAPI upstream change instead of importing internal code or maintaining a CLIProxyAPI core fork.
