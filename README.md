# sping

A terminal ping monitor with a two-line, full-width rolling bar chart. Successful replies
appear as green latency bars (yellow near the top of the scale). Every failed ping
appears as a **solid red bar**, so outages stay visible after the connection recovers.
The status bar above the chart shows current latency, average successful latency,
p95, and packet loss. The running chart occupies the second line and adapts to
the terminal width, leaving one column free to prevent wrapping. It stays in the
normal terminal view, preserving previous output. Ctrl+C stops the monitor and
leaves the final two lines visible. On narrow terminals the hostname is shortened
first, then the status line is truncated to prevent wrapping.
Current latency is green; failures and nonzero packet loss are red.
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
and the oldest scroll off the left. Bars use a fixed
scale (default: 200 ms) so heights remain comparable over time. Adjust it with
`-scale 50ms` for a faster connection.

By default, history fills the terminal width. Use `-width 50` to limit the chart
to the latest 50 pings. Failed pings appear as solid red bars.

Run `go run ./cmd/sping -help` for all options. Use `-no-color` or set `NO_COLOR`
to disable colors. Redirected output produces one plain-text line per sample
with a compact chart (40 samples by default) and statistics, without screen controls.
Statistics cover the full run, including samples that have left the chart.

Requires Go 1.27.3 or newer and the system `ping` utility on your PATH (on Linux,
usually provided by `iputils-ping`). Uses ICMP via the system utility without Go
networking dependencies. Terminal sizing uses `golang.org/x/term`. Each probe
deadline includes DNS lookup; replies taking longer than
`-timeout` count as failures. Probes run sequentially, so a slow probe can stretch
the requested interval. Windows support expects English ping output.

Build and test:

```sh
go build -o sping ./cmd/sping
go test ./...
```

## License

Licensed under the [MIT License](./LICENSE).
