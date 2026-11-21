package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/fahrypratama17/EcoSnap-BE/internal/service"
)

const ticketID = "b0000000-0000-4000-8000-000000000001"
const secretURL = "postgresql://owner:s3cr3t-pw@db.example.test:5432/postgres"

type fakeStore struct {
	opened, updates int
	last            service.SupportOwnerUpdate
	err             error
}

func (f *fakeStore) OwnerTickets(context.Context, *string, *string, int) ([]entity.SupportTicketOwnerView, error) {
	return []entity.SupportTicketOwnerView{{UserUID: "u1"}}, f.err
}
func (f *fakeStore) OwnerUpdate(_ context.Context, in service.SupportOwnerUpdate) (entity.SupportTicketOwnerView, error) {
	f.updates++
	f.last = in
	return entity.SupportTicketOwnerView{UserUID: "u1"}, f.err
}

func exec(f *fakeStore, env string, args ...string) (string, error) {
	var out bytes.Buffer
	err := run(context.Background(), args, func(string) string { return env }, &out, func(context.Context, string) (service.SupportOwnerStore, func(), error) {
		f.opened++
		return f, func() {}, nil
	})
	return out.String(), err
}

func TestMutationsNeedConfirmAndValidInput(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--id", ticketID, "--status", "resolved"},
		{"reply", "--id", ticketID, "--message", "ok"},
		{"status", "--id", ticketID, "--confirm"},
		{"status", "--status", "resolved", "--confirm"},
		{"status", "--id", ticketID, "--status", "resolved", "--message", "x", "--confirm"},
		{"reply", "--id", ticketID, "--confirm"},
		{"reply", "--id", ticketID, "--message", "a", "--message-file", "f", "--confirm"},
		{"reply", "--id", ticketID, "--message", "ok", "--confirm", "extra"},
		{"list", "--confirm"},
		{"list", "--message", "x"},
		{"delete", "--id", ticketID, "--confirm"},
		{},
	} {
		f := &fakeStore{}
		if _, err := exec(f, secretURL, args...); err == nil || f.opened != 0 {
			t.Fatalf("%v must fail before opening the database: %v", args, err)
		}
	}
	f := &fakeStore{}
	if _, err := exec(f, "", "list"); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") || f.opened != 0 {
		t.Fatalf("missing DATABASE_URL: %v", err)
	}
	for _, tt := range []struct {
		name string
		args []string
		want error
	}{
		{"blank reply", []string{"reply", "--id", ticketID, "--message", "   ", "--confirm"}, service.ErrInvalidSupportReply},
		{"long reply", []string{"reply", "--id", ticketID, "--message", strings.Repeat("a", 4001), "--confirm"}, service.ErrInvalidSupportReply},
		{"bad status", []string{"status", "--id", ticketID, "--status", "closed", "--confirm"}, service.ErrInvalidSupportStatus},
		{"bad id", []string{"status", "--id", "zzz", "--status", "resolved", "--confirm"}, service.ErrSupportTicketNotFound},
	} {
		f := &fakeStore{}
		if _, err := exec(f, secretURL, tt.args...); err == nil || err.Error() != tt.want.Error() || f.updates != 0 {
			t.Fatalf("%s: %v updates=%d", tt.name, err, f.updates)
		}
	}
}

func TestReplyDoesNotChangeStatusUnlessAsked(t *testing.T) {
	f := &fakeStore{}
	if _, err := exec(f, secretURL, "reply", "--id", ticketID, "--message", " Sudah kami cek. ", "--confirm"); err != nil {
		t.Fatal(err)
	}
	if f.last.Status != nil || f.last.Response == nil || *f.last.Response != "Sudah kami cek." || f.last.ReplaceResponse {
		t.Fatalf("reply update: %+v", f.last)
	}
	if _, err := exec(f, secretURL, "reply", "--id", ticketID, "--message", "x", "--status", "in_progress", "--replace", "--confirm"); err != nil {
		t.Fatal(err)
	}
	if f.last.Status == nil || *f.last.Status != "in_progress" || !f.last.ReplaceResponse {
		t.Fatalf("explicit status: %+v", f.last)
	}
	out, err := exec(f, secretURL, "list", "--status", "open", "--limit", "5")
	if err != nil || !strings.Contains(out, `"user_uid": "u1"`) {
		t.Fatalf("list: %v %s", err, out)
	}
}

func TestErrorsRedactConnectionDetails(t *testing.T) {
	f := &fakeStore{err: errors.New("dial " + secretURL + " failed: password s3cr3t-pw; also postgres://x:y@h/db")}
	_, err := exec(f, secretURL, "status", "--id", ticketID, "--status", "resolved", "--confirm")
	if err == nil || strings.Contains(err.Error(), "s3cr3t-pw") || strings.Contains(err.Error(), "postgres") || strings.Contains(err.Error(), "db.example.test") {
		t.Fatalf("leaked: %v", err)
	}
	var out bytes.Buffer
	err = run(context.Background(), []string{"list"}, func(string) string { return secretURL }, &out, func(context.Context, string) (service.SupportOwnerStore, func(), error) {
		return nil, nil, errors.New("connect " + secretURL)
	})
	if err == nil || strings.Contains(err.Error(), "s3cr3t-pw") || strings.Contains(err.Error(), "example") {
		t.Fatalf("connect error leaked: %v", err)
	}
}
