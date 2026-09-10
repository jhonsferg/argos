package argossftp_test

import (
	"context"
	"testing"

	argossftp "github.com/jhonsferg/argos/integrations/sftp"
)

// TestOpsContext_RecordSpans drives the remaining wrapped operations
// (Mkdir/MkdirAll/Rename/Stat/Remove) through a realistic workflow against
// the in-memory SFTP server - none of these had any coverage before.
func TestOpsContext_RecordSpans(t *testing.T) {
	exp := setTracer(t)
	client := argossftp.Wrap(newInMemoryClient(t))
	ctx := context.Background()

	if err := client.MkdirContext(ctx, "/dir"); err != nil {
		t.Fatalf("MkdirContext: %v", err)
	}
	if err := client.MkdirAllContext(ctx, "/dir/nested/deeper"); err != nil {
		t.Fatalf("MkdirAllContext: %v", err)
	}
	if _, err := client.StatContext(ctx, "/dir"); err != nil {
		t.Fatalf("StatContext: %v", err)
	}
	if err := client.RenameContext(ctx, "/dir", "/renamed"); err != nil {
		t.Fatalf("RenameContext: %v", err)
	}
	if err := client.RemoveContext(ctx, "/renamed/nested/deeper"); err != nil {
		t.Fatalf("RemoveContext: %v", err)
	}

	spans := exp.GetSpans()
	wantNames := []string{"mkdir", "mkdir_all", "stat", "rename", "remove"}
	if len(spans) != len(wantNames) {
		t.Fatalf("expected %d spans, got %d", len(wantNames), len(spans))
	}
	for i, want := range wantNames {
		if spans[i].Name != want {
			t.Errorf("span[%d] name = %q, want %q", i, spans[i].Name, want)
		}
	}
}

func TestRenameContext_RecordsDestinationPathWhenEnabled(t *testing.T) {
	exp := setTracer(t)
	client := argossftp.Wrap(newInMemoryClient(t), argossftp.WithPathAttribute(true))
	ctx := context.Background()

	if err := client.MkdirContext(ctx, "/dir"); err != nil {
		t.Fatalf("MkdirContext: %v", err)
	}
	if err := client.RenameContext(ctx, "/dir", "/renamed"); err != nil {
		t.Fatalf("RenameContext: %v", err)
	}

	spans := exp.GetSpans()
	renameSpan := spans[len(spans)-1]
	var got string
	for _, kv := range renameSpan.Attributes {
		if string(kv.Key) == "sftp.destination_path" {
			got = kv.Value.AsString()
		}
	}
	if got != "/renamed" {
		t.Errorf("sftp.destination_path = %q, want %q", got, "/renamed")
	}
}
