# Compatibility matrix

## Host runtime

| CLIProxyAPI | Platform | Result | Evidence |
|---|---|---|---|
| 7.2.130 | macOS arm64 | Pass | Plugin load, model registration, non-stream, stream, structured HTTP 400 |
| 7.2.145 | macOS arm64 | Pass | Plugin load, model registration, non-stream, stream, structured HTTP 400 |

The maintained source currently pins the public SDK at `v7.2.30` for broad ABI compatibility. CI recompiles and tests the plugin against SDK `v7.2.30`, `v7.2.130`, and `v7.2.145` without committing dependency churn.

## Known host lifecycle boundary

CLIProxyAPI 7.2.130 and 7.2.145 stop the HTTP server before unloading plugins during whole-process shutdown. An active long-running client stream can therefore make the host wait for its HTTP shutdown deadline before plugin shutdown runs. The plugin implements `plugin.quiesce`, closes host HTTP streams on quiesce/shutdown, and rejects new streams once quiesced, but it cannot reorder the host's whole-process shutdown sequence.

Treat this as a CLIProxyAPI host issue rather than hiding it in the provider. Plugin replacement paths on hosts that call `plugin.quiesce` can drain the plugin before unload.

## Validation contract

For every supported host update:

1. Verify the release checksum before running a downloaded host binary.
2. Load the plugin on an independent loopback port and isolated auth directory.
3. Confirm the plugin version and model catalog.
4. Run one non-stream request and one stream request.
5. Confirm an invalid upstream request preserves HTTP 400 rather than becoming a generic 500.
6. Remove the isolated credential copy and confirm the production port is unchanged.
