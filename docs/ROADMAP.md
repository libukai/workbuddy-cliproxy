# Maintenance roadmap

## v0.2.x: production baseline

- Keep verified CodeBuddy model IDs current.
- Surface credential expiry to CLIProxyAPI and refresh before expiration.
- Cancel active streams during plugin shutdown and report upstream read failures.
- Add CI for tests, vetting, and native shared-library builds.
- Preserve reproducible build metadata and checksums for every release.

## v0.3.x: maintainability

- Move model metadata and reasoning policy into a versioned manifest.
- Split auth, transport, model, transformation, and streaming code into focused files.
- Merge streamed tool-call fragments correctly for non-streaming clients.
- Preserve structured upstream error categories such as auth, quota, model, review, and retryable server errors.
- Make prompt rewriting an explicit configuration option that is disabled by default.

## v0.4.x: multi-account operations

- Use stable per-account auth IDs and credential file names.
- Avoid shared cookies across accounts.
- Add per-account health, quota, and cooldown signals without logging tokens.
- Test round-robin routing, token rotation, concurrent streaming, and forced shutdown.

## Continuous compatibility checks

- Compare `origin/main` with `upstream/main`.
- Check the latest CLIProxyAPI release and plugin ABI compatibility.
- Compare the CodeBuddy UI model list with the plugin manifest.
- Run `/v1/models` plus one minimal real request for each newly added or renamed model.
- Treat authentication, model registration, and real inference as separate validation states.
