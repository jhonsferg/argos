//go:build integration

package argossftp_test

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/sftp"
	tcsftp "github.com/testcontainers/testcontainers-go/modules/sftp"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"golang.org/x/crypto/ssh"

	argossftp "github.com/jhonsferg/argos/integrations/sftp"
)

// TestClient_RealSFTPServer proves the wrapper against an actual SFTP
// server over real SSH, not just the in-memory pipe unit tests use.
func TestClient_RealSFTPServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := tcsftp.Run(ctx, "atmoz/sftp:latest", tcsftp.WithUser("alice", "secret"))
	if err != nil {
		t.Fatalf("start sftp container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Logf("terminate container: %v", err)
		}
	})

	addr, err := container.Address(ctx)
	if err != nil {
		t.Fatalf("Address: %v", err)
	}

	sshClient, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            "alice",
		Auth:            []ssh.AuthMethod{ssh.Password("secret")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec // test-only container, no real host to verify
		Timeout:         30 * time.Second,
	})
	if err != nil {
		t.Fatalf("ssh.Dial: %v", err)
	}
	defer func() { _ = sshClient.Close() }()

	rawClient, err := sftp.NewClient(sshClient)
	if err != nil {
		t.Fatalf("sftp.NewClient: %v", err)
	}
	defer func() { _ = rawClient.Close() }()

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })

	client := argossftp.Wrap(rawClient)

	f, err := client.CreateContext(ctx, "/upload/greeting.txt")
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	if _, err := f.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	opened, err := client.OpenContext(ctx, "/upload/greeting.txt")
	if err != nil {
		t.Fatalf("OpenContext: %v", err)
	}
	_ = opened.Close()

	spans := exp.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans (create, open), got %d", len(spans))
	}
	if spans[0].Name != "create" || spans[1].Name != "open" {
		t.Errorf("span names = %q, %q; want create, open", spans[0].Name, spans[1].Name)
	}
}
