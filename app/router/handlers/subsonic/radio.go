// Internet Radio Station handlers — Subsonic 1.16.0.
//
// The spec restricts create/update/delete to admin users; requireAdmin
// enforces it (error 50). Reads stay open to every authenticated user.
package subsonic

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/andresbott/aether/internal/assetkey"
	"github.com/andresbott/aether/internal/assetstore"
	"gorm.io/gorm"
)

const (
	radioCoverMaxBytes = 5 * 1024 * 1024 // 5 MB
	// Multipart parse memory: 1 MB kept in memory, rest spilled to a temp file.
	radioMultipartMemory = 1 * 1024 * 1024
	// Hard cap on the whole multipart request body (cover + form fields).
	maxRadioRequestBytes = radioCoverMaxBytes + radioMultipartMemory
)

func (h *Handler) getInternetRadioStations(w http.ResponseWriter, r *http.Request) {
	stations, err := h.store.GetInternetRadioStations()
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	items := make([]map[string]any, 0, len(stations))
	for _, st := range stations {
		entry := map[string]any{
			"id":        encodeRadioID(st.ID),
			"name":      st.Name,
			"streamUrl": st.StreamURL,
			"coverArt":  encodeRadioID(st.ID),
		}
		if st.HomepageURL != "" {
			entry["homepageUrl"] = st.HomepageURL
		}
		items = append(items, entry)
	}
	writeResponse(w, map[string]any{
		"internetRadioStations": map[string]any{
			"internetRadioStation": items,
		},
	})
}

func (h *Handler) createInternetRadioStation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if isMultipart(r) {
		h.createRadioMultipart(w, r)
		return
	}
	h.createRadioQueryString(w, r)
}

func (h *Handler) createRadioQueryString(w http.ResponseWriter, r *http.Request) {
	name := paramStr(r, "name")
	if name == "" {
		writeError(w, 10, "missing name parameter")
		return
	}
	streamURL := paramStr(r, "streamUrl")
	if streamURL == "" {
		writeError(w, 10, "missing streamUrl parameter")
		return
	}
	homepageURL := paramStr(r, "homepageUrl")
	if _, err := h.store.CreateInternetRadioStation(name, streamURL, homepageURL); err != nil {
		writeError(w, 0, "internal error")
		return
	}
	writeResponse(w, nil)
}

func (h *Handler) createRadioMultipart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRadioRequestBytes)
	if err := r.ParseMultipartForm(radioMultipartMemory); err != nil {
		writeError(w, 0, "invalid multipart body")
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		writeError(w, 10, "missing name parameter")
		return
	}
	streamURL := r.Form.Get("streamUrl")
	if streamURL == "" {
		writeError(w, 10, "missing streamUrl parameter")
		return
	}
	homepageURL := r.Form.Get("homepageUrl")

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}

	if _, err := h.store.CreateInternetRadioStation(name, streamURL, homepageURL); err != nil {
		writeError(w, 0, "internal error")
		return
	}
	if coverBytes != nil {
		if err := h.assets.PutManual(assetstore.KindRadio, assetkey.Radio(streamURL), coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	writeResponse(w, nil)
}

func (h *Handler) updateInternetRadioStation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if isMultipart(r) {
		h.updateRadioMultipart(w, r)
		return
	}
	h.updateRadioQueryString(w, r)
}

func (h *Handler) updateRadioQueryString(w http.ResponseWriter, r *http.Request) {
	idStr := paramStr(r, "id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	kind, id, err := decodeID(idStr)
	if err != nil || kind != "radio" {
		writeError(w, 0, "invalid id")
		return
	}
	name := paramStr(r, "name")
	if name == "" {
		writeError(w, 10, "missing name parameter")
		return
	}
	streamURL := paramStr(r, "streamUrl")
	if streamURL == "" {
		writeError(w, 10, "missing streamUrl parameter")
		return
	}
	homepageURL := paramStr(r, "homepageUrl")
	if err := h.store.UpdateInternetRadioStation(id, name, streamURL, homepageURL); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, 70, "radio station not found")
			return
		}
		writeError(w, 0, "internal error")
		return
	}
	writeResponse(w, nil)
}

