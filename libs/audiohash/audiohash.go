// Package audiohash computes a metadata-invariant hash of an audio file's
// payload. It locates the encoded audio and ignores tag/metadata regions, so
// the hash stays stable when tags are edited and changes only when the audio
// itself does. That makes it a reliable signal for recognising the same track
// after a move or a retag, where filename and byte-count based heuristics fail.
//
// The package has no aether dependencies and is safe to use standalone.
//
// The returned hash is a self-describing, colon-prefixed string (for example
// "flacmd5:<hex>" or "fnv1a64:<hex>"). The prefix names the scheme so the value
// can be persisted and compared across scans and across versions; a future
// scheme change stays distinguishable rather than silently comparing unequal.
package audiohash

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsupported is returned by File for a format audiohash cannot fingerprint.
// Callers should treat it as "no hash available" and fall back to their other
// identity heuristics rather than as a failure.
var ErrUnsupported = errors.New("audiohash: unsupported format")

// maxHashBytes bounds how much of the audio payload the byte-hash formats read,
// so a full library scan stays I/O-cheap regardless of track length. Combined
// with the exact payload length (mixed into the digest) this is overwhelming
// evidence of identical audio. Changing it changes every emitted hash, so treat
// it as part of the on-disk format.
const maxHashBytes = 256 << 10

// File returns the metadata-invariant hash of the audio file at path. It is a
// convenience wrapper that opens the file and delegates to Reader.
func File(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: hashing the caller's file is the entire purpose of this function; the path is supplied by the caller, which in aether is a library file the scanner already admitted and opened to read tags.
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	return Reader(f, info.Size(), path)
}

// Reader returns the metadata-invariant hash of an already-open audio file,
// reusing the caller's handle instead of opening the path itself — so a scan
// that already has the file open (for tag reading, say) or a filepath walk that
// opens each entry avoids a redundant open and stat. size is the file's total
// size in bytes (for example fs.FileInfo.Size, or a walk entry's Info().Size());
// name is the file name or path, used only for its extension to pick the format,
// since some formats (notably a tagless MP3) carry no reliable leading magic.
func Reader(r io.ReaderAt, size int64, name string) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".flac":
		return flacHash(r)
	case ".mp3":
		return mp3Hash(r, size)
	case ".m4a", ".m4b", ".mp4":
		return mp4Hash(r, size)
	case ".wav", ".wave":
		return wavHash(r, size)
	case ".aif", ".aiff", ".aifc":
		return aiffHash(r, size)
	default:
		return "", ErrUnsupported
	}
}

// flacHash reads the STREAMINFO block that must open every FLAC stream and
// returns its stored MD5 of the decoded audio. Because the encoder computes
// that MD5 over the decoded samples, it is inherently independent of tags — and
// even survives re-compression at a different level.
func flacHash(r io.ReaderAt) (string, error) {
	const streamInfoSize = 34
	const prefix = 8 + streamInfoSize // "fLaC" + block header + STREAMINFO body
	buf := make([]byte, prefix)
	if _, err := r.ReadAt(buf, 0); err != nil {
		return "", fmt.Errorf("audiohash: read FLAC header: %w", err)
	}
	if string(buf[0:4]) != "fLaC" {
		return "", fmt.Errorf("audiohash: not a FLAC stream")
	}
	if blockType := buf[4] & 0x7f; blockType != 0 {
		return "", fmt.Errorf("audiohash: first FLAC block is type %d, not STREAMINFO", blockType)
	}
	if length := int(buf[5])<<16 | int(buf[6])<<8 | int(buf[7]); length < streamInfoSize {
		return "", fmt.Errorf("audiohash: STREAMINFO too short: %d bytes", length)
	}
	// The MD5 signature is the final 16 bytes of the 34-byte STREAMINFO body.
	md5 := buf[8+18 : prefix]
	// An all-zero signature is FLAC's "not computed" sentinel, not a real hash;
	// reporting it would collide every unsigned FLAC, so fall back instead.
	if bytes.Equal(md5, make([]byte, 16)) {
		return "", ErrUnsupported
	}
	return "flacmd5:" + hex.EncodeToString(md5), nil
}

// mp3Hash hashes the MPEG audio between an optional leading ID3v2 tag and any
// trailing ID3v1/APEv2 tags, so every tag region is excluded.
func mp3Hash(f io.ReaderAt, size int64) (string, error) {
	audioStart, err := id3v2Len(f)
	if err != nil {
		return "", err
	}
	return payloadHash(f, audioStart, mp3AudioEnd(f, audioStart, size))
}

// mp3AudioEnd returns the offset at which the audio payload ends — the start of
// any trailing metadata. It peels a fixed-size ID3v1 trailer and then an APEv2
// tag, in their on-disk order (ID3v1 is always last, APE sits just before it).
// floor is the audio start; nothing is peeled past it, so a corrupt tag length
// can never reach into the leading ID3v2 tag or before the file start.
func mp3AudioEnd(f io.ReaderAt, floor, size int64) int64 {
	end := size
	// ID3v1: a fixed 128-byte trailer identified by a "TAG" magic.
	if end-floor >= 128 {
		magic := make([]byte, 3)
		if _, err := f.ReadAt(magic, end-128); err == nil && string(magic) == "TAG" {
			end -= 128
		}
	}
	// APEv2 sits just before any ID3v1 trailer.
	end -= apeTagLen(f, end, floor)
	return end
}

