# PocketBase Development Guide

This document is for human developers working in this repository.

For project conventions and AI-specific operating rules, see [AGENTS.md](./AGENTS.md).

## 1. Prerequisites

- Go `1.25+`
- Node `20.19+` or `22.12+`
- `golangci-lint` if you want to run the same lint checks used by the repo

## 1.1 Install Dependencies

From the repo root:

```sh
go mod download
```

For the frontend dependencies:

```sh
cd ui
npm install
```

Notes:

- `go mod download` fetches the Go module dependencies declared in `go.mod`.
- `npm install` installs the UI toolchain and package dependencies used by Vite and the frontend build scripts.
- If you are starting from a fresh clone, run both commands before trying to build or run the app.
- Use `go mod tidy` only when you are intentionally updating module dependencies or cleaning up `go.mod` / `go.sum`.
- Do not use `go mod tidy` as the default setup command, because it may rewrite module files and remove dependencies that are not currently referenced by your local workspace.
- If your Node version is older than the prerequisite above, upgrade Node before running `npm install` or `npm run dev`.
- If `npm run build` or `npm run dev` reports a Rolldown/Vite native binding error, the local Node version is too old or the optional frontend dependency install is incomplete. Upgrade Node, then run `npm install` again in `ui`.
- JS hooks loaded by the example app must match the default loader pattern (`*.pb.js` or `*.pb.ts`). A file named only `*.js` in `pb_hooks` will not be loaded unless you override `--hooksDir` and `--hooksFilesPattern`.

## 1.2 Regenerate JSVM Typings

The JSVM hook and API typings are generated from Go definitions and checked into:

- `plugins/jsvm/internal/types/generated/types.d.ts`

Regenerate them with:

```sh
go run ./plugins/jsvm/internal/types
```

Run this command when you change the JSVM-exposed surface, for example:

- adding or removing a global hook helper
- changing a Go type that should appear in `.pb.ts` or `.pb.js` IntelliSense
- changing a field name, required/optional status, or method signature that is exposed to hooks
- updating `collectionActionAdd(...)`, `collectionActionRemove(...)`, or other exported hook bindings

Do not run it for ordinary application changes that do not affect the exported JSVM API, such as:

- UI-only changes
- internal backend refactors that do not change the public hook surface
- bug fixes that stay entirely inside Go internals

Notes:

- The generated file is intentionally committed so contributors do not need to regenerate it on every checkout.
- If you do regenerate it, review the diff carefully because the file is large and small API changes can produce widespread output changes.
- If the command fails because the local Go cache is not writable, set `GOCACHE=/tmp/pocketbase-gocache` and rerun it.

## 2. Quick Start

### Backend only

Use this when you want to run PocketBase with the embedded admin UI bundle from `ui/dist`.

```sh
cd examples/base
go run main.go serve --dev
```

Open: `http://127.0.0.1:8090`

Notes:

- `--dev` enables dev mode logging and SQL output.
- Run this command from `examples/base`, not from the repo root.
- When started with `go run`, PocketBase uses the current working directory for defaults, so running from `examples/base` keeps `pb_data`, `pb_public`, `pb_hooks`, and `pb_migrations` in the expected place.
- The UI served at `:8090` is the built bundle from `ui/dist`. It does not hot reload.
- Changes in `ui/src` will not appear here until you rebuild the frontend bundle with `cd ui && npm run build`.

### Full-stack development with frontend hot reload

Use two terminals.

Terminal 1:

```sh
cd examples/base
go run main.go serve --dev
```

Terminal 2:

```sh
cd ui
npm install
npm run dev
```

Open: `http://localhost:5173`

Recommended frontend env file:

Create `ui/.env.development.local` with:

```dotenv
PB_BACKEND_URL=http://127.0.0.1:8090
```

This keeps the Vite dev server pointed at the local PocketBase backend.

Notes:

- This mode requires Node `20.19+` or `22.12+`.
- If `npm run dev` fails with a Rolldown or Vite native binding error, your Node runtime is too old for the current UI toolchain.
- This mode uses the live `ui/src` files and does not depend on rebuilding `ui/dist`.

## 3. Hot Reload Rules

- Frontend (`ui/src`): yes, via Vite HMR when running `npm run dev`
- Embedded UI (`ui/dist` served by the Go app): no
- Go backend code: no built-in hot reload; restart the Go process after changes
- JS app hooks (`pb_hooks`): yes, auto app restart is enabled by default in `examples/base/main.go` with `--hooksWatch=true`
- JS migrations (`pb_migrations`): loaded on startup; rerun/restart as needed depending on the change

Practical rule:

- If you change Go code, restart the backend.
- If you change UI code in `ui/src`, use the Vite dev server for feedback.
- If you want the backend-served embedded UI to include your frontend changes, run `npm run build` in `ui`.

## 4. Important Entrypoints

### Backend

