# Deployment

The service is intended to run as one container plus PostgreSQL. The Dockerfile builds the Vue dashboard, embeds it into the Go binary, and runs as a non-root user.

## Docker Compose

```bash
docker compose up --build
```

Compose starts PostgreSQL, waits for it to become healthy, mounts the sample config, and exposes the app on port `18080`.

## Production Container

Build the image:

```bash
docker build -t llm-service-monitor:latest .
```

Run it with:

- `LLM_MONITOR_CONFIG=/config/config.yaml`
- a read-only mounted config file
- read-only mounted API CA, mTLS cert/key, and secret files when enabled
- a PostgreSQL DSN reachable from the container

## Runtime Notes

- The container exposes port `8080` internally.
- The root filesystem can be read-only.
- No Linux capabilities are required by the application.
- Database migrations are embedded and applied at startup with `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`.
