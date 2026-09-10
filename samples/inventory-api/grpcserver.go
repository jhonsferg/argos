package main

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	argos "github.com/jhonsferg/argos"
	"github.com/jhonsferg/argos/samples/inventory-api/inventorypb"
)

// InventoryServer implements inventorypb.InventoryServer over an ItemStore.
type InventoryServer struct {
	inventorypb.UnimplementedInventoryServer
	store *ItemStore
}

func NewInventoryServer(store *ItemStore) *InventoryServer {
	return &InventoryServer{store: store}
}

// ReserveStock decrements an item's stock. The reservation logic itself is
// wrapped in argos.TraceFunc, so it gets its own child span (and, on
// failure, an error log through core/log's global logger) without a
// hand-written tracer field on InventoryServer.
func (s *InventoryServer) ReserveStock(ctx context.Context, req *inventorypb.ReserveStockRequest) (*inventorypb.ReserveStockResponse, error) {
	if req.GetItemId() == "" || req.GetQuantity() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "item_id and a positive quantity are required")
	}

	var remaining int
	err := argos.TraceFunc(ctx, "inventory.ReserveStock", func(ctx context.Context) error {
		var reserveErr error
		remaining, reserveErr = s.store.ReserveStock(ctx, req.GetItemId(), int(req.GetQuantity()))
		return reserveErr
	})

	switch {
	case errors.Is(err, ErrItemNotFound):
		return nil, status.Error(codes.NotFound, "item not found")
	case errors.Is(err, ErrInsufficientStock):
		return nil, status.Error(codes.FailedPrecondition, "insufficient stock")
	case err != nil:
		return nil, status.Error(codes.Internal, "internal error")
	}

	return &inventorypb.ReserveStockResponse{
		ItemId:    req.GetItemId(),
		Remaining: int32(remaining),
	}, nil
}
