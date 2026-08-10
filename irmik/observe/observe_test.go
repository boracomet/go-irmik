package observe

import (
	"context"
	"log/slog"
	"testing"
)

func TestContextLogger(t *testing.T) {
	l := NewLogger(Options{Service: "test", JSON: true})
	ctx := WithLogger(context.Background(), l)
	got := FromContext(ctx)
	if got != l {
		t.Fatal("logger mismatch")
	}
	if FromContext(context.Background()) != slog.Default() {
		t.Fatal("expected default")
	}
}
