package store

import "gorm.io/gorm"

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) Transaction(fn func(tx *Store) error) error {
	return s.db.Transaction(func(gormTx *gorm.DB) error {
		return fn(&Store{db: gormTx})
	})
}
