package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

type sample struct {
	rtt time.Duration
	err string
}

type tracker struct {
	history        []sample
	sent, received int
	total          time.Duration
	latencies      []time.Duration // Successful latencies, sorted for exact percentiles.
}

func (t *tracker) add(s sample, width int) {
	t.sent++
	if s.err == "" {
		t.received++
		t.total += s.rtt
		i := sort.Search(len(t.latencies), func(i int) bool { return t.latencies[i] >= s.rtt })
		t.latencies = append(t.latencies, 0)
		copy(t.latencies[i+1:], t.latencies[i:])
		t.latencies[i] = s.rtt
	}
	t.history = append(t.history, s)
	if len(t.history) > width {
		t.history = t.history[len(t.history)-width:]
	}
}

func paint(s, color string, enabled bool) string {
	if !enabled {
		return s
	}
	return "\x1b[" + color + "m" + s + "\x1b[0m"
}

func (t *tracker) line(width int, scale time.Duration, color bool) string {
	return fmt.Sprintf("[%s] %s", t.chart(width, scale, color), t.status(color))
}

func (t *tracker) chart(width int, scale time.Duration, color bool) string {
	var chart strings.Builder
	bars := []rune("▁▂▃▄▅▆▇█")
	history := t.history
	if len(history) > width {
		history = history[len(history)-width:]
	}
	for _, s := range history {
		if s.err != "" {
			chart.WriteString(paint("█", "1;31", color))
			continue
		}
		ratio := float64(s.rtt) / float64(scale)
		// Clamp before converting to int to handle very small scale values.
		level := 7
		if ratio < 1 {
			level = max(0, int(ratio*8))
		}
		shade := "32"
		if ratio >= 0.75 {
			shade = "33"
		}
		chart.WriteString(paint(string(bars[level]), shade, color))
	}
	for i := len(history); i < width; i++ {
		chart.WriteString("·")
	}
	return chart.String()
}

func (t *tracker) status(color bool) string {
	if len(t.history) == 0 {
		return "Waiting for first reply… | avg — | p95 — | loss —"
	}
	last := t.history[len(t.history)-1]
	status := paint(fmt.Sprintf("%.1f ms", float64(last.rtt)/float64(time.Millisecond)), "32", color)
	if last.err != "" {
		status = paint("█ "+last.err, "1;31", color)
	}
	avg := "—"
	p95 := "—"
	if t.received > 0 {
		avg = fmt.Sprintf("%.1f ms", float64(t.total)/float64(t.received)/float64(time.Millisecond))
		// Nearest-rank p95: ceil(0.95*n)-1, expressed with integer arithmetic.
		n := len(t.latencies)
		p95 = fmt.Sprintf("%.1f ms", float64(t.latencies[n-1-n/20])/float64(time.Millisecond))
	}
	lost := t.sent - t.received
	loss := fmt.Sprintf("loss %.1f%% (%d/%d)", 100*float64(lost)/float64(t.sent), lost, t.sent)
	if lost > 0 {
		loss = paint(loss, "1;31", color)
	}
	return fmt.Sprintf("%s | avg %s | p95 %s | %s", status, avg, p95, loss)
}

