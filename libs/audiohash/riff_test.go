package audiohash

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// riffChunk builds one chunk of a RIFF-family container: a 4-char id, a 32-bit
// size in the container's byte order, the payload, and the pad byte an
// odd-length payload requires (the pad is not counted in the declared size).
func riffChunk(order binary.ByteOrder, id string, payload []byte) []byte {
	b := make([]byte, 8)
	copy(b[0:4], id)
	order.PutUint32(b[4:8], uint32(len(payload)))
	b = append(b, payload...)
	if len(payload)%2 == 1 {
		b = append(b, 0x00)
	}
	return b
}

// riffContainer wraps body chunks in a container header: the magic, the total
// body size, and the form type.
func riffContainer(order binary.ByteOrder, magic, form string, chunks []byte) []byte {
	body := append([]byte(form), chunks...)
	out := make([]byte, 8)
	copy(out[0:4], magic)
	order.PutUint32(out[4:8], uint32(len(body)))
	return append(out, body...)
}

// wavFixture builds a minimal but structurally valid RIFF/WAVE file: a fmt
// chunk, an optional LIST chunk standing in for editable tags, and the data
// chunk holding audio. tagAfterData puts the tag chunk behind the audio, which
// is where several taggers actually write it.
func wavFixture(audio, tag []byte, tagAfterData bool) []byte {
	fmtChunk := riffChunk(binary.LittleEndian, "fmt ", make([]byte, 16))
	data := riffChunk(binary.LittleEndian, "data", audio)
	var tagChunk []byte
	if len(tag) > 0 {
		tagChunk = riffChunk(binary.LittleEndian, "LIST", tag)
	}
	chunks := append([]byte{}, fmtChunk...)
	if tagAfterData {
		chunks = append(chunks, data...)
		chunks = append(chunks, tagChunk...)
	} else {
		chunks = append(chunks, tagChunk...)
		chunks = append(chunks, data...)
	}
	return riffContainer(binary.LittleEndian, "RIFF", "WAVE", chunks)
}

// aiffFixture builds a minimal AIFF (or AIFC) file: a COMM chunk, an optional
// "ID3 " chunk standing in for editable tags, and an SSND chunk whose first
// eight bytes are the offset and blockSize fields rather than audio.
func aiffFixture(frames, tag []byte, form string, ssndOffset uint32) []byte {
	ssndBody := make([]byte, 8)
	binary.BigEndian.PutUint32(ssndBody[0:4], ssndOffset)
	binary.BigEndian.PutUint32(ssndBody[4:8], 0) // blockSize
	ssndBody = append(ssndBody, frames...)

	chunks := riffChunk(binary.BigEndian, "COMM", make([]byte, 18))
	if len(tag) > 0 {
		chunks = append(chunks, riffChunk(binary.BigEndian, "ID3 ", tag)...)
	}
	chunks = append(chunks, riffChunk(binary.BigEndian, "SSND", ssndBody)...)
	return riffContainer(binary.BigEndian, "FORM", form, chunks)
}

func TestFileWAVIgnoresTagChunks(t *testing.T) {
	// 2 KiB of stand-in audio; the hasher never decodes, so opaque bytes are a
	// faithful stand-in for PCM frames.
	audio := bytes.Repeat([]byte{0x11, 0x22, 0x33, 0x44}, 512)

	cases := []struct {
		name string
		data []byte
	}{
		{"no-tag.wav", wavFixture(audio, nil, false)},
		{"small-tag.wav", wavFixture(audio, bytes.Repeat([]byte("A"), 40), false)},
		{"big-tag.wav", wavFixture(audio, bytes.Repeat([]byte("B"), 6000), false)},
		{"odd-tag.wav", wavFixture(audio, bytes.Repeat([]byte("C"), 41), false)}, // exercises the pad byte
		{"tag-after-data.wav", wavFixture(audio, bytes.Repeat([]byte("D"), 40), true)},
	}

	var want string
	for i, c := range cases {
		got, err := File(writeFixture(t, c.name, c.data))
		if err != nil {
			t.Fatalf("File(%s): %v", c.name, err)
		}
		if i == 0 {
			want = got
			if !strings.HasPrefix(got, "fnv1a64:") {
				t.Fatalf("File(%s) = %q, want an fnv1a64: hash", c.name, got)
			}
			continue
		}
		if got != want {
			t.Fatalf("File(%s) = %q, want %q (audio identical, only tags differ)", c.name, got, want)
		}
	}
}

