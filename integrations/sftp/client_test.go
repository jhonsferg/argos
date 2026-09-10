package argossftp_test

import (
	"context"
	"io"
	"testing"

	"github.com/pkg/sftp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argossftp "github.com/jhonsferg/argos/integrations/sftp"
)

func setTracer(t testing.TB) *tracetest.InMemoryExporter {
	t.Helper()
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return exp
}

// serverRWC combines the read/write halves of two io.Pipes into the single
// io.ReadWriteCloser sftp.NewRequestServer requires.
type serverRWC struct {
	io.Reader
	io.Writer
	closers []io.Closer
}

func (rwc serverRWC) Close() error {
	var err error
	for _, c := range rwc.closers {
		if cerr := c.Close(); err == nil {
			err = cerr
		}
	}
	return err
}

// newInMemoryClient wires a real *sftp.Client to a real sftp.RequestServer
// backed by sftp.InMemHandler() - an in-memory, non-OS-touching fake
// filesystem built into the library for exactly this - over in-process
// pipes. No network, no Docker, and nothing on the real filesystem is ever
// touched.
func newInMemoryClient(t testing.TB) *sftp.Client {
	t.Helper()

	clientToServerR, clientToServerW := io.Pipe()
	serverToClientR, serverToClientW := io.Pipe()

	server := sftp.NewRequestServer(
		serverRWC{Reader: clientToServerR, Writer: serverToClientW, closers: []io.Closer{clientToServerR, serverToClientW}},
		sftp.InMemHandler(),
	)
	go func() { _ = server.Serve() }()

	client, err := sftp.NewClientPipe(serverToClientR, clientToServerW)
	if err != nil {
		t.Fatalf("NewClientPipe: %v", err)
	}
	// Cleanup order matters here (t.Cleanup runs LIFO): the client's Close
	// blocks waiting for its receive goroutine to exit, which only happens
	// once the server side closes its end of the pipes. Registering the
	// client's cleanup before the server's means the server closes first.
	t.Cleanup(func() { _ = client.Close() })
	t.Cleanup(func() { _ = server.Close() })
	return client
}

func TestOpenContext_RecordsSpan(t *testing.T) {
	exp := setTracer(t)
	client := argossftp.Wrap(newInMemoryClient(t))

	f, err := client.CreateContext(context.Background(), "/greeting.txt")
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	_ = f.Close()

	opened, err := client.OpenContext(context.Background(), "/greeting.txt")
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

func TestOpenContext_RecordsErrorOnMissingFile(t *testing.T) {
	exp := setTracer(t)
	client := argossftp.Wrap(newInMemoryClient(t))

	if _, err := client.OpenContext(context.Background(), "/does-not-exist.txt"); err == nil {
		t.Fatal("expected an error for a missing file")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}

func TestPathAttributeOffByDefault(t *testing.T) {
	exp := setTracer(t)
	client := argossftp.Wrap(newInMemoryClient(t))

	f, err := client.CreateContext(context.Background(), "/path.txt")
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	_ = f.Close()

	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "sftp.path" {
			t.Error("sftp.path must not be recorded unless WithPathAttribute(true) is set")
		}
	}
}

func TestPathAttributeOptIn(t *testing.T) {
	exp := setTracer(t)
	client := argossftp.Wrap(newInMemoryClient(t), argossftp.WithPathAttribute(true))

	f, err := client.CreateContext(context.Background(), "/path.txt")
	if err != nil {
		t.Fatalf("CreateContext: %v", err)
	}
	_ = f.Close()

	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "sftp.path" && kv.Value.AsString() == "/path.txt" {
			found = true
		}
	}
	if !found {
		t.Error("expected sftp.path attribute when WithPathAttribute(true) is set")
	}
}
