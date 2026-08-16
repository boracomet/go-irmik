package auth

import (
	"context"
	"testing"
)

func TestStubProviderRejectsEmptyCode(t *testing.T) {
	if _, err := (&StubProvider{}).Exchange(context.Background(), ""); err == nil {
		t.Fatal("expected an empty OAuth code to be rejected")
	}
}
