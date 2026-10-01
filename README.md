# devproxy

`devproxy` is a small reverse proxy for local development. It listens on the loopback interface and selects a configured upstream from `<route>.localhost`.

## Install

Download a prebuilt binary from [GitHub Releases](https://github.com/yokonao/devproxy/releases).
Every release ships with a build provenance attestation.

See [docs/install.md](docs/install.md) for other install methods and how to verify a release.

## Configuration

Copy `config.example.yaml` to `config.yaml` and edit the targets:

```yaml
port: 3000
root: http://127.0.0.1:8080
routes:
  api: http://127.0.0.1:8081
  worker: unix:///tmp/devproxy-worker.sock
```

Route names may contain lowercase ASCII letters, digits, and interior hyphens. Requests for `localhost` use `root`, `api.localhost` uses the `api` route, and every other host returns `404`.

Start the proxy:

```sh
go run . -config ./config.yaml
```

## Security boundaries

- The listener is fixed to `127.0.0.1`; TLS termination and remote access are out of scope.
- Only `localhost` and configured `<route>.localhost` hosts are accepted.
- Targets are fixed at startup from the local configuration. Request headers cannot select an arbitrary upstream.
- Client-provided `X-Forwarded-*` values are replaced with values derived from the incoming request.
- The configuration is trusted input. Restrict its permissions and only configure trusted HTTP, HTTPS, or Unix socket targets.
- Unix targets speak HTTP over an absolute local socket path.
- Header reads time out after 10 seconds, reads and writes after 60 seconds, idle connections after 120 seconds, and graceful shutdown after 10 seconds.
- Authentication, authorization, dynamic reload, and service discovery are out of scope.

## Development

```sh
go test ./...
golangci-lint run
golangci-lint fmt
```

## License

MIT
