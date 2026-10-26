# sping

A terminal ping monitor with a rolling, single-line bar chart. Successful replies
appear as green latency bars (yellow near the top of the scale). Every failed ping
appears as a **solid red bar**, so outages stay visible after the connection recovers.
The line also shows current latency, average successful latency, p95, and packet loss.
P95 uses the nearest-rank 95th percentile of successful replies across the full run;
timeouts are counted in packet loss instead. Successful latencies are retained in
memory to calculate this exactly.

```sh
go run ./cmd/sping
go run ./cmd/sping -interval 500ms -timeout 1s -width 50 8.8.8.8
go run ./cmd/sping -count 10 localhost
```

Options go before the host. The default host is `1.1.1.1`. Press **Ctrl+C** to
stop. The chart fills from left to right. Once full, new pings enter on the right
and the oldest scroll off the left. `·` means no sample yet. Bars use a fixed
scale (default: 200 ms) so heights remain comparable over time. Adjust it with
`-scale 50ms` for a faster connection.

```text
sping → 1.1.1.1 | ▁ low → █ 200ms+
[▁▂▁█▁▂····] 28.0 ms | avg 21.2 ms | p95 35.0 ms | loss 16.7% (1/6)
```

Run `go run ./cmd/sping -help` for all options. Use `-no-color` or set `NO_COLOR`
to disable colors. Redirected output produces one plain-text line per sample.
Statistics cover the full run, including samples that have left the chart.

Requires Go 1.27.3 or newer and the system `ping` utility on your PATH (on Linux,
usually provided by `iputils-ping`). Uses ICMP via the system utility without Go
dependencies. Each probe deadline includes DNS lookup; replies taking longer than
`-timeout` count as failures. Probes run sequentially, so a slow probe can stretch
the requested interval. Windows support expects English ping output.

Build and test:

```sh
go build -o sping ./cmd/sping
go test ./...
```
