package argossmtp_test

import (
	"context"
	"net/smtp"
	"testing"

	argossmtp "github.com/jhonsferg/argos/integrations/smtp"
)

// BenchmarkSendMail_Baseline measures net/smtp.SendMail directly - the
// "without Argos" comparison point.
func BenchmarkSendMail_Baseline(b *testing.B) {
	addr := startFakeSMTPServer(b, happyPathScript)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := smtp.SendMail(addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSendMail_Instrumented measures the same operation through
// argossmtp.SendMail, documenting the allocation cost of the span+metric
// pipeline this package adds per call.
func BenchmarkSendMail_Instrumented(b *testing.B) {
	addr := startFakeSMTPServer(b, happyPathScript)
	ctx := context.Background()

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := argossmtp.SendMail(ctx, addr, nil, "a@example.com", []string{"b@example.com"}, []byte(testMessage)); err != nil {
			b.Fatal(err)
		}
	}
}
