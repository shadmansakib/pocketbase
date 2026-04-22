# PocketBase Development Reference

This file is the working reference for humans and AI agents maintaining this fork of PocketBase.

It is intentionally project-specific. Prefer it over generic Go advice whenever the two differ.

## 1. Project Snapshot

- Module: `github.com/pocketbase/pocketbase`
- Primary language: Go `1.25`
- Frontend toolchain: Node `24+`, Vite, `dprint`
- Embedded database: SQLite via `modernc.org/sqlite`
- CLI framework: `spf13/cobra`
- DB/query layer: `github.com/pocketbase/dbx`
- Validation: `go-ozzo/ozzo-validation/v4`
- JS runtime/plugin system: `goja`, `goja_nodejs`, `plugins/jsvm`
- Embedded admin UI: `ui/dist` is embedded into Go via `ui/embed.go`

PocketBase is library-first. The repo is not just a CLI app; the CLI is a thin wrapper around a reusable `core.App`.

## 2. Architecture At A Glance

```text
custom main / examples/base/main.go
               |
               v
         pocketbase.New()
               |
               v
        PocketBase / RootCmd
               |
      +--------+--------+
      |                 |
      v                 v
   cmd/*            plugins/*
      |                 |
      +--------+--------+
               |
               v
          apis.Serve()
               |
               v
      tools/router + middlewares
               |
               v
        core.RequestEvent handlers
               |
      +--------+--------+------------------+
      |                 |                  |
      v                 v                  v
    forms/*          core/*             tools/*
      |                 |                  |
      +--------+--------+------------------+
               |
               v
      dbx + SQLite + filesystem + mailer
```

The most important design choice in this codebase is that behavior is routed through `core.App`, hooks, forms, and model abstractions rather than through ad hoc SQL or one-off HTTP logic.

## 3. Repo Structure

| Path | Purpose | Notes |
| --- | --- | --- |
| `pocketbase.go` | Public launcher entrypoint | Wires `cobra`, bootstrap, shutdown, default commands |
| `examples/base` | Minimal standalone app | Mirrors released binary shape |
| `cmd` | Built-in CLI commands | `serve`, `superuser`, etc. |
| `core` | Domain backbone | app interface, models, fields, events, DB, migrations runner |
| `apis` | HTTP layer | router wiring, middlewares, REST-ish handlers |
| `forms` | Request-to-model write helpers | central place for request-driven validation/load/save logic |
| `migrations` | built-in and user-facing migration helpers | system schema bootstrap and registration helpers |
| `plugins` | optional capabilities | JS VM hooks, migrate command, GitHub update |
| `tools` | reusable infrastructure packages | router, hook, search, auth providers, filesystem, mailer, logger, etc. |
| `tests` | shared test helpers and fixtures | `tests.NewTestApp()` is the standard test entrypoint |
| `ui/src` | frontend source of truth | current source is JS + Shablon-style templating + Vite |
| `ui/dist` | built frontend bundle | embedded in Go; do not treat as primary source |

## 4. Core Runtime Flows

### Startup flow

```text
main()
  -> pocketbase.New()
  -> core.NewBaseApp(...)
  -> app.Start()
  -> cmd.NewServeCommand(...)
  -> apis.Serve(...)
  -> apis.NewRouter(...)
  -> app.OnServe() hooks
  -> http server listen/serve
```

### Write path

```text
HTTP request
  -> apis handler
  -> request parsing / auth / rules
  -> forms.RecordUpsert or app-level domain logic
  -> app.Save(...) / app.Delete(...)
  -> OnModel* hooks
  -> OnRecord* proxy hooks
  -> DB write
  -> OnModelAfter* / OnRecordAfter*
     (after commit when inside tx)
```

This write pipeline is a contract. Bypassing it is how subtle regressions enter PocketBase.

## 5. Coding Style And Conventions

### 5.1 General Go style

- Follow `gofmt` and `goimports`. The repo already enforces both in `golangci.yml`.
- Prefer ASCII in new code/comments unless the file already has a clear reason to use Unicode. The lint config enables `asciicheck`.
- Keep exported identifiers documented.
- Prefer small, cohesive packages over broad utility dumping.
- Prefer standard library and existing repo utilities before adding new dependencies.
- Keep comments high-signal. This repo uses comments to explain contracts, lifecycle ordering, and edge cases, not trivial code.
- Name things directly. Package and file names are plain and descriptive: `record_query.go`, `middlewares_rate_limit.go`, `field_text.go`.
- Avoid naked returns and keep `Printf`-style helper naming honest; the lint configuration is strict about both.

### 5.2 Dependency direction

Default direction should remain:

```text
examples/cmd/plugins/apis/forms
              |
              v
             core
              |
              v
             tools
```

