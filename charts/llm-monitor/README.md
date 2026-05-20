# LLM Service Monitor Helm Chart

This chart deploys LLM Service Monitor on Kubernetes or OpenShift with Gateway API by default, an external PostgreSQL DSN, and Kubernetes Secrets generated from values.

## Install

```bash
helm upgrade --install llm-monitor ./charts/llm-monitor \
  --namespace llm-monitor \
  --create-namespace \
  -f my-values.yaml
```

## OpenShift and Kyverno Security

The pod security settings are hardcoded in the deployment template:

- `runAsUser: 1000`
- `runAsGroup: 1000`
- `runAsNonRoot: true`
- `readOnlyRootFilesystem: true`
- `allowPrivilegeEscalation: false`
- `capabilities.drop: ["ALL"]`
- `seccompProfile.type: RuntimeDefault`

The namespace SecurityContextConstraints must allow UID `1000`. A default restricted OpenShift namespace may reject a fixed UID unless the SCC or namespace range permits it.

## Configuration

The application config is generated from `values.config` and mounted at `/config/config.yaml`. The chart does not deploy PostgreSQL; set `config.postgres.dsn` to a reachable external database.

Plain secret and certificate values are created with `stringData`:

```yaml
secretFiles:
  data:
    target-api-key: "replace-me"
    oauth-client-secret: "replace-me"
    smtp-password: "replace-me"
    mcp-bearer-token: "replace-me"

certFiles:
  data:
    llm-api-ca.crt: |
      -----BEGIN CERTIFICATE-----
      ...
      -----END CERTIFICATE-----
```

Reference those files from `values.config` using `/run/secrets/<key>` and `/run/certs/<key>`. Do not commit real secret values to source control.

For OAuth mTLS certificates that are already base64-encoded, set the data next
to the mTLS file paths. The chart writes these values to the certificate Secret
with Kubernetes `data` and mounts them at `/run/certs`:

```bash
helm upgrade --install llm-monitor ./charts/llm-monitor \
  --set-string config.auth.client_secret="$CLIENT_SECRET" \
  --set-string config.auth.mtls.cert_file_data="$CERT_CRT_BASE64" \
  --set-string config.auth.mtls.key_file_data="$CERT_KEY_BASE64"
```

The generated Secret keys are derived from `config.auth.mtls.cert_file` and
`config.auth.mtls.key_file`, which default to `client.crt` and `client.key`.
`config.auth.client_secret` is rendered inline in the generated ConfigMap; use
`secretFiles.data.oauth-client-secret` instead when the value should stay in a
mounted Kubernetes Secret.

## Gateway API

Gateway API is enabled by default and renders an `HTTPRoute` that forwards `PathPrefix /` to the application Service on port `8080`.

By default the chart references an existing Gateway named `default` in the release namespace:

```yaml
gateway:
  enabled: true
  create: false
  parentRefs:
    - name: default
  hostnames:
    - monitor.apps.example.com
```

If the platform team manages the Gateway in another namespace, set the namespace in `parentRefs`:

```yaml
gateway:
  parentRefs:
    - name: shared-gateway
      namespace: openshift-ingress
      sectionName: http
```

The chart can also create a Gateway when `gateway.create` is enabled. In that mode, `gateway.gatewayClassName` is required and the Gateway name is taken from the first `gateway.parentRefs` entry so the `HTTPRoute` attaches to the generated Gateway.

```yaml
gateway:
  enabled: true
  create: true
  gatewayClassName: openshift-default
  parentRefs:
    - name: llm-monitor-gateway
  listeners:
    - name: http
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: Same
```

Set `gateway.enabled: false` if the cluster does not have Gateway API CRDs installed.

## ingress-nginx

For clusters that already run ingress-nginx, enable the Kubernetes `Ingress` alternative and disable Gateway API rendering:

```yaml
gateway:
  enabled: false
ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "60"
  hosts:
    - host: monitor.apps.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: monitor-tls
      hosts:
        - monitor.apps.example.com
```

## OpenShift Route

OpenShift Route support is kept as an alternative for clusters that do not use Gateway API. It is disabled by default. Leave `route.host` empty to let OpenShift generate the hostname, or set it explicitly:

```yaml
gateway:
  enabled: false
route:
  enabled: true
  host: monitor.apps.example.com
  tls:
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
```
