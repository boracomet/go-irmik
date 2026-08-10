// Package health provides named readiness dependency checks (DB, Redis, custom).
// Liveness (/health) stays dumb; wire checks into /ready via App.RegisterReadyCheck
// or middleware.HealthWith.
package health

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// CheckFunc probes a dependency. Return nil when healthy.
type CheckFunc func(ctx context.Context) error

// Check is a named readiness probe.
type Check struct {
	Name     string
	Fn       CheckFunc
	Required bool // when true, failure makes ready=false
	Timeout  time.Duration
}

// Result is one check outcome.
type Result struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Elapsed string `json:"elapsed,omitempty"`
}

// Registry holds named checks.
type Registry struct {
	mu     sync.Mutex
	checks []Check
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{}
}

// Register adds a required named check (fails ready on error).
func (r *Registry) Register(name string, fn CheckFunc) {
	r.RegisterCheck(Check{Name: name, Fn: fn, Required: true})
}

// RegisterOptional adds a check that is reported but does not fail ready.
func (r *Registry) RegisterOptional(name string, fn CheckFunc) {
	r.RegisterCheck(Check{Name: name, Fn: fn, Required: false})
}

// RegisterCheck appends a full Check.
func (r *Registry) RegisterCheck(c Check) {
	if c.Name == "" || c.Fn == nil {
		return
	}
	if c.Timeout <= 0 {
		c.Timeout = 2 * time.Second
	}
	r.mu.Lock()
	r.checks = append(r.checks, c)
	r.mu.Unlock()
}

// Evaluate runs all checks and returns overall readiness plus per-check results.
// Overall ready is false if any Required check fails.
func (r *Registry) Evaluate(ctx context.Context) (ready bool, results []Result) {
	r.mu.Lock()
	checks := append([]Check(nil), r.checks...)
	r.mu.Unlock()

	ready = true
	results = make([]Result, 0, len(checks))
	for _, ch := range checks {
		res := runOne(ctx, ch)
		results = append(results, res)
		if ch.Required && !res.OK {
			ready = false
		}
	}
	return ready, results
}

// Ready is a convenience boolean Evaluate.
func (r *Registry) Ready(ctx context.Context) bool {
	ok, _ := r.Evaluate(ctx)
	return ok
}

func runOne(parent context.Context, ch Check) Result {
	start := time.Now()
	ctx, cancel := context.WithTimeout(parent, ch.Timeout)
	defer cancel()
	err := ch.Fn(ctx)
	res := Result{
		Name:    ch.Name,
		OK:      err == nil,
		Elapsed: time.Since(start).String(),
	}
	if err != nil {
		res.Error = err.Error()
	}
	return res
}

// PingDB returns a CheckFunc that pings *sql.DB.
func PingDB(db *sql.DB) CheckFunc {
	return func(ctx context.Context) error {
		if db == nil {
			return fmt.Errorf("db is nil")
		}
		return db.PingContext(ctx)
	}
}

// PingFunc wraps a custom zero-arg or error-returning probe.
func PingFunc(fn func(context.Context) error) CheckFunc {
	return fn
}
