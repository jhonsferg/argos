package main

import (
	"context"

	"gorm.io/gorm"
)

// Item is the GORM model persisted in Postgres.
type Item struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	Name  string `json:"name"`
	Price int64  `json:"price_cents"`
}

type ItemStore struct {
	db *gorm.DB
}

func NewItemStore(db *gorm.DB) *ItemStore {
	return &ItemStore{db: db}
}

func (s *ItemStore) Migrate() error {
	return s.db.AutoMigrate(&Item{})
}

func (s *ItemStore) Create(ctx context.Context, item Item) (Item, error) {
	if err := s.db.WithContext(ctx).Create(&item).Error; err != nil {
		return Item{}, err
	}
	return item, nil
}

func (s *ItemStore) Get(ctx context.Context, id uint) (Item, error) {
	var item Item
	err := s.db.WithContext(ctx).First(&item, id).Error
	return item, err
}
