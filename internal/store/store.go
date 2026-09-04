package store

import (
	"context"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

// Transaction runs fn in a DB transaction that is not bound to any context. Use
// it for request- or scan-independent work (tests, startup); prefer
// TransactionContext anywhere a caller has a context to honour.
func (s *Store) Transaction(fn func(tx *Store) error) error {
	return s.TransactionContext(context.Background(), fn)
}

// TransactionContext runs fn in a DB transaction bound to ctx, so a cancelled
// ctx (an aborted scan, a disconnected client) aborts an in-flight statement
// instead of letting the transaction run to completion while holding its locks.
func (s *Store) TransactionContext(ctx context.Context, fn func(tx *Store) error) error {
	return s.db.WithContext(ctx).Transaction(func(gormTx *gorm.DB) error {
		return fn(&Store{db: gormTx})
	})
}
