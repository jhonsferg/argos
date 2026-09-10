package argossmtp_test

import (
	"context"
	"net"
	"net/textproto"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	argossmtp "github.com/jhonsferg/argos/integrations/smtp"
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

// startFakeSMTPServer starts a minimal SMTP server that plays back the given
// newline-separated server responses on every connection it accepts, until
// the test ends. This is the same scripted-conversation technique net/smtp's
// own tests use (see GOROOT src/net/smtp/smtp_test.go, TestSendMail), made to
// serve more than one connection so it works for benchmarks too.
func startFakeSMTPServer(t testing.TB, script string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go serveScript(conn, script)
		}
	}()
	return l.Addr().String()
}

func serveScript(conn net.Conn, script string) {
	defer func() { _ = conn.Close() }()

	lines := strings.Split(strings.TrimRight(script, "\n"), "\n")
	tc := textproto.NewConn(conn)
	for i := 0; i < len(lines); i++ {
		_ = tc.PrintfLine("%s", lines[i])
		if lines[i] == "221 Goodbye" {
			return
		}
		for {
			msg, err := tc.ReadLine()
			if err != nil {
				return
			}
			if lines[i] == "354 Go ahead" && msg != "." {
				continue
			}
			break
		}
	}
}

const happyPathScript = `220 hello world
502 EH?
250 mx.example.com at your service
250 Sender ok
250 Receiver ok
354 Go ahead
250 Data ok
221 Goodbye
`

const rejectedRecipientScript = `220 hello world
502 EH?
250 mx.example.com at your service
250 Sender ok
550 Receiver rejected
`

const testMessage = "From: a@example.com\r\nTo: b@example.com\r\nSubject: hi\r\n\r\nhello\r\n"

func TestSendMail_RecordsSpan(t *testing.T) {
	exp := setTracer(t)
	addr := startFakeSMTPServer(t, happyPathScript)

	err := argossmtp.SendMail(context.Background(), addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Name != "send" {
		t.Errorf("span name = %q, want %q", spans[0].Name, "send")
	}
	if spans[0].Status.Code == codes.Error {
		t.Errorf("expected an OK span, got error status: %s", spans[0].Status.Description)
	}
}

func TestSendMail_RecordsErrorOnRejection(t *testing.T) {
	exp := setTracer(t)
	addr := startFakeSMTPServer(t, rejectedRecipientScript)

	err := argossmtp.SendMail(context.Background(), addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage))
	if err == nil {
		t.Fatal("expected an error for a rejected recipient")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Status.Code != codes.Error {
		t.Errorf("span status = %v, want %v", spans[0].Status.Code, codes.Error)
	}
}

func TestRecipientsOffByDefault(t *testing.T) {
	exp := setTracer(t)
	addr := startFakeSMTPServer(t, happyPathScript)

	if err := argossmtp.SendMail(context.Background(), addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage)); err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "smtp.recipients" {
			t.Error("smtp.recipients must not be recorded unless WithRecipients(true) is set")
		}
	}
}

func TestRecipientsOptIn(t *testing.T) {
	exp := setTracer(t)
	addr := startFakeSMTPServer(t, happyPathScript)

	err := argossmtp.SendMail(context.Background(), addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage), argossmtp.WithRecipients(true))
	if err != nil {
		t.Fatalf("SendMail: %v", err)
	}

	var found bool
	for _, kv := range exp.GetSpans()[0].Attributes {
		if string(kv.Key) == "smtp.recipients" {
			found = true
		}
	}
	if !found {
		t.Error("expected smtp.recipients attribute when WithRecipients(true) is set")
	}
}
