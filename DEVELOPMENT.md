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

Unless a section explicitly says otherwise, run Go commands from the project root.

### Backend only

Use this when you want to run PocketBase with the embedded admin UI bundle from `ui/dist`.

```sh
go run ./examples/base serve --dev \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
```

Open: `http://127.0.0.1:8090`

Notes:

- `--dev` enables dev mode logging and SQL output.
- The explicit path flags keep `pb_data`, `pb_public`, `pb_hooks`, and `pb_migrations` in `examples/base` even though the command is run from the project root.
- The UI served at `:8090` is the built bundle from `ui/dist`. It does not hot reload.
- Changes in `ui/src` will not appear here until you rebuild the frontend bundle with `cd ui && npm run build`.

### Full-stack development with frontend hot reload

Use two terminals.

Terminal 1:

```sh
go run ./examples/base serve --dev \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
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

These are the important local directories for the example app:

| Path | Purpose |
| --- | --- |
| `examples/base/pb_data` | SQLite DBs, app data, runtime files |
| `examples/base/pb_public` | Static files served by the catch-all route |
| `examples/base/pb_hooks` | JS app hooks loaded by `plugins/jsvm` |
| `examples/base/pb_migrations` | JS migrations loaded by `plugins/jsvm` and `migratecmd` |

Default behavior comes from:

- `pocketbase.New()` using the configured `--dir` and plugin directory flags
- `examples/base/main.go` registering JS hooks, migrations, and public directory support

## 7. Useful Backend Flags

Common flags while running the example app:

```sh
go run ./examples/base serve --dev \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
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

PocketBase builds as a terminal CLI executable, not a macOS Finder `.app` bundle.

On macOS, a correct build is usually shown by Finder as a Unix executable and by `file` as a Mach-O executable. You run it from the terminal, for example:

```sh
.builds/pocketbase serve
```

Build from the repo root and explicitly target `./examples/base`.

Do not run a bare `go build -o .builds/pocketbase` from the repo root when you want a runnable PocketBase server. The repo root is the library package, while `examples/base` is the standalone `main` package used for executable builds. A bare repo-root build can produce a package archive that Finder may show as a generic document instead of a Unix executable.

### Development CLI build

Use this when you want a quick local binary with normal debug metadata:

```sh
mkdir -p .builds
go build -o .builds/pocketbase ./examples/base
```

Verify the output from the repo root:

```sh
file .builds/pocketbase
.builds/pocketbase --help
```

On Apple Silicon macOS, `file` should print something like:

```text
.builds/pocketbase: Mach-O 64-bit executable arm64
```

On Intel macOS, it should say `x86_64` instead of `arm64`.

Run the built development binary from the project root. Because the executable lives in `.builds`, always pass the local runtime directories explicitly:

```sh
.builds/pocketbase serve --dev \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
```

### Production CLI build

Build the frontend bundle first if the release should include the latest embedded admin UI:

```sh
cd ui
npm run build
```

Then return to the project root and build the standalone CLI executable:

```sh
cd ..
mkdir -p .builds
go build -trimpath -ldflags="-s -w -buildid=" -o .builds/pocketbase ./examples/base
```

Command breakdown:

- `go build`: compile a Go package.
- `-trimpath`: remove local filesystem paths from compiled package metadata.
- `-ldflags="-s -w -buildid="`: pass linker flags that strip symbol/debug tables and clear the Go build id.
- `-o .builds/pocketbase`: write the output binary to the gitignored `.builds` directory.
- `./examples/base`: build the standalone PocketBase `main` package, not the repo-root library package.

Verify the production output from the repo root:

```sh
file .builds/pocketbase
.builds/pocketbase --version
.builds/pocketbase --help
```

Run it from the project root with explicit local runtime paths:

```sh
.builds/pocketbase serve \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
```

If the local Go cache is not writable, use a temporary cache path:

```sh
GOCACHE=/tmp/pocketbase-gocache go build -trimpath -ldflags="-s -w -buildid=" -o .builds/pocketbase ./examples/base
```

### Backend-only build without embedded UI

