package audiohash

import (
	"encoding/binary"
	"fmt"
	"io"
	"slices"
)

// wavHash hashes the contents of the RIFF/WAVE "data" chunk, which holds the
// PCM samples. Tags live in separate chunks — a LIST/INFO block, or an "id3 "
// chunk, either of which may sit before or after the audio — so editing them
// never touches data.
func wavHash(f io.ReaderAt, size int64) (string, error) {
	start, length, err := findChunk(f, size, binary.LittleEndian, "RIFF", []string{"WAVE"}, "data")
	if err != nil {
		return "", err
	}
	return payloadHash(f, start, start+length)
}

// aiffHash hashes the sample frames of the AIFF "SSND" chunk. Its first eight
// bytes are the offset and blockSize fields — container bookkeeping, not audio —
// so they are skipped. Tags live in ID3/ANNO/NAME chunks and never touch the
// frames. AIFC (compressed AIFF) uses the same layout and is accepted too.
func aiffHash(f io.ReaderAt, size int64) (string, error) {
	start, length, err := findChunk(f, size, binary.BigEndian, "FORM", []string{"AIFF", "AIFC"}, "SSND")
	if err != nil {
		return "", err
	}
	const ssndPrefix = 8 // offset + blockSize
	if length < ssndPrefix {
		return "", fmt.Errorf("audiohash: AIFF SSND chunk too short: %d bytes", length)
	}
	return payloadHash(f, start+ssndPrefix, start+length)
}

// findChunk walks the chunk list of a RIFF-family container and returns the
// offset and length of the chunk named want. WAV (RIFF, little-endian sizes)
// and AIFF (FORM, big-endian sizes) differ only in byte order, container magic
// and form type, so one walker serves both — the same way findBox serves every
// MP4 flavour.
//
// The wanted chunk's declared length is clamped to what the file actually
// holds: a truncated or stream-written WAV routinely declares a "data" size it
// does not have, and that file is still perfectly hashable. A chunk being
// *skipped* gets no such latitude — an out-of-range length there would make the
// next offset a guess, so it is an error.
//
// The clamp is best-effort and has one honest limitation. When a file whose
// wanted chunk had to be clamped also carries a tag chunk *after* it, the clamp
// runs to end of file and pulls those tags into the digest, so editing them moves
// the hash. Such a file's declared length is already impossible, which leaves no
// way to locate where its audio really ends — there is nothing better to do than
// take what is there. A file in that shape is simply not retag-stable.
func findChunk(f io.ReaderAt, size int64, order binary.ByteOrder, container string, forms []string, want string) (start, length int64, err error) {
	const containerHeader = 12 // magic + size + form type
	hdr := make([]byte, containerHeader)
	if _, err := f.ReadAt(hdr, 0); err != nil {
		return 0, 0, fmt.Errorf("audiohash: read %s header: %w", container, err)
	}
	if string(hdr[0:4]) != container {
		return 0, 0, fmt.Errorf("audiohash: not a %s container", container)
	}
	if form := string(hdr[8:12]); !slices.Contains(forms, form) {
		return 0, 0, fmt.Errorf("audiohash: unexpected %s form type %q", container, form)
	}

	chunk := make([]byte, 8)
	for off := int64(containerHeader); off+8 <= size; {
		if _, err := f.ReadAt(chunk, off); err != nil {
			return 0, 0, fmt.Errorf("audiohash: read %s chunk header: %w", container, err)
		}
		id := string(chunk[0:4])
		declared := int64(order.Uint32(chunk[4:8]))
		body := off + 8
		if id == want {
			if avail := size - body; declared > avail {
				declared = avail
			}
			return body, declared, nil
		}
		if declared > size-body {
			return 0, 0, fmt.Errorf("audiohash: %s chunk %q declares %d bytes, past end of file", container, id, declared)
		}
		// Chunk bodies are padded to an even length; the pad byte is not counted
		// in the declared size. The advance is never below 8, so a zero-length
		// chunk cannot stall the walk.
		off = body + declared + declared&1
	}
	return 0, 0, fmt.Errorf("audiohash: %s %q chunk not found", container, want)
}