// apeTagLen returns the length of the APEv2 tag ending at end, or 0 when there
// is none. Its 32-byte footer carries an "APETAGEX" preamble and a declared size
// covering the items plus that footer; a flag marks an optional 32-byte header
// sitting on top of the tag. Nothing is reported past floor, so a corrupt length
// can never reach into the leading ID3v2 tag or before the start of the file.
func apeTagLen(f io.ReaderAt, end, floor int64) int64 {
	if end-floor < 32 {
		return 0
	}
	foot := make([]byte, 32)
	if _, err := f.ReadAt(foot, end-32); err != nil {
		return 0
	}
	if string(foot[:8]) != "APETAGEX" {
		return 0
	}
	total := int64(binary.LittleEndian.Uint32(foot[12:16]))
	if flags := binary.LittleEndian.Uint32(foot[20:24]); flags&0x80000000 != 0 {
		total += 32 // header present
	}
	if total < 32 || end-floor < total {
		return 0
	}
	return total
}

// id3v2Len returns the total byte length of a leading ID3v2 tag, or 0 when the
// file does not start with one. The audio payload begins right after it.
func id3v2Len(f io.ReaderAt) (int64, error) {
	hdr := make([]byte, 10)
	n, err := f.ReadAt(hdr, 0)
	if err != nil && err != io.EOF {
		return 0, fmt.Errorf("audiohash: read ID3v2 header: %w", err)
	}
	if n < 10 || string(hdr[:3]) != "ID3" {
		return 0, nil
	}
	// Bytes 6..9 are a synchsafe integer: the tag size excluding the 10-byte
	// header (and excluding the optional 10-byte footer).
	size := int64(hdr[6]&0x7f)<<21 | int64(hdr[7]&0x7f)<<14 | int64(hdr[8]&0x7f)<<7 | int64(hdr[9]&0x7f)
	total := 10 + size
	if hdr[5]&0x10 != 0 { // footer-present flag
		total += 10
	}
	return total, nil
}

// mp4Hash hashes the contents of the MP4 "mdat" box, which holds the coded
// audio samples. Tags live in separate boxes (moov/udta/meta/ilst), so editing
// them never touches mdat.
func mp4Hash(f io.ReaderAt, size int64) (string, error) {
	start, length, err := findBox(f, size, "mdat")
	if err != nil {
		return "", err
	}
	return payloadHash(f, start, start+length)
}

// findBox scans the top-level MP4 boxes for the box named want, returning the
// offset and length of its payload (the bytes after the box header).
func findBox(f io.ReaderAt, size int64, want string) (start, length int64, err error) {
	hdr := make([]byte, 8)
	for off := int64(0); off+8 <= size; {
		if _, err := f.ReadAt(hdr, off); err != nil {
			return 0, 0, fmt.Errorf("audiohash: read MP4 box header: %w", err)
		}
		boxSize := int64(binary.BigEndian.Uint32(hdr[0:4]))
		boxType := string(hdr[4:8])
		headerLen := int64(8)
		switch boxSize {
		case 1: // 64-bit "largesize" follows the type field
			ext := make([]byte, 8)
			if _, err := f.ReadAt(ext, off+8); err != nil {
				return 0, 0, fmt.Errorf("audiohash: read MP4 largesize: %w", err)
			}
			large := binary.BigEndian.Uint64(ext)
			if large > math.MaxInt64 {
				return 0, 0, fmt.Errorf("audiohash: MP4 box %q declares an out-of-range largesize", boxType)
			}
			boxSize = int64(large)
			headerLen = 16
		case 0: // box runs to end of file
			boxSize = size - off
		}
		if boxSize < headerLen || off+boxSize > size {
			return 0, 0, fmt.Errorf("audiohash: malformed MP4 box %q", boxType)
		}
		if boxType == want {
			return off + headerLen, boxSize - headerLen, nil
		}
		off += boxSize
	}
	return 0, 0, fmt.Errorf("audiohash: MP4 %q box not found", want)
}

// payloadHash returns the bounded FNV-1a hash of the byte range [start, end),
// with the exact range length bound into the digest.
func payloadHash(f io.ReaderAt, start, end int64) (string, error) {
	if end < start {
		end = start
	}
	payloadLen := end - start
	readLen := payloadLen
	if readLen > maxHashBytes {
		readLen = maxHashBytes
	}

	h := fnv.New64a()
	var lenBuf [8]byte
	// payloadLen is non-negative by construction: end is clamped to start above.
	binary.BigEndian.PutUint64(lenBuf[:], uint64(payloadLen)) //nolint:gosec // G115: payloadLen >= 0, so the conversion cannot wrap.
	_, _ = h.Write(lenBuf[:])
	if readLen > 0 {
		if _, err := io.Copy(h, io.NewSectionReader(f, start, readLen)); err != nil {
			return "", fmt.Errorf("audiohash: read audio payload: %w", err)
		}
	}
	return fmt.Sprintf("fnv1a64:%016x", h.Sum64()), nil
}
