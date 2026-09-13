package store

import (
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound is the store's not-found sentinel. Every store lookup that can
// miss returns this — never gorm.ErrRecordNotFound — so callers (handlers, the
// CLI) recognise a missing row without importing the ORM. The store is the only
// DB gateway; this is what keeps GORM from leaking past it.
var ErrNotFound = errors.New("record not found")

// notFound maps GORM's not-found sentinel onto ErrNotFound and leaves every
// other error untouched. Store methods run a query error through it before
// returning to a caller so the gorm sentinel never crosses the boundary.
func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
