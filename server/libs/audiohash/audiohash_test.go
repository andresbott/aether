package audiohash

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// flacFixture builds a minimal but structurally valid FLAC file: the "fLaC"
// magic, a STREAMINFO block carrying md5 as its decoded-audio signature, and an
// optional trailing VORBIS_COMMENT block standing in for editable tags.
func flacFixture(md5 [16]byte, comment []byte) []byte {
	b := []byte("fLaC")
	// STREAMINFO is block type 0; it is the last block only when no comment
	// block follows it (top bit of the type byte is the last-block flag).
	head := byte(0x00)
	if len(comment) == 0 {
		head = 0x80
	}
	b = append(b, head)
	b = append(b, 0x00, 0x00, 34) // 24-bit big-endian body length = 34
	body := make([]byte, 34)
	copy(body[18:], md5[:]) // the MD5 signature is the final 16 bytes
	b = append(b, body...)
	if len(comment) > 0 {
		b = append(b, 0x80|0x04) // VORBIS_COMMENT (type 4), flagged last
		n := len(comment)
		b = append(b, byte(n>>16), byte(n>>8), byte(n))
		b = append(b, comment...)
	}
	return b
}

func writeFixture(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFileFLACReturnsStreamInfoMD5IgnoringTags(t *testing.T) {
	md5 := [16]byte{
		0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
	}
	want := "flacmd5:" + hex.EncodeToString(md5[:])

	// Same decoded audio (same MD5), two different tag payloads: the hash must
	// depend only on the audio, so both must produce the same value.
	a := writeFixture(t, "a.flac", flacFixture(md5, []byte("ARTIST=Alice")))
	b := writeFixture(t, "b.flac", flacFixture(md5, []byte("ARTIST=Bob and a considerably longer comment block")))

	for _, p := range []string{a, b} {
		got, err := File(p)
		if err != nil {
			t.Fatalf("File(%s): %v", p, err)
		}
		if got != want {
			t.Fatalf("File(%s) = %q, want %q", p, got, want)
		}
	}
}

func TestFileRealFLACGolden(t *testing.T) {
	// A real encoder-produced FLAC: proves the STREAMINFO MD5 is read from the
	// correct offset on genuine files, not just hand-built fixtures. The want
	// value was read independently from the file's bytes, not from this code.
	got, err := File("testdata/sample.flac")
	if err != nil {
		t.Fatalf("File(sample.flac): %v", err)
	}
	want := "flacmd5:1ee0193671609c7d63cfe89b920ad313"
	if got != want {
		t.Fatalf("File(sample.flac) = %q, want %q", got, want)
	}
}

func TestFileRealMP3(t *testing.T) {
	// A real ID3v2.4-tagged MP3: proves the front tag is located and skipped on
	// a genuine file without error, yielding a well-formed hash.
	got, err := File("testdata/sample.mp3")
	if err != nil {
		t.Fatalf("File(sample.mp3): %v", err)
	}
	if !strings.HasPrefix(got, "fnv1a64:") || len(got) != len("fnv1a64:")+16 {
		t.Fatalf("File(sample.mp3) = %q, want a 16-hex-digit fnv1a64 hash", got)
	}
}

func TestFileFLACZeroMD5IsUnsupported(t *testing.T) {
	// A zero STREAMINFO MD5 is FLAC's "signature not computed" sentinel, not a
	// real fingerprint; treating it as one would match every other unsigned
	// FLAC. It must fall back instead.
	p := writeFixture(t, "unsigned.flac", flacFixture([16]byte{}, []byte("ARTIST=X")))
	if _, err := File(p); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("File(zero-md5 flac) err = %v, want ErrUnsupported", err)
	}
}

// syncsafe encodes n as ID3v2's 4-byte synchsafe integer (7 bits per byte).
func syncsafe(n int) []byte {
	return []byte{
		byte((n >> 21) & 0x7f),
		byte((n >> 14) & 0x7f),
		byte((n >> 7) & 0x7f),
		byte(n & 0x7f),
	}
}

// id3v2 builds an ID3v2.4 front tag wrapping the given body bytes.
func id3v2(body []byte) []byte {
	h := []byte{'I', 'D', '3', 0x04, 0x00, 0x00}
	h = append(h, syncsafe(len(body))...)
	return append(h, body...)
}

