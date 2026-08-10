// Package scheduler provides a lightweight cron-like job registry.
//
// Uses a ticker-based runner with optional interval schedules. For full cron
// expressions, see AddCron (5-field, UTC). No external cron dependency.
package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Func is invoked when a job fires.
type Func func(ctx context.Context) error

// Job is a named scheduled function.
type Job struct {
	Name string
	Fn   Func
	// Every runs on a fixed interval (ignored if Cron is set).
	Every time.Duration
	// Cron is a 5-field expression: min hour dom month dow (UTC).
	// Supports "*", "n", and "*/n" per field. Experimental subset.
	Cron string
}

// Scheduler holds registered jobs.
type Scheduler struct {
	mu   sync.Mutex
	jobs []Job
}

// New returns an empty Scheduler.
func New() *Scheduler {
	return &Scheduler{}
}

// Add registers a job.
func (s *Scheduler) Add(job Job) error {
	if job.Name == "" {
		return fmt.Errorf("scheduler: job name required")
	}
	if job.Fn == nil {
		return fmt.Errorf("scheduler: job %q has nil Fn", job.Name)
	}
	if job.Cron == "" && job.Every <= 0 {
		return fmt.Errorf("scheduler: job %q needs Every or Cron", job.Name)
	}
	if job.Cron != "" {
		if _, err := parseCron(job.Cron); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.jobs = append(s.jobs, job)
	s.mu.Unlock()
	return nil
}

// Run starts all jobs until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	s.mu.Lock()
	jobs := append([]Job(nil), s.jobs...)
	s.mu.Unlock()

	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		wg.Add(1)
		go func() {
			defer wg.Done()
			if job.Cron != "" {
				runCron(ctx, job)
				return
			}
			runEvery(ctx, job)
		}()
	}
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func runEvery(ctx context.Context, job Job) {
	t := time.NewTicker(job.Every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = job.Fn(ctx)
		}
	}
}

func runCron(ctx context.Context, job Job) {
	spec, err := parseCron(job.Cron)
	if err != nil {
		return
	}
	// Check once per minute.
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var lastMin int = -1
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			now = now.UTC()
			if now.Second() != 0 || now.Minute() == lastMin {
				continue
			}
			lastMin = now.Minute()
			if matchCron(spec, now) {
				_ = job.Fn(ctx)
			}
		}
	}
}

type cronSpec struct {
	min, hour, dom, month, dow fieldMatch
}

type fieldMatch struct {
	any   bool
	step  int
	value int // used when step==0 && !any
}

func parseCron(expr string) (cronSpec, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return cronSpec{}, fmt.Errorf("scheduler: cron needs 5 fields, got %q", expr)
	}
	var spec cronSpec
	var err error
	if spec.min, err = parseField(parts[0], 0, 59); err != nil {
		return cronSpec{}, err
	}
	if spec.hour, err = parseField(parts[1], 0, 23); err != nil {
		return cronSpec{}, err
	}
	if spec.dom, err = parseField(parts[2], 1, 31); err != nil {
		return cronSpec{}, err
	}
	if spec.month, err = parseField(parts[3], 1, 12); err != nil {
		return cronSpec{}, err
	}
	if spec.dow, err = parseField(parts[4], 0, 6); err != nil {
		return cronSpec{}, err
	}
	return spec, nil
}

func parseField(s string, min, max int) (fieldMatch, error) {
	if s == "*" {
		return fieldMatch{any: true}, nil
	}
	if strings.HasPrefix(s, "*/") {
		n, err := strconv.Atoi(s[2:])
		if err != nil || n <= 0 {
			return fieldMatch{}, fmt.Errorf("scheduler: bad step %q", s)
		}
		return fieldMatch{step: n}, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min || n > max {
		return fieldMatch{}, fmt.Errorf("scheduler: bad field %q", s)
	}
	return fieldMatch{value: n}, nil
}

func matchCron(spec cronSpec, t time.Time) bool {
	return matchField(spec.min, t.Minute()) &&
		matchField(spec.hour, t.Hour()) &&
		matchField(spec.dom, t.Day()) &&
		matchField(spec.month, int(t.Month())) &&
		matchField(spec.dow, int(t.Weekday()))
}

func matchField(f fieldMatch, v int) bool {
	if f.any {
		return true
	}
	if f.step > 0 {
		return v%f.step == 0
	}
	return f.value == v
}
