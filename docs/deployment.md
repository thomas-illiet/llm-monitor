# Deployment

The service is intended to run as one container plus PostgreSQL. The Dockerfile builds the Vue dashboard, embeds it into the Go binary, and runs as a non-root user.

## Runtime Shape

- App container: Go HTTP server, embedded Vue dashboard, scheduler, API, metrics, optional MCP endpoint.
- PostgreSQL: persisted checks, runs, model inventory, events, alerts, and dashboard history.
- Optional SMTP target: model lifecycle alert delivery.

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

Or pull the published multi-architecture image from GitHub Container Registry:

```bash
docker pull ghcr.io/thomas-illiet/llm-monitor:latest
docker pull ghcr.io/thomas-illiet/llm-monitor:1.2.3
```

Run it with:

- `LLM_MONITOR_CONFIG=/config/config.yaml`
- a read-only mounted config file
- read-only mounted API CA, mTLS cert/key, and secret files when enabled
- a PostgreSQL DSN reachable from the container

Example:

```bash
docker run --rm \
  -p 8080:8080 \
  -e LLM_MONITOR_CONFIG=/config/config.yaml \
  -v "$PWD/config.yaml:/config/config.yaml:ro" \
  ghcr.io/thomas-illiet/llm-monitor:latest
```

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
