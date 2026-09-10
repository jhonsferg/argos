// Package store is an in-memory user repository - no database needed to
// run this sample, since it demonstrates the gorilla/mux+gRPC wiring, not a
// persistence integration (those are covered by the other samples).
package store

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
)

type User struct {
	ID    string
	Name  string
	Email string
}

var ErrNotFound = errors.New("user not found")

type Store struct {
	mu    sync.Mutex
	users map[string]User
}

func New() *Store {
	return &Store{users: make(map[string]User)}
}

func (s *Store) Create(_ context.Context, name, email string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := User{ID: uuid.NewString(), Name: name, Email: email}
	s.users[u.ID] = u
	return u, nil
}

func (s *Store) Get(_ context.Context, id string) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}