var latencyPattern = regexp.MustCompile(`(?i)time[=<]\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)

func parseLatency(output string) (time.Duration, error) {
	match := latencyPattern.FindStringSubmatch(output)
	if match == nil {
		return 0, errors.New("no latency in ping response")
	}
	ms, err := strconv.ParseFloat(match[1], 64)
	if err != nil || ms > float64(time.Duration(1<<63-1))/float64(time.Millisecond) {
		return 0, errors.New("invalid latency in ping response")
	}
	return time.Duration(ms * float64(time.Millisecond)), nil
}

func probe(ctx context.Context, binary, host string, timeout time.Duration) sample {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"-n", "-c", "1", host}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", host}
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return sample{err: "TIMEOUT"}
	}
	if err != nil {
		reason := "NO REPLY"
		lower := strings.ToLower(string(output))
		switch {
		case strings.Contains(lower, "unknown host"), strings.Contains(lower, "cannot resolve"), strings.Contains(lower, "name or service not known"), strings.Contains(lower, "could not find host"):
			reason = "DNS ERROR"
		case strings.Contains(lower, "unreachable"):
			reason = "UNREACHABLE"
		case strings.Contains(lower, "not permitted"), strings.Contains(lower, "permission denied"):
			reason = "PERMISSION DENIED"
		}
		return sample{err: reason}
	}
	rtt, err := parseLatency(string(output))
	if err != nil {
		return sample{err: "NO REPLY"}
	}
	return sample{rtt: rtt}
}

func run(ctx context.Context, args []string, out, errOut io.Writer, interactive bool) error {
	flags := flag.NewFlagSet("sping", flag.ContinueOnError)
	flags.SetOutput(errOut)
	interval := flags.Duration("interval", time.Second, "time between ping starts")
	timeout := flags.Duration("timeout", time.Second, "maximum wait for each reply")
	width := flags.Int("width", 0, "chart history limit (0 = terminal width; redirected output uses 40)")
	scale := flags.Duration("scale", 200*time.Millisecond, "latency represented by a full-height bar")
	count := flags.Int("count", 0, "stop after this many pings (0 = continuous)")
	noColor := flags.Bool("no-color", false, "disable ANSI colors")
	flags.Usage = func() {
		fmt.Fprintln(errOut, "Usage: sping [options] [host]\n\nLive ping history. Default host: 1.1.1.1. Press Ctrl+C to stop.\nOptions must precede the host.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *interval <= 0 || *timeout <= 0 || *scale <= 0 || *width < 0 || *count < 0 {
		return errors.New("interval, timeout and scale must be positive; width and count must be nonnegative")
	}
	host := "1.1.1.1"
	if flags.NArg() > 1 {
		return errors.New("expected a single host; options must precede the host")
	}
	if flags.NArg() == 1 {
		host = flags.Arg(0)
	}
	if host == "" || strings.HasPrefix(host, "-") || strings.ContainsAny(host, "\r\n\t\x1b ") {
		return errors.New("host must be an IP address or hostname")
	}
	binary, err := exec.LookPath("ping")
	if err != nil {
		return errors.New("ping executable not found; install your operating system's ping utility")
	}
	_, noColorEnv := os.LookupEnv("NO_COLOR")
	color := interactive && !*noColor && !noColorEnv && os.Getenv("TERM") != "dumb"
	t := tracker{}
	if interactive {
		// Reserve two lines in the normal terminal; leave prior output visible.
		fmt.Fprint(out, "\x1b[?25l\n")
		defer func() {
			fmt.Fprint(out, "\x1b[0m\x1b[?25h\n")
		}()
		drawScreen(out, &t, host, *width, *scale, color)
	} else {
		fmt.Fprintf(out, "sping → %s\n", host)
	}
	// Run probes separately so the screen can respond to resizing while waiting.
	results := make(chan sample)
	probeCtx, stopProbes := context.WithCancel(ctx)
	defer stopProbes()
	go func() {
		for probeCtx.Err() == nil {
			started := time.Now()
			s := probe(probeCtx, binary, host, *timeout)
			select {
			case results <- s:
			case <-probeCtx.Done():
				return
			}
			timer := time.NewTimer(max(0, *interval-time.Since(started)))
			select {
			case <-probeCtx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()
	resize := time.NewTicker(200 * time.Millisecond)
	defer resize.Stop()
	lastWidth, lastHeight := terminalSize(out)
	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil
		case <-resize.C:
			if interactive {
				w, h := terminalSize(out)
				if w != lastWidth || h != lastHeight {
					lastWidth, lastHeight = w, h
					drawScreen(out, &t, host, *width, *scale, color)
				}
			}
		case s := <-results:
			if ctx.Err() != nil {
				return nil
			}
			historyWidth := *width
			if historyWidth == 0 {
				historyWidth = 40
				if interactive {
					historyWidth, _ = terminalSize(out)
				}
			}
			t.add(s, historyWidth)
			if interactive {
				drawScreen(out, &t, host, *width, *scale, color)
			} else {
				fmt.Fprintln(out, t.line(historyWidth, *scale, false))
			}
			if *count > 0 && t.sent >= *count {
				return nil
			}
		}
	}
	return nil
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	interactive := term.IsTerminal(int(os.Stdout.Fd())) && os.Getenv("TERM") != "dumb"
	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr, interactive); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "sping:", err)
		os.Exit(1)
	}
}