| File | Role |
| --- | --- |
| `examples/base/main.go` | Main development entrypoint and release-like example app |
| `pocketbase.go` | Core PocketBase launcher, bootstrap, CLI wiring |
| `cmd/serve.go` | `serve` command defaults and flags |
| `apis/serve.go` | HTTP server startup and shutdown flow |
| `apis/base.go` | Router creation, default middlewares, API route registration |

### Frontend

| File | Role |
| --- | --- |
| `ui/index.html` | HTML shell for the Vite app |
| `ui/src/main.js` | Frontend bootstrap entrypoint |
| `ui/src/router.js` | Client-side route registration |
| `ui/src/pb.js` | PocketBase JS SDK client setup and auth wiring |
| `ui/embed.go` | Embeds `ui/dist` into the Go binary |
| `ui/embed_no_ui.go` | Disables embedding when built with `-tags no_ui` |

## 5. Runtime Flow At A Glance

### Backend

```text
examples/base/main.go
  -> pocketbase.New()
  -> app.Start()
  -> cmd.NewServeCommand(...)
  -> apis.Serve(...)
  -> apis.NewRouter(...)
  -> /api routes + embedded UI/static handlers
```

### Frontend

```text
ui/index.html
  -> ui/src/main.js
  -> ui/src/router.js
  -> ui/src/pb.js
  -> requests to PocketBase backend at :8090
```

## 6. Common Local Directories

When you run from `examples/base`, these are the important local directories:

| Path | Purpose |
| --- | --- |
| `examples/base/pb_data` | SQLite DBs, app data, runtime files |
| `examples/base/pb_public` | Static files served by the catch-all route |
| `examples/base/pb_hooks` | JS app hooks loaded by `plugins/jsvm` |
| `examples/base/pb_migrations` | JS migrations loaded by `plugins/jsvm` and `migratecmd` |

Default behavior comes from:

- `pocketbase.New()` using the current working directory during `go run`
- `examples/base/main.go` registering JS hooks, migrations, and public directory support

## 7. Useful Backend Flags

Common flags while running the example app:

```sh
cd examples/base
go run main.go serve --dev --dir ./pb_data
```

Useful flags:

- `--dev`: print logs and SQL statements
- `--dir`: set the app data directory
- `--http`: override the HTTP bind address
- `--https`: enable HTTPS listener
- `--origins`: set CORS origins
- `--hooksDir`: override JS hooks directory
- `--hooksWatch`: auto restart on `pb_hooks` changes
- `--migrationsDir`: override user migration directory
- `--publicDir`: override static public directory
- `--indexFallback`: enable SPA-style `index.html` fallback for public files

## 8. Frontend Development Notes

- The source of truth is `ui/src`, not `ui/dist`.
- The current UI source is plain JS modules plus Shablon-style templating, bundled with Vite.
- The alias `@` points to `ui/src`.
- Environment variables for the UI must start with `PB` because `ui/vite.config.js` sets `envPrefix: "PB"`.
- `ui/dist` is generated output and is embedded into the Go binary.
- `npm run build` runs `dprint fmt` first and then `vite build`.
- `npm run build` requires the Node version listed above. On older Node releases it may fail before Vite starts with a native binding error from Rolldown.

## 9. Testing

From the repo root:

```sh
go test ./...
```

Or via the Makefile:

```sh
make test
```

Coverage report:

```sh
make test-report
```

Testing notes:

- The repo uses the standard Go `testing` package.
- Many tests use `tests.NewTestApp()` to clone `tests/data` into a temp app data dir.
- Integration-style tests often live alongside the package they cover.

## 10. Linting And Formatting

From the repo root:

```sh
make lint
```

Equivalent direct command:

```sh
golangci-lint run -c ./golangci.yml ./...
```

Frontend build formatting is handled by:

```sh
cd ui
npm run build
```

That command runs `dprint fmt` before the Vite build.

## 11. Building

### Build the example backend executable

```sh
cd examples/base
go build
```

This produces a local `base` executable that behaves like the repo's example standalone app.

Notes:

- By default, `go build` writes the binary into the current directory.
- On macOS and Linux, the default output file name is `base`.
- On Windows, the default output file name is `base.exe`.
- If you want a custom output name or location, pass `-o`.
- For release artifacts in this repo, prefer the gitignored `/.builds/` directory.
- After a successful build, verify the output with `file`. On macOS you should see something like `Mach-O 64-bit executable`, not `current ar archive`.
- If the target file already exists and `file` reports an archive or something unexpected, remove the stale file and rebuild.

### macOS executable build

Use this command from `examples/base` when you want a production-style executable on macOS:

```sh
mkdir -p ../../.builds
go build -trimpath -ldflags="-s -w -buildid=" -o ../../.builds/pocketbase
```

This writes the executable to `../../.builds/pocketbase` relative to `examples/base`.
If you need to troubleshoot build-cache permissions, prefix the command with `GOCACHE=/tmp/pocketbase-gocache`.
After building, verify it with:

```sh
file ../../.builds/pocketbase
```

Example:

```sh
cd examples/base
go build -o ../../.builds/pocketbase
```

