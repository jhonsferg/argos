package store

import (
	"context"
	"errors"
	"testing"
)

func TestStore_CreateAndGet(t *testing.T) {
	s := New()

	created, err := s.Create(context.Background(), "ana", "ana@example.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a generated ID")
	}

	got, err := s.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != created {
		t.Errorf("Get returned %+v, want %+v", got, created)
	}
}

func TestStore_GetMissing(t *testing.T) {
	s := New()

	if _, err := s.Get(context.Background(), "does-not-exist"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get error = %v, want %v", err, ErrNotFound)
	}
}
