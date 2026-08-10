// Package scheduler provides an opt-in job registry with fixed intervals and
// timezone-aware cron (robfig/cron/v3). Import only when you need background jobs.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// Func is invoked when a job fires.
type Func func(ctx context.Context) error

// Job is a named scheduled function.
type Job struct {
	Name string
	Fn   Func
	// Every runs on a fixed interval (ignored if Cron is set).
	Every time.Duration
	// Cron is a cron expression (5-field standard, or robfig descriptors like @hourly).
	// Evaluated in Location / LocationName (default UTC).
	Cron string
	// Location for cron evaluation. Takes precedence over LocationName.
	Location *time.Location
	// LocationName is an IANA timezone (e.g. "Europe/Istanbul") when Location is nil.
	LocationName string
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
		loc, err := resolveLocation(job.Location, job.LocationName)
		if err != nil {
			return err
		}
		if _, err := parseSchedule(job.Cron); err != nil {
			return fmt.Errorf("scheduler: job %q cron: %w", job.Name, err)
		}
		job.Location = loc
	}
	s.mu.Lock()
	s.jobs = append(s.jobs, job)
	s.mu.Unlock()
	return nil
}

// AddCron registers a cron job with an explicit timezone location.
// loc may be nil (UTC). Prefer this over setting Job.Location manually.
func (s *Scheduler) AddCron(name, expr string, loc *time.Location, fn Func) error {
	return s.Add(Job{Name: name, Cron: expr, Location: loc, Fn: fn})
}

// AddCronTZ registers a cron job using an IANA timezone name (e.g. "America/New_York").
// Empty tz means UTC.
func (s *Scheduler) AddCronTZ(name, expr, tz string, fn Func) error {
	return s.Add(Job{Name: name, Cron: expr, LocationName: tz, Fn: fn})
}

// Jobs returns a snapshot of registered jobs (for tests/introspection).
func (s *Scheduler) Jobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Job, len(s.jobs))
	copy(out, s.jobs)
	return out
}

// NextFire returns the next activation time for expr after t in loc.
// loc may be nil (UTC). Useful for tests and admin UIs.
func NextFire(expr string, loc *time.Location, after time.Time) (time.Time, error) {
	loc, err := resolveLocation(loc, "")
	if err != nil {
		return time.Time{}, err
	}
	sched, err := parseSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	if after.IsZero() {
		after = time.Now()
	}
	return sched.Next(after.In(loc)), nil
}

// NextFireByName returns the next fire time for a registered cron job.
func (s *Scheduler) NextFireByName(name string, after time.Time) (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.Name != name {
			continue
		}
		if job.Cron == "" {
			return time.Time{}, fmt.Errorf("scheduler: job %q is not a cron job", name)
		}
		return NextFire(job.Cron, job.Location, after)
	}
	return time.Time{}, fmt.Errorf("scheduler: job %q not found", name)
}

// Run starts all jobs until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	s.mu.Lock()
	jobs := append([]Job(nil), s.jobs...)
	s.mu.Unlock()

	var wg sync.WaitGroup
	// Group cron jobs by location so each timezone gets one cron runner.
	type cronGroup struct {
		loc  *time.Location
		jobs []Job
	}
	groups := map[string]*cronGroup{}

	for _, job := range jobs {
		job := job
		if job.Cron == "" {
			wg.Add(1)
			go func() {
				defer wg.Done()
				runEvery(ctx, job)
			}()
			continue
		}
		loc := job.Location
		if loc == nil {
			loc = time.UTC
		}
		key := loc.String()
		g, ok := groups[key]
		if !ok {
			g = &cronGroup{loc: loc}
			groups[key] = g
		}
		g.jobs = append(g.jobs, job)
	}

	for _, g := range groups {
		g := g
		wg.Add(1)
		go func() {
			defer wg.Done()
			runCronGroup(ctx, g.loc, g.jobs)
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

func runCronGroup(ctx context.Context, loc *time.Location, jobs []Job) {
	c := cron.New(cron.WithLocation(loc), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	for _, job := range jobs {
		job := job
		_, err := c.AddFunc(job.Cron, func() {
			_ = job.Fn(ctx)
		})
		if err != nil {
			// Validated at Add; ignore unexpected parse errors at runtime.
			continue
		}
	}
	c.Start()
	defer func() {
		stopCtx := c.Stop()
		<-stopCtx.Done()
	}()
	<-ctx.Done()
}

func resolveLocation(loc *time.Location, name string) (*time.Location, error) {
	if loc != nil {
		return loc, nil
	}
	if name == "" {
		return time.UTC, nil
	}
	loaded, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("scheduler: timezone %q: %w", name, err)
	}
	return loaded, nil
}

func parseSchedule(expr string) (cron.Schedule, error) {
	sched, err := cron.ParseStandard(expr)
	if err != nil {
		return nil, err
	}
	return sched, nil
}
