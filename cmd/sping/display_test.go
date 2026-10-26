package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTwoLinesShowsStatsAboveFullWidthGraph(t *testing.T) {
	tr := tracker{}
	tr.add(sample{err: "TIMEOUT"}, 120)
	tr.add(sample{rtt: 25 * time.Millisecond}, 120)
	rows := strings.Split(tr.twoLines("localhost", 120, 0, 100*time.Millisecond, true), "\r\n")
	if len(rows) != 2 {
		t.Fatalf("expected exactly two rows, got %d", len(rows))
	}
	if !strings.Contains(rows[0], "avg 25.0 ms | p95 25.0 ms") {
		t.Fatalf("missing status statistics: %q", rows[0])
	}
	for _, want := range []string{"\x1b[32m25.0 ms\x1b[0m", "\x1b[1;31mloss 50.0% (1/2)\x1b[0m"} {
		if !strings.Contains(rows[0], want) {
			t.Fatalf("missing status color %q: %q", want, rows[0])
		}
	}
	if !strings.HasPrefix(rows[1], "\x1b[2K\x1b[1;31m█\x1b[0m") {
		t.Fatalf("expected red failure bar: %q", rows[1])
	}
	plain := strings.Split(tr.twoLines("localhost", 120, 0, 100*time.Millisecond, false), "\r\n")
	graph := strings.TrimPrefix(plain[1], "\x1b[2K")
	if len([]rune(graph)) != 120 {
		t.Fatalf("graph should span the width: %q", graph)
	}
	if !strings.HasPrefix(graph, "█▃") {
		t.Fatalf("unexpected graph order or latency: %q", graph)
	}
}

func TestTwoLinesFitsNarrowTerminals(t *testing.T) {
	tr := tracker{}
	for i := 0; i < 10; i++ {
		tr.add(sample{rtt: time.Duration(i) * time.Millisecond}, 10)
	}
	for _, width := range []int{1, 10, 80} {
		rows := strings.Split(tr.twoLines("localhost", width, 0, time.Second, false), "\r\n")
		if len(rows) != 2 {
			t.Fatalf("wrong height: %d", len(rows))
		}
		for _, row := range rows {
			row = strings.TrimPrefix(row, "\x1b[2K")
			if len([]rune(row)) > width {
				t.Fatalf("row wraps at width %d: %q", width, row)
			}
		}
	}
}

func TestDrawPreservesTerminalOutsideTwoLines(t *testing.T) {
	var out bytes.Buffer
	drawScreen(&out, &tracker{}, "localhost", 0, time.Second, false)
	for _, forbidden := range []string{"\x1b[?1049", "\x1b[H", "\x1b[J"} {
		if strings.Contains(out.String(), forbidden) {
			t.Fatalf("unexpected full-screen control: %q", out.String())
		}
	}
	if !strings.HasPrefix(out.String(), "\r\x1b[1A\x1b[2K") {
		t.Fatal("redraw should only move up one row")
	}
}

func TestColoredStatusClipsWithoutBreakingEscapes(t *testing.T) {
	tr := tracker{}
	tr.add(sample{err: "TIMEOUT"}, 1)
	for _, width := range []int{1, 5, 10, 80} {
		status := strings.Split(tr.twoLines("localhost", width, 0, time.Second, true), "\r\n")[0]
		if visibleWidth(status) > width {
			t.Fatalf("colored status wraps at width %d: %q", width, status)
		}
		if strings.Contains(colorCode.ReplaceAllString(status, ""), "\x1b") {
			t.Fatalf("broken escape sequence: %q", status)
		}
		if !strings.Contains(status, "\x1b[1;31m") || !strings.HasSuffix(status, "\x1b[0m") {
			t.Fatalf("missing red color or reset: %q", status)
		}
	}
}