func TestFileAIFFIgnoresTagChunksAndTheSSNDPrefix(t *testing.T) {
	frames := bytes.Repeat([]byte{0x55, 0x66}, 1024)

	cases := []struct {
		name string
		data []byte
	}{
		{"bare.aiff", aiffFixture(frames, nil, "AIFF", 0)},
		{"tagged.aiff", aiffFixture(frames, bytes.Repeat([]byte("T"), 300), "AIFF", 0)},
		{"retagged.aiff", aiffFixture(frames, bytes.Repeat([]byte("U"), 900), "AIFF", 0)},
		{"aifc.aiff", aiffFixture(frames, bytes.Repeat([]byte("V"), 300), "AIFC", 0)},
		// offset/blockSize are container bookkeeping, not audio: a different
		// value must not move the hash.
		{"offset.aiff", aiffFixture(frames, nil, "AIFF", 4096)},
	}

	var want string
	for i, c := range cases {
		got, err := File(writeFixture(t, c.name, c.data))
		if err != nil {
			t.Fatalf("File(%s): %v", c.name, err)
		}
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Fatalf("File(%s) = %q, want %q (only the audio frames may matter)", c.name, got, want)
		}
	}
}

func TestFileWAVDependsOnAudio(t *testing.T) {
	a := writeFixture(t, "a.wav", wavFixture(bytes.Repeat([]byte{0x01}, 2048), nil, false))
	b := writeFixture(t, "b.wav", wavFixture(bytes.Repeat([]byte{0x02}, 2048), nil, false))

	ha, err := File(a)
	if err != nil {
		t.Fatalf("File(a): %v", err)
	}
	hb, err := File(b)
	if err != nil {
		t.Fatalf("File(b): %v", err)
	}
	if ha == hb {
		t.Fatalf("different audio hashed equal: %q", ha)
	}
}

func TestFileWAVClampsAnOverlongDataSize(t *testing.T) {
	// A stream-written or truncated WAV declares a data size the file does not
	// hold. It is still hashable: the declared length is clamped to what is
	// actually there, which here is exactly the honest length — so the two
	// files must hash identically.
	audio := bytes.Repeat([]byte{0x77}, 1024)
	honest := wavFixture(audio, bytes.Repeat([]byte("A"), 40), false)

	overlong := append([]byte{}, honest...)
	// The data chunk is last, so its 4-byte size field sits 4 bytes before the
	// audio, i.e. at len(file)-len(audio)-4.
	binary.LittleEndian.PutUint32(overlong[len(overlong)-len(audio)-4:], uint32(len(audio)+4096))

	hHonest, err := File(writeFixture(t, "honest.wav", honest))
	if err != nil {
		t.Fatalf("File(honest): %v", err)
	}
	hOverlong, err := File(writeFixture(t, "overlong.wav", overlong))
	if err != nil {
		t.Fatalf("File(overlong): %v", err)
	}
	if hHonest != hOverlong {
		t.Fatalf("overlong data size = %q, want %q (it must clamp to the bytes present)", hOverlong, hHonest)
	}
}

func TestFileZeroLengthPayloadIsUnsupported(t *testing.T) {
	// A zero-length payload range digests nothing but its eight length bytes, so
	// every such file lands on one universal value — and the duration guard in the
	// scanner cannot separate them either, since they all decode to ~0 seconds.
	// A declared "data" size of 0 is the sentinel a stream-writing recorder leaves
	// behind when it never goes back to patch the header, so real audio can sit
	// behind it; an AIFF whose SSND holds only its 8-byte offset/blockSize prefix
	// is the same shape. Declining costs a fallback; a shared hash costs one
	// track's listening history.
	zeroData := func(audio []byte) []byte {
		data := wavFixture(audio, nil, false)
		// The data chunk is last, so its 4-byte size field sits 4 bytes ahead of
		// the audio, i.e. at len(file)-len(audio)-4.
		binary.LittleEndian.PutUint32(data[len(data)-len(audio)-4:], 0)
		return data
	}

	cases := []struct {
		name string
		data []byte
	}{
		{"zero-data-a.wav", zeroData(bytes.Repeat([]byte{0x11}, 2048))},
		{"zero-data-b.wav", zeroData(bytes.Repeat([]byte{0x22}, 4096))},
		{"empty-data.wav", wavFixture(nil, bytes.Repeat([]byte("A"), 40), false)},
		{"empty-ssnd.aiff", aiffFixture(nil, nil, "AIFF", 0)},
	}

	for _, c := range cases {
		if _, err := File(writeFixture(t, c.name, c.data)); !errors.Is(err, ErrUnsupported) {
			t.Errorf("File(%s) err = %v, want ErrUnsupported", c.name, err)
		}
	}
}

func TestFileWAVRejectsAnOverlongSkippedChunk(t *testing.T) {
	// A chunk being skipped gets no latitude: an out-of-range length makes the
	// next chunk offset a guess, so it must be an error rather than a hash
	// computed over whatever happened to be there.
	audio := bytes.Repeat([]byte{0x33}, 512)
	tag := bytes.Repeat([]byte("A"), 40)
	data := wavFixture(audio, tag, false)
	// The LIST chunk's size field is at 12 (container header) + 24 (fmt chunk) + 4.
	binary.LittleEndian.PutUint32(data[12+24+4:], uint32(len(tag)+4096))

	_, err := File(writeFixture(t, "broken.wav", data))
	if err == nil {
		t.Fatal("an out-of-range skipped chunk must be an error")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Fatalf("got ErrUnsupported, want a parse error: %v", err)
	}
}
