// Package server implements userspb.UsersServer over internal/store.
package server

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jhonsferg/argos/samples/users-service/internal/store"
	"github.com/jhonsferg/argos/samples/users-service/userspb"
)

type Server struct {
	userspb.UnimplementedUsersServer
	store *store.Store
}

func New(s *store.Store) *Server {
	return &Server{store: s}
}

func (s *Server) GetUser(ctx context.Context, req *userspb.GetUserRequest) (*userspb.User, error) {
	u, err := s.store.Get(ctx, req.GetId())
	if errors.Is(err, store.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &userspb.User{Id: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (s *Server) CreateUser(ctx context.Context, req *userspb.CreateUserRequest) (*userspb.User, error) {
	if req.GetName() == "" || req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "name and email are required")
	}
	u, err := s.store.Create(ctx, req.GetName(), req.GetEmail())
	if err != nil {
		return nil, status.Error(codes.Internal, "internal error")
	}
	return &userspb.User{Id: u.ID, Name: u.Name, Email: u.Email}, nil
}
