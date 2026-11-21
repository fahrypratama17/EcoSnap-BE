package service

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCleanSupportText(t *testing.T) {
	for _, tt := range []struct {
		name, in  string
		max       int
		multiline bool
		want      string
		ok        bool
	}{
		{"trims unicode space", "\u3000 halo \u00a0", 150, false, "halo", true},
		{"normalises newlines", "a\r\nb", 10, true, "a\nb", true},
		{"counts characters not bytes", strings.Repeat("あ", 150), 150, false, strings.Repeat("あ", 150), true},
		{"one over", strings.Repeat("あ", 151), 150, false, "", false},
		{"blank", " \t\n", 150, true, "", false},
		{"newline rejected in subject", "a\nb", 150, false, "", false},
		{"nul rejected", "a\x00b", 150, true, "", false},
		{"invalid utf8", "a\xffb", 150, true, "", false},
	} {
		got, ok := cleanSupportText(tt.in, tt.max, tt.multiline)
		if ok != tt.ok || got != tt.want {
			t.Fatalf("%s: %q %v", tt.name, got, ok)
		}
	}
}

func TestSupportTransitions(t *testing.T) {
	for _, tt := range []struct {
		from, to string
		want     error
	}{
		{"open", "in_progress", nil}, {"open", "resolved", nil}, {"in_progress", "resolved", nil}, {"resolved", "in_progress", nil},
		{"open", "open", ErrInvalidSupportTransit}, {"in_progress", "open", ErrInvalidSupportTransit}, {"resolved", "open", ErrInvalidSupportTransit},
		{"open", "closed", ErrInvalidSupportStatus},
	} {
		if err := CheckSupportTransition(tt.from, tt.to); !errors.Is(err, tt.want) {
			t.Fatalf("%s->%s: %v", tt.from, tt.to, err)
		}
	}
}

func TestSupportOwnerValidationPrecedesStore(t *testing.T) {
	s := NewSupportOwnerService(nil) // a nil store panics if validation lets a call through
	blank, ok, bad := "  ", "ok", "closed"
	const id = "b0000000-0000-4000-8000-000000000001"
	for _, tt := range []struct {
		name, id string
		status   *string
		response *string
		want     error
	}{
		{"bad id", "x", &ok, nil, ErrSupportTicketNotFound},
		{"nothing", id, nil, nil, ErrSupportNothingToChange},
		{"bad status", id, &bad, nil, ErrInvalidSupportStatus},
		{"blank reply", id, nil, &blank, ErrInvalidSupportReply},
	} {
		if _, err := s.Update(context.Background(), tt.id, tt.status, tt.response, false); !errors.Is(err, tt.want) {
			t.Fatalf("%s: %v", tt.name, err)
		}
	}
	if _, err := s.List(context.Background(), "", "", 101); !errors.Is(err, ErrInvalidHistoryQuery) {
		t.Fatalf("limit: %v", err)
	}
}
