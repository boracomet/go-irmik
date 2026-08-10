package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestEvery(t *testing.T) {
	s := New()
	var n atomic.Int32
	if err := s.Add(Job{
		Name:  "tick",
		Every: 20 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			n.Add(1)
			return nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_ = s.Run(ctx)
	if n.Load() < 2 {
		t.Fatalf("expected >=2 ticks, got %d", n.Load())
	}
}

func TestAddCronRegistration(t *testing.T) {
	s := New()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddCron("daily", "0 9 * * *", loc, func(ctx context.Context) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddCronTZ("hourly", "@hourly", "Europe/Istanbul", func(ctx context.Context) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	jobs := s.Jobs()
	if len(jobs) != 2 {
		t.Fatalf("jobs=%d", len(jobs))
	}
	if jobs[0].Location == nil || jobs[0].Location.String() != loc.String() {
		t.Fatalf("location=%v", jobs[0].Location)
	}
	if jobs[1].Location == nil || jobs[1].Location.String() == "" {
		t.Fatal("expected loaded Istanbul location")
	}
}

func TestAddCronBadTZ(t *testing.T) {
	s := New()
	err := s.Add(Job{
		Name:         "bad",
		Cron:         "0 * * * *",
		LocationName: "Not/AZone",
		Fn:           func(ctx context.Context) error { return nil },
	})
	if err == nil {
		t.Fatal("expected timezone error")
	}
}

func TestAddCronBadExpr(t *testing.T) {
	s := New()
	err := s.Add(Job{
		Name: "bad",
		Cron: "not-a-cron",
		Fn:   func(ctx context.Context) error { return nil },
	})
	if err == nil {
		t.Fatal("expected cron parse error")
	}
}

func TestNextFire(t *testing.T) {
	loc := time.UTC
	after := time.Date(2026, 8, 10, 8, 15, 0, 0, loc)
	next, err := NextFire("0 9 * * *", loc, after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 10, 9, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("next=%v want=%v", next, want)
	}

	// Same wall-clock in another zone.
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	afterNY := time.Date(2026, 8, 10, 8, 15, 0, 0, ny)
	nextNY, err := NextFire("0 9 * * *", ny, afterNY)
	if err != nil {
		t.Fatal(err)
	}
	wantNY := time.Date(2026, 8, 10, 9, 0, 0, 0, ny)
	if !nextNY.Equal(wantNY) {
		t.Fatalf("nextNY=%v want=%v", nextNY, wantNY)
	}
}

func TestNextFireByName(t *testing.T) {
	s := New()
	_ = s.AddCronTZ("cleanup", "30 2 * * *", "UTC", func(ctx context.Context) error {
		return nil
	})
	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	next, err := s.NextFireByName("cleanup", after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 1, 2, 30, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next=%v want=%v", next, want)
	}
}

func TestStepCronStillValid(t *testing.T) {
	after := time.Date(2026, 8, 10, 10, 7, 0, 0, time.UTC)
	next, err := NextFire("*/5 * * * *", time.UTC, after)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 10, 10, 10, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next=%v want=%v", next, want)
	}
}