That creates `.builds/pocketbase` at the repo root on macOS and Linux, or `.builds/pocketbase.exe` on Windows.

### Cross-compile a static binary

```sh
cd examples/base
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build
```

### Build frontend bundle for embedding

```sh
cd ui
npm run build
```

Do this whenever you want the Go app to serve the updated embedded admin UI.

### Build without bundling the UI

```sh
cd examples/base
go build -tags no_ui
```

With the `no_ui` build tag, `ui/embed_no_ui.go` is used and the admin UI is not embedded in the binary.

### Production build guidance

For a production release, prefer a trimmed binary with no local source paths in the debug metadata:

```sh
cd examples/base
go build -trimpath -ldflags="-s -w -buildid="
```

That writes the binary into `examples/base` using the default output name unless you pass `-o`.
For production artifacts, prefer writing into `.builds/` instead:

```sh
cd examples/base
mkdir -p ../../.builds
go build -trimpath -ldflags="-s -w -buildid=" -o ../../.builds/pocketbase
```

If you want to ship the admin UI inside the binary, build the frontend bundle first:

```sh
cd ui
npm run build
cd ../examples/base
go build -trimpath -ldflags="-s -w -buildid="
```

You can also combine the production flags with a custom output name:

```sh
cd examples/base
mkdir -p ../../.builds
go build -trimpath -ldflags="-s -w -buildid=" -o ../../.builds/pocketbase
```

If you want a backend-only binary with no embedded admin UI, build with the `no_ui` tag:

```sh
cd examples/base
go build -trimpath -ldflags="-s -w -buildid=" -tags no_ui
```

Notes:

- `-trimpath` removes local filesystem paths from the compiled package metadata and panic/debug output.
- `-ldflags="-s -w -buildid="` strips symbol and DWARF tables and clears the build id string.
- These flags help reduce the amount of machine-specific information visible in a decompiled or inspected binary.
- They do not remove strings that you hardcode into Go code, JS hooks, templates, HTML, or the embedded frontend bundle.
- For that reason, never put secrets, private URLs, API keys, passwords, or absolute development paths directly into source files that are compiled or embedded.
- Prefer runtime configuration through environment variables, app settings, or external secret managers.
- If you ship the embedded UI, inspect `ui/dist` before release and make sure it does not contain development-only values from your local `.env` files.
- Use the `no_ui` build tag if you intentionally do not want any admin UI assets embedded in the release binary.
- Keep `pb_hooks` and `pb_migrations` free of machine-specific paths, test credentials, or throwaway debugging strings before publishing a build.
- If your local Go cache path is not writable, set `GOCACHE=/tmp/pocketbase-gocache` for the build command.

When not to use the production flags:

- During local development, where full symbols and paths make debugging easier.
- When you are actively investigating a crash and want readable stack traces.
- When you need a byte-for-byte comparable binary for debugging a build pipeline issue.

## 12. Migrations And Hooks During Development

The example app enables both JS hooks and JS migrations through `plugins/jsvm` and `plugins/migratecmd`.

Defaults when running from `examples/base`:

- hooks dir: `./pb_hooks`
- migrations dir: `./pb_migrations`
- public dir: `./pb_public`
- hook files: `*.pb.js` or `*.pb.ts`

Useful command examples:

```sh
cd examples/base
go run main.go migrate up
go run main.go migrate create add_example_field
go run main.go migrate collections
```

## 13. Recommended Daily Workflow

### Backend-focused work

```sh
cd examples/base
go run main.go serve --dev
```

Use the embedded UI at `http://127.0.0.1:8090`.

### Frontend-focused work

Terminal 1:

```sh
cd examples/base
go run main.go serve --dev
```

Terminal 2:

```sh
cd ui
npm run dev
```

Open `http://localhost:5173`.

### Before finishing a frontend change

```sh
cd ui
npm run build
```

Then restart the backend if you want to verify the embedded UI version at `:8090`.

## 14. Feature References

- Collection admin actions API and behavior reference: [COLLECTION_ADMIN_ACTIONS_REFERENCE.md](./COLLECTION_ADMIN_ACTIONS_REFERENCE.md)
- Example hook registrations for `posts.status`: [examples/base/pb_hooks/collection_admin_actions.js](./examples/base/pb_hooks/collection_admin_actions.js)

## 15. Common Gotchas

- If you run `go run ./examples/base/main.go serve` from the repo root, your default relative paths will resolve from the root, not `examples/base`.
- If you only run the backend, you are testing the embedded `ui/dist` bundle, not the live Vite frontend.
- If you forget `npm run build`, the backend may still serve an old embedded UI.
- There is no built-in Go hot reload in this repo.
- The frontend dev server and backend server are separate processes during UI development.
- UI environment variables must use the `PB_` prefix.

## 16. Handy Command List

From repo root:

```sh
make test
make test-report
make lint
make jstypes
```

From `examples/base`:

```sh
go run main.go serve --dev
go run main.go migrate up
go build
go build -tags no_ui
```

From `ui`:

```sh
npm install
npm run dev
npm run build
```