```sh
mkdir -p .builds
go build -tags no_ui -o .builds/pocketbase-no-ui ./examples/base
```

With the `no_ui` build tag, `ui/embed_no_ui.go` is used and the admin UI is not embedded in the binary.

### Cross-compile a static Linux binary

```sh
mkdir -p .builds
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -buildid=" -o .builds/pocketbase-linux-amd64 ./examples/base
```

### Build notes

- By default, `go build` writes the binary into the current directory and names it after the package directory.
- From `examples/base`, the default output name is `base` on macOS/Linux and `base.exe` on Windows.
- For repo-local artifacts, use the gitignored `/.builds/` directory with `-o .builds/pocketbase` from the repo root.
- Always include `./examples/base` in repo-root build commands so Go builds the executable `main` package.
- The production command is the same executable build as the development command, with extra path-trimming and linker-stripping flags.
- A macOS `.app` bundle is not produced by these commands. That requires separate app packaging/signing work and is not how PocketBase is normally distributed.
- If Finder shows the output as a generic document or `file` reports `current ar archive`, you built the wrong package or are inspecting a stale/wrong output file. Remove that output and rebuild from the repo root with `go build ... ./examples/base`.
- If `file` reports `Mach-O 64-bit executable` but Finder still labels it generically, trust `file` and run it from the terminal. Finder labels are not the source of truth for Go CLI binaries.
- These flags help reduce the amount of machine-specific information visible in a decompiled or inspected binary.
- They do not remove strings that you hardcode into Go code, JS hooks, templates, HTML, or the embedded frontend bundle.
- For that reason, never put secrets, private URLs, API keys, passwords, or absolute development paths directly into source files that are compiled or embedded.
- Prefer runtime configuration through environment variables, app settings, or external secret managers.
- If you ship the embedded UI, inspect `ui/dist` before release and make sure it does not contain development-only values from your local `.env` files.
- Use the `no_ui` build tag if you intentionally do not want any admin UI assets embedded in the release binary.
- Keep `pb_hooks` and `pb_migrations` free of machine-specific paths, test credentials, or throwaway debugging strings before publishing a build.

When not to use the production flags:

- During local development, where full symbols and paths make debugging easier.
- When you are actively investigating a crash and want readable stack traces.
- When you need a byte-for-byte comparable binary for debugging a build pipeline issue.

## 12. Migrations And Hooks During Development

The example app enables both JS hooks and JS migrations through `plugins/jsvm` and `plugins/migratecmd`.

Defaults if you run from `examples/base` without explicit flags:

- hooks dir: `./pb_hooks`
- migrations dir: `./pb_migrations`
- public dir: `./pb_public`
- hook files: `*.pb.js` or `*.pb.ts`

Useful command examples:

```sh
go run ./examples/base migrate up --migrationsDir ./examples/base/pb_migrations
go run ./examples/base migrate create add_example_field --migrationsDir ./examples/base/pb_migrations
go run ./examples/base migrate collections --migrationsDir ./examples/base/pb_migrations
```

## 13. Recommended Daily Workflow

### Backend-focused work

```sh
go run ./examples/base serve --dev \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
```

Use the embedded UI at `http://127.0.0.1:8090`.

### Frontend-focused work

Terminal 1:

```sh
go run ./examples/base serve --dev \
  --dir ./examples/base/pb_data \
  --hooksDir ./examples/base/pb_hooks \
  --migrationsDir ./examples/base/pb_migrations \
  --publicDir ./examples/base/pb_public
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

- If you run the example app from the repo root without `--dir`, `--hooksDir`, `--migrationsDir`, and `--publicDir`, default relative paths will resolve from the root, not `examples/base`.
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
go run ./examples/base serve --dev --dir ./examples/base/pb_data --hooksDir ./examples/base/pb_hooks --migrationsDir ./examples/base/pb_migrations --publicDir ./examples/base/pb_public
go run ./examples/base migrate up --migrationsDir ./examples/base/pb_migrations
go build -o .builds/pocketbase ./examples/base
go build -tags no_ui -o .builds/pocketbase-no-ui ./examples/base
```

From `ui`:

```sh
npm install
npm run dev
npm run build
```
