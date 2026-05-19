FROM node:24-alpine AS frontend
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.25.10-alpine AS backend
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN rm -rf cmd/server/static && mkdir -p cmd/server/static
COPY --from=frontend /src/web/dist/ ./cmd/server/static/
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/llm-monitor ./cmd/server

FROM alpine:3.22
RUN addgroup -S -g 1000 appuser && adduser -S -D -H -u 1000 -G appuser appuser
RUN apk add --no-cache ca-certificates
USER appuser:appuser
WORKDIR /app
COPY --from=backend /out/llm-monitor /app/llm-monitor
ENV LLM_MONITOR_CONFIG=/config/config.yaml
EXPOSE 8080
ENTRYPOINT ["/app/llm-monitor"]
