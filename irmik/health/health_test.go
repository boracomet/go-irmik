package health_test

import (
	"context"
	"errors"
	"testing"

	"github.com/boracomet/go-irmik/irmik/health"
)

func TestRegistryReady(t *testing.T) {
	r := health.New()
	r.Register("ok", func(ctx context.Context) error { return nil })
	r.Register("db", func(ctx context.Context) error { return errors.New("down") })

	ok, results := r.Evaluate(context.Background())
	if ok {
		t.Fatal("expected not ready")
	}
	if len(results) != 2 {
		t.Fatalf("results=%d", len(results))
	}
	var dbRes *health.Result
	for i := range results {
		if results[i].Name == "db" {
			dbRes = &results[i]
		}
	}
	if dbRes == nil || dbRes.OK || dbRes.Error == "" {
		t.Fatalf("db result=%+v", dbRes)
	}
}

func TestOptionalDoesNotFail(t *testing.T) {
	r := health.New()
	r.Register("core", func(ctx context.Context) error { return nil })
	r.RegisterOptional("cache", func(ctx context.Context) error { return errors.New("miss") })
	if !r.Ready(context.Background()) {
		t.Fatal("optional failure should not fail ready")
	}
}