Rules:

- `core` is the center of gravity.
- `apis` should depend on `core`, `forms`, and `tools`, not the reverse.
- `forms` should orchestrate model loading/validation, not become a second API layer.
- `tools` should stay generic and reusable.
- Avoid new cross-package shortcuts that make `core` depend on `apis` or on frontend concerns.

### 5.3 Interface-first boundaries

- Accept `core.App` at package boundaries whenever practical.
- Reach for concrete `*core.BaseApp` only when behavior truly requires implementation details.
- The large `core.App` interface exists on purpose to make extension and testing easier.

### 5.4 Hooks are first-class

The hook system is not a side feature; it is a primary extension mechanism.

Rules:

- A hook handler must call `e.Next()` unless it is intentionally terminating the chain.
- Lower `Priority` values execute earlier.
- Prefer tagged hooks or record-specific hooks when scoping matters.
- Use `OnRecord*` when the logic is record-specific; use `OnModel*` only when model-level generality is required.
- Remember that `OnModelAfter*Success` and `OnRecordAfter*Success` are delayed until transaction commit.
- Do not use hooks for behavior that should have been a plain function call in the same package; reserve hooks for extension seams, lifecycle interception, and cross-cutting behavior.

### 5.5 Persistence rules

- Prefer `app.Save(...)` and `app.Delete(...)` for model persistence.
- Use `SaveNoValidate` only when there is a clear reason and the caller owns the invariants.
- Prefer forms like `forms.RecordUpsert` for request-driven record writes.
- Use raw SQL only when the model/collection APIs cannot express the needed operation clearly or efficiently.
- If you drop to raw SQL, preserve error normalization, transaction boundaries, and collection/field invariants.

SQLite-specific rules:

- Default read path: `ConcurrentDB()`
- Default write path: `NonconcurrentDB()`
- For multi-step writes, use `RunInTransaction(...)`
- Inside a transaction, always use the callback `txApp`, not the outer `app`

The codebase explicitly separates concurrent and nonconcurrent builders to reduce `SQLITE_BUSY` behavior. Respect that split.

### 5.6 Transactions

- Use `app.RunInTransaction(...)` or `app.AuxRunInTransaction(...)` for grouped writes.
- Nested transactions are allowed only if all nested work uses the provided `txApp`.
- If you need post-commit work, rely on the existing after-transaction hook behavior or `txApp.TxInfo().OnComplete(...)`.
- Do not mix outer app builders and tx builders in the same logical write sequence.

### 5.7 API layer style

- Route registration stays centralized through `apis.NewRouter()` and binder helpers such as `bindRecordCrudApi`.
- API handlers operate on `*core.RequestEvent`.
- Use the event error helpers such as `e.BadRequestError`, `e.ForbiddenError`, `e.NotFoundError`, `e.InternalServerError`.
- Perform auth/rule checks early.
- Enrich records right before the response, not prematurely.
- Keep middleware concerns in `apis/middlewares*.go`, not duplicated inside handlers.

### 5.8 Model and field style

- Field types register themselves via `init()` and the `Fields[...]` registry.
- Model structs embed `core.BaseModel` where appropriate.
- Validation is layered: prepare value, validate value, model hooks, and post-commit behavior.
- Hidden/system/auth semantics are sensitive. Treat changes to these areas as security-relevant.
- When adding a field type, inspect existing field files and their paired tests before designing the new one.

### 5.9 Error handling

- Return errors; do not hide them.
- Add context when crossing boundaries or when raw SQL/filesystem/network operations are involved.
- Normalize domain-specific errors where the codebase already does so.
- Avoid panic except for truly impossible initialization failures or explicit `Must*` behavior.

### 5.10 Testing style

- Use the standard `testing` package.
- Prefer table-driven tests for validation, parsing, and rule-heavy behavior.
- Use `tests.NewTestApp()` for integration-style app tests.
- Keep tests close to the package they cover.
- Cover hook ordering, access rules, hidden fields, transaction behavior, and error paths for risky changes.
- This repo does not use testify/assert-style helpers as a default pattern; stay consistent with existing tests.

## 6. Tooling And Quality Gates

Primary commands:

- `go test ./...`
- `make test`
- `golangci-lint run -c ./golangci.yml ./...`
- `make lint`

Frontend commands:

- `cd ui && npm install`
- `cd ui && npm run dev`
- `cd ui && npm run build`

Notes:

- UI changes are not complete until `ui/dist` is rebuilt.
- `ui/package.json` runs `dprint fmt` as part of the build.

## 7. Frontend Rules

Important nuance: current source files show a JS + Shablon-style frontend bootstrapped by Vite (`ui/index.html`, `ui/src/main.js`), even though `CONTRIBUTING.md` still mentions Svelte. Treat the current source tree as the source of truth.

