# Documentation

![LLM Service Monitor banner](assets/banner.svg)

Use this index to jump to the right level of detail.

## Reading Order

1. [Architecture](architecture.md)
2. [Configuration](configuration.md)
3. [Deployment](deployment.md)
4. [Operations](operations.md)
5. [Scheduled Tasks](tasks/README.md)
6. [API](api.md)
7. [Development](development.md)

## Documentation Map

| Area | Page | Use it for |
| --- | --- | --- |
| Overview | [Architecture](architecture.md) | Service layers, package responsibilities, and data flow. |
| Setup | [Configuration](configuration.md) | YAML settings, secrets, schedules, retention, and dashboard options. |
| Runtime | [Deployment](deployment.md) | Docker Compose, production container runtime, and image notes. |
| Operations | [Operations](operations.md) | Health checks, Prometheus scraping, alerts, troubleshooting, and maintenance. |
| Scheduler | [Scheduled Tasks](tasks/README.md) | One file per recurring backend task and its persistence side effects. |
| Integration | [API](api.md) | JSON endpoints, Prometheus metrics, and optional MCP tools. |
| Development | [Development](development.md) | Local commands, code organization, and pre-ship verification. |

## Source Files

- Production template: [config.example.yaml](../config.example.yaml)
- Local Docker Compose template: [examples/config.compose.yaml](../examples/config.compose.yaml)
- Local embedding fixture: [examples/embedding-fixture.txt](../examples/embedding-fixture.txt)
