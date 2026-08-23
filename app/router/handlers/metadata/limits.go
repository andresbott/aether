package metadata

import "errors"

// maxSelectionPaths caps a request's paths[]: defense in depth against a
// pathological single request. For the picture-selection endpoints
// (inventory, raw-tags, removals, decoded via decodeSelection) this matters
// now that the selection travels in a request body instead of a query
// string — the body itself already removes the header-overflow risk this
// redesign fixes (see
// docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md);
// this cap only bounds the work/DoS surface a single request can trigger.
// For identify/identify-album it additionally bounds real cost: each path
// costs one fpcalc run (~1s of CPU) plus one rate-limited AcoustID call.
// One constant shared by every paths[]-accepting endpoint, so the bound
// cannot drift between them.
const maxSelectionPaths = 50

// maxSelectionBodyBytes caps the size of a picture-selection POST body
// (inventory, raw-tags, removals — decoded via decodeSelection), applied via
// http.MaxBytesReader before json.Decode ever runs: defense in depth against
// a pathologically large body, independent of the maxSelectionPaths count
// check above (a request could try to inflate its body some other way, e.g.
// absurdly long relpaths). A selection of at most maxSelectionPaths short
// relpaths is a few KB, so 1 MiB only rejects abuse, never a legitimate
// selection.
const maxSelectionBodyBytes = 1 << 20 // 1 MiB

// errTooManyPaths is returned when a request's paths[] exceeds
// maxSelectionPaths.
var errTooManyPaths = errors.New("too many paths in one request")

// errNoSelection is returned when a request's paths[] is empty.
var errNoSelection = errors.New("at least one path is required")

// errUnknownSlot is returned when a picture endpoint's slot is neither
// "embedded" nor "folder".
var errUnknownSlot = errors.New("slot must be one of embedded, folder")

// errUnknownType is returned when a picture endpoint's type is not in the
// registry (metadataedit.PictureTypeByID).
var errUnknownType = errors.New("unknown picture type")
