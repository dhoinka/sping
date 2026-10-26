package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseLatency(t *testing.T) {
	for _, tt := range []struct {
		output string
		want   time.Duration
	}{
		{"64 bytes from 1.1.1.1: icmp_seq=0 ttl=57 time=12.345 ms", 12345 * time.Microsecond},
		{"Reply from 127.0.0.1: bytes=32 time<1ms TTL=128", time.Millisecond},
		{"64 bytes from ::1: icmp_seq=1 ttl=64 time=0.031 ms", 31 * time.Microsecond},
	} {
		got, err := parseLatency(tt.output)
		if err != nil || got != tt.want {
			t.Errorf("parseLatency(%q) = %v, %v; want %v", tt.output, got, err, tt.want)
		}
	}
	if _, err := parseLatency("Request timeout for icmp_seq 0"); err == nil {
		t.Fatal("timeout must not count as success")
	}
}

func TestFailureStaysVisibleAndStatsSurviveHistoryRollover(t *testing.T) {
	tr := tracker{}
	tr.add(sample{rtt: 10 * time.Millisecond}, 2)
	tr.add(sample{err: "TIMEOUT"}, 2)
	tr.add(sample{rtt: 30 * time.Millisecond}, 2)
	line := tr.line(2, 200*time.Millisecond, true)
	for _, want := range []string{"\x1b[1;31m█\x1b[0m", "avg 20.0 ms", "loss 33.3% (1/3)"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q missing %q", line, want)
		}
	}
	if len(tr.history) != 2 {
		t.Fatalf("history should be bounded: %d", len(tr.history))
	}
	plain := tr.line(2, 200*time.Millisecond, false)
	if !strings.HasPrefix(plain, "[█▂]") {
		t.Fatalf("newest reply must appear after the older failure: %q", plain)
	}
	if strings.Contains(plain, "\x1b") || !strings.Contains(plain, "█") {
		t.Fatalf("plain output must preserve failures without ANSI: %q", plain)
	}
}

func TestChartFillsFromLeftThenScrollsLeft(t *testing.T) {
	tr := tracker{}
	for i, want := range []string{"[▁··]", "[▁▂·]", "[▁▂▃]", "[▂▃▄]"} {
		tr.add(sample{rtt: time.Duration(i) * 25 * time.Millisecond}, 3)
		line := tr.line(3, 200*time.Millisecond, false)
		if !strings.HasPrefix(line, want) {
			t.Fatalf("after ping %d: expected %s, got %q", i+1, want, line)
		}
	}
}

func TestAllFailures(t *testing.T) {
	tr := tracker{}
	tr.add(sample{err: "TIMEOUT"}, 3)
	line := tr.line(3, time.Millisecond, false)
	for _, want := range []string{"[█··]", "█ TIMEOUT", "avg — | p95 —", "loss 100.0% (1/1)"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q missing %q", line, want)
		}
	}
}

func TestP95UsesSuccessfulRepliesAcrossFullRun(t *testing.T) {
	tr := tracker{}
	for i := 20; i >= 1; i-- {
		tr.add(sample{rtt: time.Duration(i) * time.Millisecond}, 2)
	}
	tr.add(sample{err: "TIMEOUT"}, 2)
	line := tr.line(2, 200*time.Millisecond, false)
	if !strings.Contains(line, "avg 10.5 ms | p95 19.0 ms") {
		t.Fatalf("expected nearest-rank p95 across successful replies, including older history: %q", line)
	}
}
