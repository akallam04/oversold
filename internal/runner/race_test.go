package runner

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/akallam04/oversold/internal/history"
	"github.com/akallam04/oversold/internal/report"
)

func TestBarrierReleasesEveryAction(t *testing.T) {
	const n = 8
	b := NewBarrier(n)
	var passed atomic.Int32
	var wg sync.WaitGroup
	results := make([]bool, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = b.Wait()
			passed.Add(1)
		}()
	}

	b.WaitAll()
	time.Sleep(20 * time.Millisecond)
	if got := passed.Load(); got != 0 {
		t.Fatalf("%d actions passed the barrier before release", got)
	}
	b.Release(true)
	wg.Wait()
	if got := passed.Load(); got != n {
		t.Fatalf("%d of %d actions released", got, n)
	}
	for i, ok := range results {
		if !ok {
			t.Errorf("action %d was told not to proceed", i)
		}
	}
}

func TestBarrierAbort(t *testing.T) {
	b := NewBarrier(2)
	out := make(chan bool, 2)
	for range 2 {
		go func() { out <- b.Wait() }()
	}
	b.WaitAll()
	b.Release(false)
	b.Release(true)
	if <-out || <-out {
		t.Error("aborted barrier let an action proceed")
	}
}

func TestJitterIsSeeded(t *testing.T) {
	lo, hi := 0*time.Millisecond, 50*time.Millisecond
	a := Jitter(42, 3, 5, lo, hi)
	b := Jitter(42, 3, 5, lo, hi)
	if !slices.Equal(a, b) {
		t.Errorf("same seed and run gave %v and %v", a, b)
	}
	if slices.Equal(a, Jitter(43, 3, 5, lo, hi)) && slices.Equal(a, Jitter(42, 4, 5, lo, hi)) {
		t.Errorf("different seed and run gave the same jitter %v", a)
	}
	for _, d := range a {
		if d < lo || d > hi {
			t.Errorf("jitter %v outside [%v, %v]", d, lo, hi)
		}
	}
	for _, d := range Jitter(1, 1, 3, 0, 0) {
		if d != 0 {
			t.Errorf("zero range gave %v", d)
		}
	}
}

func TestRunAllKeepsGoingAfterHarnessErrors(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		pattern string
		ran     int
		stopped string
	}{
		{"no errors", "ooooo", 5, ""},
		{"scattered errors", "xxoxxoxx", 8, ""},
		{"three in a row", "oxxxoo", 4, "stopped after 3 harness errors in a row, 4 of 6 runs recorded"},
		{"three in a row at the end", "ooxxx", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls []int
			entries, stopped, err := runAll(context.Background(), len(tt.pattern), func(i int) (report.Entry, error, error) {
				calls = append(calls, i)
				e := report.Entry{File: fmt.Sprintf("run-%03d.json", i), History: &history.History{}}
				if tt.pattern[i-1] == 'x' {
					return e, boom, nil
				}
				return e, nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != tt.ran || len(calls) != tt.ran {
				t.Errorf("ran %d runs, want %d", len(calls), tt.ran)
			}
			if stopped != tt.stopped {
				t.Errorf("stopped = %q, want %q", stopped, tt.stopped)
			}
		})
	}
}
