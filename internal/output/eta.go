package output

import "time"

// ETA estimates the time left in a scan from its progress (#191). It uses
// the rate over the last etaWindow of samples, so a scan that slows down
// (a resolver starts rate-limiting) or speeds up is reflected within
// seconds, and says nothing until etaWarmup of samples exist.
type ETA struct {
	// Now is the clock; nil means time.Now. Tests set a fake one.
	Now func() time.Time

	first   time.Time
	samples []etaSample
}

type etaSample struct {
	at        time.Time
	processed int64
}

const (
	etaWarmup = 5 * time.Second
	etaWindow = 30 * time.Second
)

// Update records processed out of total and returns the estimated time
// left. ok is false during warm-up, once the scan is complete, and while
// there is no progress to extrapolate from. A drop in processed (the next
// -dL target) starts the estimate over.
func (e *ETA) Update(processed, total int64) (left time.Duration, ok bool) {
	now := time.Now()
	if e.Now != nil {
		now = e.Now()
	}
	if n := len(e.samples); n > 0 && processed < e.samples[n-1].processed {
		e.samples = e.samples[:0]
	}
	if len(e.samples) == 0 {
		e.first = now
	}
	e.samples = append(e.samples, etaSample{at: now, processed: processed})
	// Keep the samples inside the window, and always at least two.
	cut := 0
	for cut < len(e.samples)-2 && now.Sub(e.samples[cut].at) > etaWindow {
		cut++
	}
	if cut > 0 {
		e.samples = append(e.samples[:0], e.samples[cut:]...)
	}
	if processed >= total || now.Sub(e.first) < etaWarmup {
		return 0, false
	}
	oldest := e.samples[0]
	elapsed, done := now.Sub(oldest.at), processed-oldest.processed
	if elapsed <= 0 || done <= 0 {
		return 0, false
	}
	return time.Duration(float64(elapsed) * float64(total-processed) / float64(done)), true
}

// FormatETA renders an estimate to the second, such as "4m12s".
func FormatETA(d time.Duration) string {
	return max(d.Round(time.Second), time.Second).String()
}
