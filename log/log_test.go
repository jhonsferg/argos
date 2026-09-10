package log

import (
	"context"
	"testing"

	"github.com/jhonsferg/argos/logging"
)

type recordingLogger struct {
	infos []string
}

func (r *recordingLogger) Debug(context.Context, string, ...logging.KV) {}
func (r *recordingLogger) Info(_ context.Context, msg string, _ ...logging.KV) {
	r.infos = append(r.infos, msg)
}
func (r *recordingLogger) Warn(context.Context, string, ...logging.KV)         {}
func (r *recordingLogger) Error(context.Context, string, error, ...logging.KV) {}
func (r *recordingLogger) With(...logging.KV) logging.Logger                   { return r }

func TestDefault_NoopBeforeSetDefault(t *testing.T) {
	// A fresh atomic.Pointer[logging.Logger] is populated by this package's
	// init(), so Default() must never be nil and must never panic even if
	// SetDefault was never called in this test binary.
	if Default() == nil {
		t.Fatal("Default() returned nil before SetDefault was ever called")
	}
	Debug(context.Background(), "msg")
	Info(context.Background(), "msg")
	Warn(context.Background(), "msg")
	Error(context.Background(), "msg", nil)
	_ = With(logging.F("k", "v"))
}

func TestSetDefault_RoutesPackageLevelCalls(t *testing.T) {
	rec := &recordingLogger{}
	SetDefault(rec)
	t.Cleanup(func() { SetDefault(noopLogger{}) })

	Info(context.Background(), "hello")

	if len(rec.infos) != 1 || rec.infos[0] != "hello" {
		t.Errorf("recorded infos = %v, want [hello]", rec.infos)
	}
}

func TestSetDefault_NilFallsBackToNoop(t *testing.T) {
	SetDefault(nil)
	t.Cleanup(func() { SetDefault(noopLogger{}) })

	if Default() == nil {
		t.Fatal("Default() returned nil after SetDefault(nil)")
	}
	// Must not panic.
	Info(context.Background(), "hello")
}