func (h *Handler) updateRadioMultipart(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRadioRequestBytes)
	if err := r.ParseMultipartForm(radioMultipartMemory); err != nil {
		writeError(w, 0, "invalid multipart body")
		return
	}
	idStr := r.Form.Get("id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	kind, id, err := decodeID(idStr)
	if err != nil || kind != "radio" {
		writeError(w, 0, "invalid id")
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		writeError(w, 10, "missing name parameter")
		return
	}
	streamURL := r.Form.Get("streamUrl")
	if streamURL == "" {
		writeError(w, 10, "missing streamUrl parameter")
		return
	}
	homepageURL := r.Form.Get("homepageUrl")

	existing, err := h.store.GetInternetRadioStation(id)
	if err != nil {
		writeError(w, 70, "radio station not found")
		return
	}

	coverBytes, coverExt, err := readCoverFile(r)
	if err != nil {
		writeError(w, 0, err.Error())
		return
	}

	if err := h.store.UpdateInternetRadioStation(id, name, streamURL, homepageURL); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, 70, "radio station not found")
			return
		}
		writeError(w, 0, "internal error")
		return
	}

	oldKey := assetkey.Radio(existing.StreamURL)
	newKey := assetkey.Radio(streamURL)
	switch {
	case coverBytes != nil:
		if err := h.assets.PutManual(assetstore.KindRadio, newKey, coverExt, coverBytes); err != nil {
			writeError(w, 0, "internal error")
			return
		}
		if oldKey != newKey {
			_ = h.assets.Delete(assetstore.KindRadio, oldKey)
		}
	case r.Form.Get("coverClear") == "true":
		_ = h.assets.Delete(assetstore.KindRadio, newKey)
		if oldKey != newKey {
			_ = h.assets.Delete(assetstore.KindRadio, oldKey)
		}
	default:
		// URL changed with no cover change: re-key the existing cover so it
		// isn't orphaned. Rekey is a lossless directory rename that preserves
		// named entries and auto variants.
		if oldKey != newKey {
			if err := h.assets.Rekey(assetstore.KindRadio, oldKey, newKey); err != nil {
				// The station's own update already succeeded, so both
				// ErrKeyOccupied and hard failures are logged but don't fail
				// the request — the edit committed, and the images stay intact.
				slog.Warn("radio cover re-key failed",
					"old_url", existing.StreamURL, "new_url", streamURL, "error", err)
			}
		}
	}

	writeResponse(w, nil)
}

func (h *Handler) deleteInternetRadioStation(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	idStr := paramStr(r, "id")
	if idStr == "" {
		writeError(w, 10, "missing id parameter")
		return
	}
	kind, id, err := decodeID(idStr)
	if err != nil || kind != "radio" {
		writeError(w, 0, "invalid id")
		return
	}
	existing, err := h.store.GetInternetRadioStation(id)
	if err != nil {
		writeError(w, 70, "radio station not found")
		return
	}
	if err := h.store.DeleteInternetRadioStation(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeError(w, 70, "radio station not found")
			return
		}
		writeError(w, 0, "internal error")
		return
	}
	_ = h.assets.Delete(assetstore.KindRadio, assetkey.Radio(existing.StreamURL))
	writeResponse(w, nil)
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
}

// readCoverFile pulls the optional "coverFile" part from a parsed multipart
// form and returns (bytes, "png"|"jpg", nil) on success, (nil, "", nil) when
// no file is present, or (nil, "", err) on validation failure.
func readCoverFile(r *http.Request) ([]byte, string, error) {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, "", nil
	}
	parts := r.MultipartForm.File["coverFile"]
	if len(parts) == 0 {
		return nil, "", nil
	}
	fh := parts[0]
	if fh.Size > radioCoverMaxBytes {
		return nil, "", fmt.Errorf("cover file too large (max %d bytes)", radioCoverMaxBytes)
	}
	f, err := fh.Open()
	if err != nil {
		return nil, "", fmt.Errorf("read cover file")
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, radioCoverMaxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read cover file")
	}
	if int64(len(data)) > radioCoverMaxBytes {
		return nil, "", fmt.Errorf("cover file too large (max %d bytes)", radioCoverMaxBytes)
	}
	sniff := data
	if len(sniff) > 512 {
		sniff = sniff[:512]
	}
	ct := http.DetectContentType(sniff)
	switch ct {
	case "image/png":
		return data, "png", nil
	case "image/jpeg":
		return data, "jpg", nil
	default:
		return nil, "", fmt.Errorf("unsupported image format (%s)", ct)
	}
}