Rules:

- Edit `ui/src`, `ui/public`, and related assets; do not hand-edit `ui/dist` unless performing an emergency generated-file patch and documenting why.
- Remember that `ui/dist` is embedded by Go. If the bundle is stale, the server will serve stale UI.
- Keep backend API contract changes synchronized with UI expectations.
- Favor existing frontend patterns in `ui/src` instead of introducing a parallel component architecture.

## 8. Migrations And Schema Changes

- Built-in/system schema is managed in `migrations/` plus `core.SystemMigrations`.
- App/user migrations register through the migration helpers and related plugins.
- Prefer collection/field abstractions for schema creation when possible.
- When raw SQL is necessary in migrations, provide both `up` and `down` logic and keep names stable.
- Be extra careful with collection names, indexes, auth tables, and system collections. These ripple into rules, files, and UI behavior.

## 9. Common Change Playbooks

### 9.1 Adding or changing an API endpoint

1. Add or adjust the route binder in `apis`.
2. Keep auth, rate limit, and collection-rule checks near the top of the handler.
3. Use `forms` or `core.App` persistence APIs instead of ad hoc SQL.
4. Add focused API tests in `apis/*_test.go`.
5. If the UI depends on the endpoint, update `ui/src` and rebuild `ui/dist`.

### 9.2 Adding or changing a record write flow

1. Start from `forms.RecordUpsert` and related handlers.
2. Verify hidden/system/auth-field behavior.
3. Check hook side effects and transaction timing.
4. Add tests for create, update, forbidden access, validation failure, and transactional rollback where relevant.

### 9.3 Adding a field type

1. Register the field via `init()`.
2. Implement interfaces consistently with existing fields.
3. Add value preparation, validation, schema column mapping, and any record interceptors.
4. Add mirrored tests covering happy path and edge cases.
5. Update UI field initialization/settings/view code if the field is user-facing.

### 9.4 Adding a migration

1. Decide whether it is a system migration or an app migration.
2. Keep migration steps idempotent where practical.
3. Preserve rollback logic when possible.
4. Test against a realistic `tests` app setup if behavior is nontrivial.

### 9.5 Changing the admin UI

1. Edit `ui/src`.
2. Verify the route/store pattern already used there.
3. Run `npm run build` in `ui`.
4. Confirm embedded assets reflect the source change.

## 10. Strict AI Rules

These rules are mandatory for AI-assisted work on this repo.

1. Read the target package and nearby tests before editing.
2. Preserve the existing architecture; do not introduce shortcuts that bypass `core.App`, hooks, or forms without a strong reason.
3. Do not replace model/form logic with raw SQL just because SQL is shorter.
4. Do not skip `e.Next()` in hook handlers unless intentional and documented by the code path.
5. Treat hidden fields, auth logic, access rules, token flows, and file handling as security-sensitive.
6. Use `RunInTransaction` and the provided `txApp` for grouped writes.
7. Do not mix outer app state with transactional app state.
8. Do not hand-edit `ui/dist` as the primary source of truth.
9. Add or update tests for any behavior change, especially around hooks, permissions, migrations, and persistence.
10. Keep comments and docs aligned with the code when changing lifecycle behavior.
11. Prefer minimal, local changes that fit existing patterns over broad refactors.
12. Before proposing new abstractions, verify the repo does not already have an equivalent helper in `core`, `forms`, or `tools`.
13. If contributing upstream is the goal, note that current repo docs explicitly say PRs are temporarily restricted and LLM contributions are not welcome; coordinate through the fork/process being used here instead of assuming direct upstream submission.

## 11. High-Risk Areas

Changes here require extra scrutiny:

- `core` hook ordering and lifecycle semantics
- auth flows and token issuance
- hidden/system field behavior
- collection import/export and schema sync
- filesystem and file field handling
- migrations and bootstrap logic
- realtime subscriptions
- rate limiting and security headers
- transaction boundaries and after-commit behavior

## 12. Definition Of Done For Maintenance Work

A change is usually not done until all of the following are true:

- The implementation follows existing package boundaries.
- Relevant tests were added or updated.
- Lint/format expectations are still satisfied.
- Hook and transaction behavior were considered explicitly.
- UI assets were rebuilt if `ui/src` changed.
- Docs were updated if public behavior, lifecycle, or workflow changed.

## 13. Practical Defaults

When in doubt:

- prefer `core.App` over concrete app types
- prefer forms over ad hoc request mutation
- prefer `app.Save` over `SaveNoValidate`
- prefer transactions for multi-step writes
- prefer package-local consistency over cleverness
- prefer extending existing hooks and helpers over inventing new layers
