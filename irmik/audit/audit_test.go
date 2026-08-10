package audit

import (
	"context"
	"testing"
)

func TestMemory(t *testing.T) {
	m := &Memory{}
	err := Record(context.Background(), m, Event{
		Actor:    "admin",
		Action:   "user.delete",
		Resource: "user:1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ev := m.Snapshot()
	if len(ev) != 1 || ev[0].Action != "user.delete" {
		t.Fatalf("%+v", ev)
	}
}