// id3v2WithFooter builds an ID3v2.4 tag with the footer-present flag set and a
// 10-byte footer appended, as permitted by ID3v2.4.
func id3v2WithFooter(body []byte) []byte {
	h := []byte{'I', 'D', '3', 0x04, 0x00, 0x10} // 0x10 = footer present
	h = append(h, syncsafe(len(body))...)
	h = append(h, body...)
	footer := []byte{'3', 'D', 'I', 0x04, 0x00, 0x10}
	footer = append(footer, syncsafe(len(body))...)
	return append(h, footer...)
}

func cat(chunks ...[]byte) []byte {
	var out []byte
	for _, c := range chunks {
		out = append(out, c...)
	}
	return out
}

// id3v1 builds a 128-byte ID3v1 trailer, padded with fill so two calls with
// different fills stand in for two different tag payloads.
func id3v1(fill byte) []byte {
	b := make([]byte, 128)
	copy(b, "TAG")
	for i := 3; i < len(b); i++ {
		b[i] = fill
	}
	return b
}

// apeTag builds a minimal footer-only APEv2 tag (no header block). The item
// body is padded with fill so two calls stand in for two tag payloads.
func apeTag(fill byte) []byte {
	body := bytes.Repeat([]byte{fill}, 40)
	foot := make([]byte, 32)
	copy(foot, "APETAGEX")
	binary.LittleEndian.PutUint32(foot[8:12], 2000)                  // version 2.0
	binary.LittleEndian.PutUint32(foot[12:16], uint32(len(body)+32)) // size: items + footer
	binary.LittleEndian.PutUint32(foot[16:20], 1)                    // item count
	binary.LittleEndian.PutUint32(foot[20:24], 0)                    // flags: footer, no header
	return append(body, foot...)
}

func TestFileMP3IgnoresID3v2FrontTag(t *testing.T) {
	// 2 KiB of stand-in audio; the hasher never decodes, so opaque bytes are a
	// faithful stand-in for encoded frames.
	audio := bytes.Repeat([]byte{0xAA, 0xBB, 0xCC, 0xDD}, 512)

	cases := []struct {
		name string
		data []byte
	}{
		{"no-tag.mp3", audio},
		{"small-tag.mp3", cat(id3v2(bytes.Repeat([]byte("A"), 128)), audio)},
		{"big-tag.mp3", cat(id3v2(bytes.Repeat([]byte("B"), 6000)), audio)},
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

func TestFileMP3IgnoresID3v2Footer(t *testing.T) {
	audio := bytes.Repeat([]byte{0x5A}, 1024)

	bare, err := File(writeFixture(t, "bare.mp3", audio))
	if err != nil {
		t.Fatalf("File(bare): %v", err)
	}
	withFooter, err := File(writeFixture(t, "footer.mp3",
		cat(id3v2WithFooter(bytes.Repeat([]byte("F"), 200)), audio)))
	if err != nil {
		t.Fatalf("File(footer): %v", err)
	}
	if bare != withFooter {
		t.Fatalf("File(footer) = %q, want %q (ID3v2 footer must be excluded)", withFooter, bare)
	}
}

func TestFileMP3IgnoresTrailingID3v1(t *testing.T) {
	audio := bytes.Repeat([]byte{0x11, 0x22, 0x33, 0x44}, 512)

	cases := []struct {
		name string
		data []byte
	}{
		{"bare.mp3", audio},
		{"v1.mp3", cat(audio, id3v1('x'))},
		{"v1-other.mp3", cat(audio, id3v1('y'))},
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
			t.Fatalf("File(%s) = %q, want %q (trailing ID3v1 must be ignored)", c.name, got, want)
		}
	}
}

func TestFileMP3IgnoresTrailingAPE(t *testing.T) {
	audio := bytes.Repeat([]byte{0x55, 0x66, 0x77, 0x88}, 512)

	cases := []struct {
		name string
		data []byte
	}{
		{"bare.mp3", audio},
		{"ape.mp3", cat(audio, apeTag('a'))},
		{"ape-other.mp3", cat(audio, apeTag('b'))},
		{"ape-and-v1.mp3", cat(audio, apeTag('a'), id3v1('z'))}, // APE then ID3v1
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
			t.Fatalf("File(%s) = %q, want %q (trailing APE must be ignored)", c.name, got, want)
		}
	}
}

func TestFileUnsupportedFormatReturnsErrUnsupported(t *testing.T) {
	// .wma is one of walk.go's sixteen extensions that this package does not
	// cover; such a file must report ErrUnsupported so the scanner falls back to
	// its other identity signals rather than treating it as a failure.
	p := writeFixture(t, "song.wma", bytes.Repeat([]byte{0x00}, 64))
	_, err := File(p)
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("File(.wma) err = %v, want ErrUnsupported", err)
	}
}

