![LLM Service Monitor banner](docs/assets/banner.svg)

# LLM Service Monitor

Single-image monitor for OpenAI-compatible LLM services. The Go backend schedules probes, persists PostgreSQL history, sends model lifecycle alerts by SMTP, and serves a Vue/Vuetify dashboard from embedded static assets.

## Quick Start

```bash
docker compose up --build
```

Docker Compose exposes the app at [http://localhost:18080](http://localhost:18080). The sample compose config disables OAuth and SMTP and points the target at a local placeholder API. For a real target, copy [config.example.yaml](config.example.yaml), fill in the endpoint, auth, SMTP, and certificate settings, then mount secrets read-only.

## Development

```bash
go test ./...
cd web
npm install
npm run build
```

The production image builds the Vue app first, embeds `web/dist` into `cmd/server/static`, and then compiles a rootless Go runtime image.

## Documentation

- [Architecture](docs/architecture.md)
- [Configuration](docs/configuration.md)
- [Deployment](docs/deployment.md)
- [Operations](docs/operations.md)
- [API](docs/api.md)
- [Development](docs/development.md)
