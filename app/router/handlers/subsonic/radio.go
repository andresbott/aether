// Internet Radio Station handlers — Subsonic 1.16.0.
//
// TODO: Subsonic spec restricts create/update/delete to admin users.
// Blocked on auth implementation (see TODO.md > Security > Authentication).
package subsonic

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/gorm"
)

const (
	radioCoverMaxBytes = 5 * 1024 * 1024 // 5 MB
	// Multipart parse memory: 1 MB kept in memory, rest spilled to a temp file.
	radioMultipartMemory = 1 * 1024 * 1024
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

	st, err := h.store.CreateInternetRadioStation(name, streamURL, homepageURL)
	if err != nil {
		writeError(w, 0, "internal error")
		return
	}
	if coverBytes != nil {
		path, err := h.writeRadioCover(st.ID, coverExt, coverBytes)
		if err != nil {
			writeError(w, 0, "internal error")
			return
		}
		if err := h.store.UpdateInternetRadioStationCoverPath(st.ID, path); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}
	writeResponse(w, nil)
}

func (h *Handler) updateInternetRadioStation(w http.ResponseWriter, r *http.Request) {
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

	switch {
	case coverBytes != nil:
		// Remove the old file if its extension differs from the new one.
		if existing.CoverPath != "" && filepath.Ext(existing.CoverPath) != "."+coverExt {
			_ = os.Remove(existing.CoverPath)
		}
		path, err := h.writeRadioCover(id, coverExt, coverBytes)
		if err != nil {
			writeError(w, 0, "internal error")
			return
		}
		if err := h.store.UpdateInternetRadioStationCoverPath(id, path); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	case r.Form.Get("coverClear") == "true":
		if existing.CoverPath != "" {
			_ = os.Remove(existing.CoverPath)
		}
		if err := h.store.UpdateInternetRadioStationCoverPath(id, ""); err != nil {
			writeError(w, 0, "internal error")
			return
		}
	}

	writeResponse(w, nil)
}

func (h *Handler) deleteInternetRadioStation(w http.ResponseWriter, r *http.Request) {
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
	if existing.CoverPath != "" {
		_ = os.Remove(existing.CoverPath)
	}
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
	defer f.Close()
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

// writeRadioCover writes data to {radioCoverDir}/<id>.<ext> atomically and
// returns the final path.
func (h *Handler) writeRadioCover(id uint, ext string, data []byte) (string, error) {
	if h.radioCoverDir == "" {
		return "", fmt.Errorf("radio cover dir not configured")
	}
	if err := os.MkdirAll(h.radioCoverDir, 0755); err != nil {
		return "", fmt.Errorf("create radio cover dir: %w", err)
	}
	final := filepath.Join(h.radioCoverDir, fmt.Sprintf("%d.%s", id, ext))
	tmp, err := os.CreateTemp(h.radioCoverDir, "cover-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp cover: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("write temp cover: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("close temp cover: %w", err)
	}
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("rename temp cover: %w", err)
	}
	return final, nil
}
