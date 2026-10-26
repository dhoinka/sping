# Agent guide

## Project

sping is a small Go CLI that displays live ping latency and packet loss in a
two-line terminal display: statistics above a full-width running chart.
See [README.md](./README.md) for usage.

- [go.mod](./go.mod) defines the required Go version.
- [cmd/sping/](./cmd/sping/) contains the CLI and its tests.
- Runtime probing uses the system `ping` executable.

## Conventions

- Write idiomatic Go; prefer the standard library and keep changes focused.
- Format changed Go files with `gofmt`.
- Handle errors explicitly and use contexts for cancellation and timeouts.
- Keep terminal output readable and preserve plain output when redirected.
- Keep failed pings visible as red full-height bars. The chart fills from left
  to right, then scrolls older samples off the left.
- Update the README when flags or user-visible behavior change.

## Validation

Run these after code changes:

```sh
go test ./...
go vet ./...
go build -o sping ./cmd/sping
```

Add focused tests for meaningful behavior changes. Keep unit tests independent
of external network access; use localhost for manual ping checks. Do not commit
generated binaries.
