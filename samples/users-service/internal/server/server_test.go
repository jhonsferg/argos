package server

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhonsferg/argos/samples/users-service/internal/store"
	"github.com/jhonsferg/argos/samples/users-service/userspb"
)

func TestServer_CreateAndGetUser(t *testing.T) {
	s := New(store.New())
	ctx := context.Background()

	created, err := s.CreateUser(ctx, &userspb.CreateUserRequest{Name: "ana", Email: "ana@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.GetId() == "" {
		t.Fatal("expected a generated ID")
	}

	got, err := s.GetUser(ctx, &userspb.GetUserRequest{Id: created.GetId()})
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.GetName() != "ana" || got.GetEmail() != "ana@example.com" {
		t.Errorf("unexpected user: %+v", got)
	}
}

func TestServer_CreateUser_MissingFields(t *testing.T) {
	s := New(store.New())

	_, err := s.CreateUser(context.Background(), &userspb.CreateUserRequest{Name: "ana"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("CreateUser error code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}

func TestServer_GetUser_NotFound(t *testing.T) {
	s := New(store.New())

	_, err := s.GetUser(context.Background(), &userspb.GetUserRequest{Id: "missing"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("GetUser error code = %v, want %v", status.Code(err), codes.NotFound)
	}
}
