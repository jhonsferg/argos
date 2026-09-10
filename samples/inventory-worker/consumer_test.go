package main

import (
	"context"
	"errors"
	"testing"
)

type fakeAuditRecorder struct {
	recorded []StockChanged
	err      error
}

func (f *fakeAuditRecorder) Record(_ context.Context, evt StockChanged) error {
	if f.err != nil {
		return f.err
	}
	f.recorded = append(f.recorded, evt)
	return nil
}

type fakeAnalyticsAppender struct {
	appended []StockChanged
	err      error
}

func (f *fakeAnalyticsAppender) Append(_ context.Context, evt StockChanged) error {
	if f.err != nil {
		return f.err
	}
	f.appended = append(f.appended, evt)
	return nil
}

func TestEventProcessor_Process_RecordsBoth(t *testing.T) {
	audit := &fakeAuditRecorder{}
	analytics := &fakeAnalyticsAppender{}
	p := NewEventProcessor(audit, analytics)

	evt := StockChanged{ItemID: "widget", Remaining: 95}
	if err := p.Process(context.Background(), evt); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(audit.recorded) != 1 || audit.recorded[0] != evt {
		t.Errorf("audit.recorded = %+v, want [%+v]", audit.recorded, evt)
	}
	if len(analytics.appended) != 1 || analytics.appended[0] != evt {
		t.Errorf("analytics.appended = %+v, want [%+v]", analytics.appended, evt)
	}
}

func TestEventProcessor_Process_StopsOnAuditFailure(t *testing.T) {
	audit := &fakeAuditRecorder{err: errors.New("mongo down")}
	analytics := &fakeAnalyticsAppender{}
	p := NewEventProcessor(audit, analytics)

	if err := p.Process(context.Background(), StockChanged{ItemID: "widget"}); err == nil {
		t.Fatal("expected an error when the audit store fails")
	}
	if len(analytics.appended) != 0 {
		t.Error("analytics should not be appended when the audit write fails first")
	}
}

func TestEventProcessor_Process_PropagatesAnalyticsFailure(t *testing.T) {
	audit := &fakeAuditRecorder{}
	analytics := &fakeAnalyticsAppender{err: errors.New("cassandra down")}
	p := NewEventProcessor(audit, analytics)

	if err := p.Process(context.Background(), StockChanged{ItemID: "widget"}); err == nil {
		t.Fatal("expected an error when the analytics store fails")
	}
	if len(audit.recorded) != 1 {
		t.Error("expected the audit record to still have been written before the analytics failure")
	}
}
