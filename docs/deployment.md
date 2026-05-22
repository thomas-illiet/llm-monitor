# Deployment

The service runs the same image as separate API, scheduler, and worker containers backed by PostgreSQL and Redis. The Dockerfile builds the Vue dashboard, embeds it into the API binary, and runs as a non-root user.

## Runtime Shape

- App container: Go HTTP server, embedded Vue dashboard, API, metrics, optional MCP endpoint.
- Scheduler container: Asynq periodic schedules and dynamic per-model schedule sync.
- Worker container: queued monitor task execution.
- PostgreSQL: persisted checks, runs, model inventory, events, alerts, and dashboard history.
- Redis: Asynq queues and retained manual task status.
- Optional SMTP relay: model lifecycle alert delivery.

## Docker Compose

```bash
docker compose up --build
```

Compose starts PostgreSQL, Redis, MailDev, the API server, scheduler, and worker. It waits for PostgreSQL and Redis to become healthy, mounts the sample config, and exposes the app on port `18080`.

## Production Container

Build the image:

```bash
docker build -t llm-service-monitor:latest .
```

Or pull the published multi-architecture image from GitHub Container Registry:

```bash
docker pull ghcr.io/thomas-illiet/llm-monitor:latest
docker pull ghcr.io/thomas-illiet/llm-monitor:1.2.3
```

Run it with:

- `LLM_MONITOR_CONFIG=/config/config.yaml`
- a read-only mounted config file
- read-only mounted API CA and mTLS cert/key files when enabled
- a PostgreSQL DSN reachable from the container
- a Redis address reachable from the API, scheduler, and worker containers

Example:

```bash
docker run --rm \
  -p 8080:8080 \
  -e LLM_MONITOR_CONFIG=/config/config.yaml \
  -v "$PWD/config.yaml:/config/config.yaml:ro" \
  ghcr.io/thomas-illiet/llm-monitor:latest
```

Run the scheduler and worker from the same image with
`/app/llm-monitor-scheduler` and `/app/llm-monitor-worker` as the container
command.

Images are published from the `main` branch and `v*` tags. Use `latest` for the current default-branch image, or version tags such as `1.2.3` for releases. If the package is private, authenticate first with `docker login ghcr.io`.

## Runtime Notes

- The container exposes port `8080` internally.
- The root filesystem can be read-only.
- No Linux capabilities are required by the application.
- Database migrations are embedded and applied at startup with `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`.

## Related Docs

- [Configuration](configuration.md) explains required settings and secret file fields.
- [Operations](operations.md) covers health endpoints, Prometheus scraping, alerts, and maintenance.
- [Development](development.md) covers local build and verification commands.
