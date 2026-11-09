package output

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// TestETA covers #191's estimator with a fake clock.
func TestETA(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e := &ETA{Now: func() time.Time { return now }}
	tick := func(processed int64) (time.Duration, bool) {
		left, ok := e.Update(processed, 1000)
		now = now.Add(time.Second)
		return left, ok
	}
	// 10 names a second; nothing during the 5 s warm-up.
	for i := int64(0); i < 5; i++ {
		if _, ok := tick(i * 10); ok {
			t.Fatalf("estimate during warm-up at %d s", i)
		}
	}
	left, ok := tick(50)
	if !ok || left != 95*time.Second {
		t.Fatalf("after warm-up: %v %v, want 95s (950 names at 10/s)", left, ok)
	}
	// The rate drops to 1/s: within the 30 s window the estimate follows.
	p := int64(50)
	for range 40 {
		p++
		left, ok = tick(p)
	}
	if !ok || left < 800*time.Second {
		t.Errorf("after slowing to 1/s: %v, want at least the 909 names left at about 1/s", left)
	}
	// Done: no estimate.
	if _, ok := tick(1000); ok {
		t.Error("estimate at 100%")
	}
	// The next -dL target starts over, warm-up included.
	if _, ok := tick(1); ok {
		t.Error("estimate right after progress went back to the start")
	}
	// Stalled since warm-up: nothing to extrapolate from.
	e2 := &ETA{Now: func() time.Time { return now }}
	for range 8 {
		if _, ok := e2.Update(0, 100); ok {
			t.Fatal("estimate with no progress")
		}
		now = now.Add(time.Second)
	}
}

func TestFormatETA(t *testing.T) {
	for d, want := range map[time.Duration]string{
		252*time.Second + 400*time.Millisecond: "4m12s",
		200 * time.Millisecond:                 "1s",
		3723 * time.Second:                     "1h2m3s",
	} {
		if got := FormatETA(d); got != want {
			t.Errorf("FormatETA(%v) = %q, want %q", d, got, want)
		}
	}
}

// TestProgressShowsETA: the progress line gains the estimate after warm-up.
func TestProgressShowsETA(t *testing.T) {
	var buf bytes.Buffer
	w := New(nil, false, FormatText)
	w.stderr = &buf
	w.SetProgressLines(true)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	w.eta.Now = func() time.Time { return now }
	for i := int64(0); i <= 6; i++ {
		w.Progress(float64(i), i*10, 1000, 0)
		now = now.Add(time.Second)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if strings.Contains(lines[0], "ETA") {
		t.Errorf("first line %q shows an ETA during warm-up", lines[0])
	}
	if last := lines[len(lines)-1]; !strings.HasSuffix(last, "| ETA 1m34s") {
		t.Errorf("last line %q, want it to end with the ETA", last)
	}
}
