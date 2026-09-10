package main

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Shipment tracks a single order's shipment status. It's keyed by order_id
// (a natural key), so "create" and "update" are the same upsert operation -
// a shipment's status simply changes over its lifecycle.
type Shipment struct {
	OrderID   string    `bson:"_id" json:"order_id"`
	Status    string    `bson:"status" json:"status"`
	UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}

type ShipmentStore struct {
	coll *mongo.Collection
}

func NewShipmentStore(db *mongo.Database) *ShipmentStore {
	return &ShipmentStore{coll: db.Collection("shipments")}
}

func (s *ShipmentStore) Upsert(ctx context.Context, sh Shipment) (Shipment, error) {
	sh.UpdatedAt = time.Now().UTC()
	_, err := s.coll.UpdateOne(ctx,
		bson.M{"_id": sh.OrderID},
		bson.M{"$set": sh},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return Shipment{}, err
	}
	return sh, nil
}

func (s *ShipmentStore) Get(ctx context.Context, orderID string) (Shipment, error) {
	var sh Shipment
	err := s.coll.FindOne(ctx, bson.M{"_id": orderID}).Decode(&sh)
	return sh, err
}
