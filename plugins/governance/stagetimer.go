package governance

import (
	"fmt"
	"strings"
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// StageTimer accumulates named stage durations for a single pass of a
// multi-step governance operation and emits them as one log line.
//
// It exists because the governance paths that matter most for correctness have
// no steady-state timing at all. The reset cycle in particular runs every
// workerInterval on one goroutine, and a Go ticker drops ticks when the
// receiver is slow, so a pass that overruns silently stretches the real reset
// cadence. The only externally visible symptom is a last_reset column that
// lags the window boundary, which is indistinguishable from a reset bug
// without per-stage numbers.
//
// It lives in this package rather than a shared utility package because every
// caller is governance instrumentation (reset sweep, dump, ghost
// reconciliation, virtual key save and reload) and because every one of those
// call sites already imports this package. That keeps the dependency graph
// unchanged.
//
// A StageTimer is not safe for concurrent use. Each pass builds its own.
type StageTimer struct {
	name   string
	start  time.Time
	last   time.Time
	stages []stageDuration
	fields []stageField
}

// stageDuration is one named stage and the time spent in it.
type stageDuration struct {
	name string
	took time.Duration
}

// stageField is one name/value pair appended to the summary line. Counts
// recorded this way are what make the durations interpretable: a dump stage
// taking eight seconds means nothing without the number of rows it wrote.
type stageField struct {
	name  string
	value any
}

// NewStageTimer starts a timer for the named operation. The name is used as the
// prefix of the emitted log line, so callers should pass a greppable tag such
// as "[reset-cycle-timing]".
func NewStageTimer(name string) *StageTimer {
	now := time.Now()
	return &StageTimer{name: name, start: now, last: now}
}

// Mark records the time elapsed since the previous mark under the given stage
// name. The first Mark measures from the timer's creation.
func (t *StageTimer) Mark(stage string) {
	if t == nil {
		return
	}
	now := time.Now()
	t.stages = append(t.stages, stageDuration{name: stage, took: now.Sub(t.last)})
	t.last = now
}

// Field attaches a name/value pair to the summary line, for the counts that
// give the stage durations meaning.
func (t *StageTimer) Field(name string, value any) {
	if t == nil {
		return
	}
	t.fields = append(t.fields, stageField{name: name, value: value})
}

// Total returns the time elapsed since the timer was created.
func (t *StageTimer) Total() time.Duration {
	if t == nil {
		return 0
	}
	return time.Since(t.start)
}

// Log emits one summary line covering every field and stage. It logs at Info
// when the total reaches threshold and at Debug otherwise, so a healthy pass
// stays quiet while a slow one is visible without enabling debug logging for
// the whole process.
func (t *StageTimer) Log(logger schemas.Logger, threshold time.Duration) {
	if t == nil || logger == nil {
		return
	}
	total := t.Total()
	line := t.summary(total)
	if total >= threshold {
		logger.Info("%s", line)
	} else {
		logger.Debug("%s", line)
	}
}

// summary renders the timer as a single line: the name, the total, every
// attached field, then every stage in the order it was marked.
func (t *StageTimer) summary(total time.Duration) string {
	var b strings.Builder
	b.WriteString(t.name)
	fmt.Fprintf(&b, " total=%s", formatMillis(total))
	for _, f := range t.fields {
		fmt.Fprintf(&b, " %s=%v", f.name, f.value)
	}
	for _, s := range t.stages {
		fmt.Fprintf(&b, " %s=%s", s.name, formatMillis(s.took))
	}
	return b.String()
}

// formatMillis renders a duration as whole milliseconds so every number on a
// summary line is directly comparable at a glance.
func formatMillis(d time.Duration) string {
	return fmt.Sprintf("%dms", d.Milliseconds())
}
