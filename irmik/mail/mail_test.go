package mail

import (
	"context"
	"strings"
	"testing"
)

func TestMemorySender(t *testing.T) {
	m := &Memory{}
	err := m.Send(context.Background(), Message{
		From:    "a@example.com",
		To:      []string{"b@example.com"},
		Subject: "Hi",
		Body:    "hello",
	})
	if err != nil || len(m.Messages) != 1 {
		t.Fatalf("err=%v n=%d", err, len(m.Messages))
	}
}

func TestBuildMessageHTML(t *testing.T) {
	raw, err := buildMessage("a@x.com", Message{
		To:      []string{"b@x.com"},
		Subject: "Sub\nject",
		Body:    "plain",
		HTML:    "<b>hi</b>",
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "\nject") && strings.Contains(s, "Subject: Sub\n") {
		t.Fatal("newline in subject not sanitized")
	}
	if !strings.Contains(s, "multipart/alternative") || !strings.Contains(s, "<b>hi</b>") {
		t.Fatalf("bad mime: %s", s)
	}
}
