// internal/scanner/config.go
package scanner

// AssetRekeyer is the capability the scanner needs to move an entity's stored
// images when its natural-identity key changes. It is satisfied by
// *assetstore.Store but the scanner does not import that package.
//
// It is optional: a nil rekeyer means no hook, no error, and scans keep working
// in any context that has no asset store (tests, CLI tools).
type AssetRekeyer interface {
	Rekey(kind, oldKey, newKey string) error
}

type Config struct {
	TagReadWorkers int
	AssetRekeyer   AssetRekeyer
}
