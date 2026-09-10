package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/logging"
	"github.com/jhonsferg/argos/samples/users-service/userspb"
)

// fakeUsersClient implements userspb.UsersClient for testing the HTTP layer
// without a real gRPC backend.
type fakeUsersClient struct {
	users     map[string]*userspb.User
	createErr error
}

func newFakeUsersClient() *fakeUsersClient {
	return &fakeUsersClient{users: make(map[string]*userspb.User)}
}

func (f *fakeUsersClient) GetUser(_ context.Context, in *userspb.GetUserRequest, _ ...grpc.CallOption) (*userspb.User, error) {
	u, ok := f.users[in.GetId()]
	if !ok {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return u, nil
}

func (f *fakeUsersClient) CreateUser(_ context.Context, in *userspb.CreateUserRequest, _ ...grpc.CallOption) (*userspb.User, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	u := &userspb.User{Id: "u1", Name: in.GetName(), Email: in.GetEmail()}
	f.users[u.Id] = u
	return u, nil
}

func testLogger() argos.Logger {
	return logging.NewZerolog(bytes.NewBuffer(nil), 0, nil)
}

func TestHandleCreateUser(t *testing.T) {
	client := newFakeUsersClient()
	api := NewAPI(client, testLogger())

	body := `{"name":"ana","email":"ana@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var got userspb.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.GetName() != "ana" {
		t.Errorf("unexpected user: %+v", &got)
	}
}

func TestHandleGetUser_Found(t *testing.T) {
	client := newFakeUsersClient()
	client.users["u1"] = &userspb.User{Id: "u1", Name: "ana", Email: "ana@example.com"}
	api := NewAPI(client, testLogger())

	req := httptest.NewRequest(http.MethodGet, "/users/u1", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandleGetUser_NotFound(t *testing.T) {
	api := NewAPI(newFakeUsersClient(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/users/missing", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandleCreateUser_InvalidArgument(t *testing.T) {
	client := newFakeUsersClient()
	client.createErr = status.Error(codes.InvalidArgument, "name and email are required")
	api := NewAPI(client, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestHandleHealth(t *testing.T) {
	api := NewAPI(newFakeUsersClient(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	api.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
