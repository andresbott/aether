package subsonic

import "net/http"

// getUser answers the OpenSubsonic getUser call for the authenticated caller.
// Clients use it at login time to discover their permissions (some refuse to
// start without it). Aether is a single-tier music server, so every role is a
// fixed constant except adminRole/settingsRole, which follow the same admin
// lookup that gates the radio writes (h.admin).
//
// The response always describes the resolved request owner (the real login),
// regardless of the username query parameter: a PAT-authenticated client knows
// itself by its token's virtual username and would pass that alias, so echoing
// it back — or rejecting the mismatch — would misreport or lock out that
// client. /rest never exposes another user's record (getUsers is intentionally
// unimplemented; user administration lives on /api/v1), so serving the caller's
// own record is the only supported behavior.
func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	owner := requestOwner(r)

	admin := true
	if h.admin != nil {
		ok, err := h.admin(owner)
		if err != nil {
			writeError(w, 0, "internal error")
			return
		}
		admin = ok
	}

	writeResponse(w, map[string]any{
		"user": map[string]any{
			"username":            owner,
			"scrobblingEnabled":   true,
			"adminRole":           admin,
			"settingsRole":        admin,
			"downloadRole":        true,
			"uploadRole":          false,
			"playlistRole":        true,
			"coverArtRole":        true,
			"commentRole":         false,
			"podcastRole":         false,
			"streamRole":          true,
			"jukeboxRole":         false,
			"shareRole":           false,
			"videoConversionRole": false,
		},
	})
}