func TestReaderHashesFromOpenHandle(t *testing.T) {
	md5 := [16]byte{
		0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30,
	}
	audio := bytes.Repeat([]byte{0xAA, 0xBB, 0xCC, 0xDD}, 256)

	cases := []struct {
		name       string
		data       []byte
		wantPrefix string
	}{
		{"song.flac", flacFixture(md5, []byte("ARTIST=A")), "flacmd5:"},
		{"song.mp3", cat(id3v2(bytes.Repeat([]byte("T"), 80)), audio, id3v1('z')), "fnv1a64:"},
		{"song.m4a", cat(
			mp4Box("ftyp", []byte("M4A ")),
			mp4Box("moov", bytes.Repeat([]byte("m"), 60)),
			mp4Box("mdat", audio),
		), "fnv1a64:"},
		{"song.wav", wavFixture(audio, []byte("LISTINFOtag"), false), "fnv1a64:"},
		{"song.aiff", aiffFixture(audio, []byte("id3tag"), "AIFF", 0), "fnv1a64:"},
		{"song.ogg", oggStream(1, 4321, vorbisIdent(),
			append([]byte("\x03vorbis"), []byte("ARTIST=A")...),
			bytes.Repeat([]byte{0x05}, 600), audio), "oggfnv1a64:"},
		{"song.opus", oggStream(1, 4321, opusHead(),
			append([]byte("OpusTags"), []byte("ARTIST=A")...), audio), "oggfnv1a64:"},
	}

	for _, c := range cases {
		// Reader reuses a caller-supplied handle (here a bytes.Reader, an
		// io.ReaderAt) and never opens a path itself.
		got, err := Reader(bytes.NewReader(c.data), int64(len(c.data)), c.name)
		if err != nil {
			t.Fatalf("Reader(%s): %v", c.name, err)
		}
		if !strings.HasPrefix(got, c.wantPrefix) {
			t.Fatalf("Reader(%s) = %q, want prefix %q", c.name, got, c.wantPrefix)
		}
		// It must agree with File() reading the same bytes off disk.
		viaFile, err := File(writeFixture(t, c.name, c.data))
		if err != nil {
			t.Fatalf("File(%s): %v", c.name, err)
		}
		if got != viaFile {
			t.Fatalf("Reader(%s) = %q, File = %q; want equal", c.name, got, viaFile)
		}
	}
}

// mp4Box builds a single 32-bit-sized MP4 box of the given type wrapping
// payload.
func mp4Box(typ string, payload []byte) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint32(b[0:4], uint32(8+len(payload)))
	copy(b[4:8], typ)
	return append(b, payload...)
}

func TestFileMP4IgnoresMetadataBoxes(t *testing.T) {
	audio := bytes.Repeat([]byte{0x13, 0x37}, 512)
	ftyp := mp4Box("ftyp", []byte("M4A isom"))
	mdat := mp4Box("mdat", audio)

	cases := []struct {
		name string
		data []byte
	}{
		{"small.m4a", cat(ftyp, mp4Box("moov", bytes.Repeat([]byte("m"), 50)), mdat)},
		{"big.m4a", cat(ftyp, mp4Box("moov", bytes.Repeat([]byte("M"), 5000)), mdat)},
		{"swap.m4b", cat(ftyp, mdat, mp4Box("moov", bytes.Repeat([]byte("x"), 200)))}, // mdat before moov
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
			t.Fatalf("File(%s) = %q, want %q (metadata boxes must be ignored)", c.name, got, want)
		}
	}
}

func TestFileMP3DependsOnAudioBytes(t *testing.T) {
	a := writeFixture(t, "a.mp3", bytes.Repeat([]byte{0x01}, 2048))
	b := writeFixture(t, "b.mp3", bytes.Repeat([]byte{0x02}, 2048))

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

func TestFileMP3DependsOnLengthBeyondBound(t *testing.T) {
	// Two payloads whose first maxHashBytes are identical but whose total
	// lengths differ. Only mixing the exact payload length into the digest
	// tells them apart, since the bounded read sees the same prefix.
	short := writeFixture(t, "short.mp3", bytes.Repeat([]byte{0x07}, maxHashBytes+1024))
	long := writeFixture(t, "long.mp3", bytes.Repeat([]byte{0x07}, maxHashBytes+8192))

	hs, err := File(short)
	if err != nil {
		t.Fatalf("File(short): %v", err)
	}
	hl, err := File(long)
	if err != nil {
		t.Fatalf("File(long): %v", err)
	}
	if hs == hl {
		t.Fatalf("same-prefix payloads of different length hashed equal: %q", hs)
	}
}
