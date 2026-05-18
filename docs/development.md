# Development

## Prerequisites

- Go 1.25.10 or newer
- Node.js compatible with the lockfile
- PostgreSQL for full local runtime testing

## Backend

```bash
go test ./...
go run ./cmd/server
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

After changing frontend code, run the production build and resync `cmd/server/static` from `web/dist` so the embedded dashboard matches the current source.

## Code Organization

- Keep Go packages organized by responsibility; avoid rebuilding large catch-all files.
- Keep Vue feature code under `web/src/features/dashboard`.
- Keep reusable stateful frontend logic in composables and pure formatting/mapping logic in `utils`.
- Add Go doc comments for production types/functions and JSDoc comments for exported frontend composables, types, and named helpers.

## Verification Checklist

Run before shipping changes:

```bash
go test ./...
cd web && npx vue-tsc --noEmit && npm run build
diff -qr web/dist ../cmd/server/static
docker compose config
```
