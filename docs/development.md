# Development

## Prerequisites

- Go 1.25.10 or newer
- Node.js compatible with the lockfile
- PostgreSQL and Redis for full local runtime testing

## Backend

```bash
go test ./...
go run ./cmd/server
go run ./cmd/scheduler
go run ./cmd/worker
```

Set `LLM_MONITOR_CONFIG` when running with a config file outside the repository root.

## Frontend

```bash
cd web
npm install
npm run dev
npm run build
```

The Vite dev server proxies `/api` and `/healthz` to `http://localhost:8080`.

After changing frontend code, run the production build. The Dockerfile copies
`web/dist` into `cmd/server/static` during the image build, so local embedded-static
checks should compare those folders when you manually refresh `cmd/server/static`.

## Code Organization

- Keep Go packages organized by responsibility; avoid rebuilding large catch-all files.
- Keep scheduled work under `internal/schedule`: queue integration in `queue`, generic task contracts in `runner`, task wiring in `tasks`, category handlers in `tasks/checks`, `tasks/models`, and `tasks/retention`, and shared task contracts in `tasks/shared`.
- Keep Vue feature code under `web/src/features/dashboard`.
- Keep reusable stateful frontend logic in composables and pure formatting/mapping logic in `utils`.
- Add Go doc comments for production types/functions and JSDoc comments for exported frontend composables, types, and named helpers.

## Verification Checklist

Run before shipping changes:

```bash
go test ./...
cd web && npm run build
diff -qr web/dist ../cmd/server/static
docker compose config
```

## Related Docs

- [Architecture](architecture.md) explains package responsibilities before changing backend boundaries.
- [API](api.md) documents response contracts used by the dashboard.
- [Scheduled Tasks](tasks/README.md) covers queued task behavior and monitor task tests.
