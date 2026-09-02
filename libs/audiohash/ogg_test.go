package audiohash

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"testing"
)

// oggLacing encodes a packet length as Ogg segment lacing values: as many 255s
// as fit, then a terminating value below 255. A packet whose length is an exact
// multiple of 255 still needs the terminating zero, or the next page would be
// read as a continuation.
func oggLacing(n int) []byte {
	out := make([]byte, 0, n/255+1)
	for n >= 255 {
		out = append(out, 255)
		n -= 255
	}
	return append(out, byte(n))
}

// oggPageBytes builds one Ogg page. The CRC field is deliberately left zero:
// the hasher must never check it, because a retag changes every downstream CRC.
func oggPageBytes(serial, seq uint32, granule uint64, flags byte, segments [][]byte) []byte {
	var lacing []byte
	var payload []byte
	for _, seg := range segments {
		lacing = append(lacing, byte(len(seg)))
		payload = append(payload, seg...)
	}
	head := make([]byte, oggHeaderFixed)
	copy(head[0:4], "OggS")
	head[4] = 0 // version
	head[5] = flags
	binary.LittleEndian.PutUint64(head[6:14], granule)
	binary.LittleEndian.PutUint32(head[14:18], serial)
	binary.LittleEndian.PutUint32(head[18:22], seq)
	head[26] = byte(len(lacing))
	out := append(head, lacing...)
	return append(out, payload...)
}

// oggSegments splits a packet into Ogg segments: 255-byte pieces plus a
// terminating piece below 255. Splitting a packet any other way would make the
// pieces read as separate packets, since a lacing value below 255 is exactly
// what ends one.
func oggSegments(pkt []byte) [][]byte {
	var out [][]byte
	for _, n := range oggLacing(len(pkt)) {
		out = append(out, pkt[:n])
		pkt = pkt[n:]
	}
	return out
}

// oggStream lays out one packet per page, spilling onto further pages when a
// packet needs more than the 255 segments a page header can count. granule goes
// on the final page, which is where oggLastGranule looks for it.
func oggStream(serial uint32, granule uint64, packets ...[]byte) []byte {
	var out []byte
	seq := uint32(0)
	for i, pkt := range packets {
		segs := oggSegments(pkt)
		continued := false
		for len(segs) > 0 {
			page := segs[:min(255, len(segs))]
			segs = segs[len(page):]

			flags := byte(0)
			if seq == 0 {
				flags |= 0x02 // beginning of stream
			}
			if continued {
				flags |= 0x01 // opens with the tail of the previous page's packet
			}
			g := uint64(0)
			if i == len(packets)-1 && len(segs) == 0 {
				g = granule
				flags |= 0x04 // end of stream
			}
			out = append(out, oggPageBytes(serial, seq, g, flags, page)...)
			seq++
			continued = true
		}
	}
	return out
}

func TestReadOggPageParsesHeaderAndPayload(t *testing.T) {
	payload := bytes.Repeat([]byte{0xAB}, 300) // spans two lacing values
	page := oggPageBytes(0xDEADBEEF, 7, 4242, 0x04, [][]byte{payload[:255], payload[255:]})

	got, err := readOggPage(bytes.NewReader(page), 0, int64(len(page)))
	if err != nil {
		t.Fatalf("readOggPage: %v", err)
	}
	if got.serial != 0xDEADBEEF {
		t.Fatalf("serial = %#x, want 0xDEADBEEF", got.serial)
	}
	if got.total != int64(len(page)) {
		t.Fatalf("total = %d, want %d (it must cover header, lacing table and payload)", got.total, len(page))
	}
	if !bytes.Equal(got.lacing, []byte{255, 45}) {
		t.Fatalf("lacing = %v, want [255 45]", got.lacing)
	}
	if !bytes.Equal(got.payload, payload) {
		t.Fatalf("payload mismatch: got %d bytes, want %d", len(got.payload), len(payload))
	}
}

func TestReadOggPageRejectsBadSyncAndVersion(t *testing.T) {
	good := oggPageBytes(1, 0, 0, 0x02, [][]byte{[]byte("hello")})

	badSync := append([]byte{}, good...)
	copy(badSync[0:4], "XggS")
	if _, err := readOggPage(bytes.NewReader(badSync), 0, int64(len(badSync))); err == nil {
		t.Fatal("a bad capture pattern must be an error, not a silently walked page")
	}

	badVersion := append([]byte{}, good...)
	badVersion[4] = 1
	if _, err := readOggPage(bytes.NewReader(badVersion), 0, int64(len(badVersion))); err == nil {
		t.Fatal("an unknown Ogg page version must be an error")
	}
}

func TestReadOggPageRejectsAPageRunningPastEOF(t *testing.T) {
	page := oggPageBytes(1, 0, 0, 0x02, [][]byte{bytes.Repeat([]byte{0x01}, 200)})
	truncated := page[:len(page)-50]
	if _, err := readOggPage(bytes.NewReader(truncated), 0, int64(len(truncated))); err == nil {
		t.Fatal("a page whose payload runs past EOF must be an error")
	}
}

// vorbisIdent and opusHead are the first packets that identify each mapping.
// Only their leading magic matters to the hasher.
func vorbisIdent() []byte {
	return append([]byte("\x01vorbis"), bytes.Repeat([]byte{0x01}, 23)...)
}

func opusHead() []byte {
	return append([]byte("OpusHead"), bytes.Repeat([]byte{0x01}, 11)...)
}

func TestFileOggVorbisIgnoresTheCommentHeader(t *testing.T) {
	ident := vorbisIdent()
	setup := bytes.Repeat([]byte{0x05}, 900) // the codebooks: a retag never rewrites them
	audio := bytes.Repeat([]byte{0x42, 0x43}, 4000)

	cases := []struct {
		name    string
		comment []byte
	}{
		{"short.ogg", append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 20)...)},
		{"long.ogg", append([]byte("\x03vorbis"), bytes.Repeat([]byte("B"), 3000)...)},
		{"huge.ogg", append([]byte("\x03vorbis"), bytes.Repeat([]byte("C"), 90000)...)}, // spans many pages
	}

	var want string
	for i, c := range cases {
		data := oggStream(0x0BADF00D, 7654321, ident, c.comment, setup, audio)
		got, err := File(writeFixture(t, c.name, data))
		if err != nil {
			t.Fatalf("File(%s): %v", c.name, err)
		}
		if i == 0 {
			want = got
			if !strings.HasPrefix(got, "oggfnv1a64:") {
				t.Fatalf("File(%s) = %q, want an oggfnv1a64: hash", c.name, got)
			}
			continue
		}
		if got != want {
			t.Fatalf("File(%s) = %q, want %q (only the comment header differs)", c.name, got, want)
		}
	}
}

func TestFileOpusIgnoresOpusTags(t *testing.T) {
	head := opusHead()
	audio := bytes.Repeat([]byte{0x77, 0x88}, 4000)

	bare, err := File(writeFixture(t, "bare.opus",
		oggStream(42, 999, head, append([]byte("OpusTags"), bytes.Repeat([]byte("A"), 30)...), audio)))
	if err != nil {
		t.Fatalf("File(bare): %v", err)
	}
	retagged, err := File(writeFixture(t, "retagged.opus",
		oggStream(42, 999, head, append([]byte("OpusTags"), bytes.Repeat([]byte("B"), 5000)...), audio)))
	if err != nil {
		t.Fatalf("File(retagged): %v", err)
	}
	if bare != retagged {
		t.Fatalf("File(retagged) = %q, want %q (OpusTags must be excluded)", retagged, bare)
	}
}

func TestFileOggSurvivesRepagination(t *testing.T) {
	// Same packets, different page layout: a remux changes page boundaries,
	// sequence numbers and CRCs but not packet contents. Hashing reassembled
	// packets rather than file bytes is what makes this hold.
	ident := vorbisIdent()
	comment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...)
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x11, 0x22, 0x33}, 2000)

	onePagePerPacket := oggStream(9, 555, ident, comment, setup, audio)

	// The same four packets, repaginated: all three headers share one page, and
	// the audio packet is split across two. Packet *segmentation* is preserved —
	// only a lacing value below 255 may end a packet, so the first audio page
	// must end on a 255 — while page boundaries, sequence numbers and CRCs all
	// differ. That is exactly what a remux does.
	headers := append(append(oggSegments(ident), oggSegments(comment)...), oggSegments(setup)...)
	audioSegs := oggSegments(audio)
	const split = 11 // 11 full 255-byte segments stay on the first audio page

	var packed []byte
	packed = append(packed, oggPageBytes(9, 0, 0, 0x02, headers)...)
	packed = append(packed, oggPageBytes(9, 1, 0, 0, audioSegs[:split])...)
	packed = append(packed, oggPageBytes(9, 2, 555, 0x01|0x04, audioSegs[split:])...)

	a, err := File(writeFixture(t, "a.ogg", onePagePerPacket))
	if err != nil {
		t.Fatalf("File(a): %v", err)
	}
	b, err := File(writeFixture(t, "b.ogg", packed))
	if err != nil {
		t.Fatalf("File(b): %v", err)
	}
	if a != b {
		t.Fatalf("repagination changed the hash: %q vs %q", a, b)
	}
}

func TestFileOggKeepsTheSetupHeaderAndTheAudio(t *testing.T) {
	ident := vorbisIdent()
	comment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...)
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x11}, 3000)

	base, err := File(writeFixture(t, "base.ogg", oggStream(1, 100, ident, comment, setup, audio)))
	if err != nil {
		t.Fatalf("File(base): %v", err)
	}
	otherSetup, err := File(writeFixture(t, "setup.ogg",
		oggStream(1, 100, ident, comment, bytes.Repeat([]byte{0x06}, 600), audio)))
	if err != nil {
		t.Fatalf("File(setup): %v", err)
	}
	if base == otherSetup {
		t.Fatal("the setup header is audio-characteristic and must be inside the digest")
	}
	otherAudio, err := File(writeFixture(t, "audio.ogg",
		oggStream(1, 100, ident, comment, setup, bytes.Repeat([]byte{0x12}, 3000))))
	if err != nil {
		t.Fatalf("File(audio): %v", err)
	}
	if base == otherAudio {
		t.Fatal("different audio must hash differently")
	}
}

func TestFileOggDependsOnTheGranule(t *testing.T) {
	ident := vorbisIdent()
	comment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...)
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x11}, 3000)

	short, err := File(writeFixture(t, "short.ogg", oggStream(1, 1000, ident, comment, setup, audio)))
	if err != nil {
		t.Fatalf("File(short): %v", err)
	}
	long, err := File(writeFixture(t, "long.ogg", oggStream(1, 999999, ident, comment, setup, audio)))
	if err != nil {
		t.Fatalf("File(long): %v", err)
	}
	if short == long {
		t.Fatal("the granule is the length component of the digest and must be mixed in")
	}
}

func TestFileOggIgnoresAnotherLogicalStream(t *testing.T) {
	// A Skeleton track or a chained second stream must not move the hash: only
	// the serial of the first page is hashed.
	ident := vorbisIdent()
	comment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...)
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x11}, 3000)

	plain := oggStream(1, 4321, ident, comment, setup, audio)
	// Splice a foreign-serial page in between, and keep the real last page last
	// so the granule search still finds it.
	foreign := oggPageBytes(2, 0, 0, 0x02, [][]byte{bytes.Repeat([]byte{0xEE}, 100)})
	firstPage, err := readOggPage(bytes.NewReader(plain), 0, int64(len(plain)))
	if err != nil {
		t.Fatalf("readOggPage: %v", err)
	}
	multiplexed := append([]byte{}, plain[:firstPage.total]...)
	multiplexed = append(multiplexed, foreign...)
	multiplexed = append(multiplexed, plain[firstPage.total:]...)

	a, err := File(writeFixture(t, "plain.ogg", plain))
	if err != nil {
		t.Fatalf("File(plain): %v", err)
	}
	b, err := File(writeFixture(t, "muxed.ogg", multiplexed))
	if err != nil {
		t.Fatalf("File(muxed): %v", err)
	}
	if a != b {
		t.Fatalf("a foreign logical stream changed the hash: %q vs %q", a, b)
	}
}

func TestFileOggToleratesAFalseOggSInTheAudio(t *testing.T) {
	// Audio bytes can spell "OggS" by accident. Because a retag shifts every
	// absolute offset, a granule search that accepted such a match could pick a
	// different one before and after the retag — so the last-page candidate must
	// be required to end exactly at EOF.
	ident := vorbisIdent()
	setup := bytes.Repeat([]byte{0x05}, 600)
	// Plant a full plausible page header carrying the stream's own serial so it
	// clears sync, version and serial checks but fails the ends-at-EOF check.
	plantedHeader := make([]byte, 0, 34)
	plantedHeader = append(plantedHeader, []byte("OggS")...) // capture pattern
	plantedHeader = append(plantedHeader, 0x00)              // version
	plantedHeader = append(plantedHeader, 0x00)              // flags
	granuleBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(granuleBuf, 0xDEAD) // distinguishable granule
	plantedHeader = append(plantedHeader, granuleBuf...)
	serialBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(serialBuf, 1) // match the fixture's serial
	plantedHeader = append(plantedHeader, serialBuf...)
	plantedHeader = append(plantedHeader, 0x63, 0x00, 0x00, 0x00) // sequence 99
	plantedHeader = append(plantedHeader, 0x00, 0x00, 0x00, 0x00) // CRC (never checked)
	plantedHeader = append(plantedHeader, 0x03)                   // 3 segments
	plantedHeader = append(plantedHeader, 100, 50, 30)            // lacing: total 180 bytes
	audio := append(bytes.Repeat([]byte{0x11}, 2000), plantedHeader...)
	audio = append(audio, bytes.Repeat([]byte{0x12}, 500)...) // more audio after it

	a, err := File(writeFixture(t, "a.ogg", oggStream(1, 2468, ident,
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 20)...), setup, audio)))
	if err != nil {
		t.Fatalf("File(a): %v", err)
	}
	b, err := File(writeFixture(t, "b.ogg", oggStream(1, 2468, ident,
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("B"), 4000)...), setup, audio)))
	if err != nil {
		t.Fatalf("File(b): %v", err)
	}
	if a != b {
		t.Fatalf("a false OggS in the audio broke retag invariance: %q vs %q", a, b)
	}
}

func TestOggLastGranuleIgnoresAPlantedHeaderThatDoesNotEndAtEOF(t *testing.T) {
	// Direct unit test on oggLastGranule: it must return the true final granule
	// (2468) rather than the planted candidate's granule (0xDEAD), because the
	// planted page header does not end exactly at EOF. This is the white-box
	// assertion that fails when the ends-at-EOF check is removed.
	ident := vorbisIdent()
	setup := bytes.Repeat([]byte{0x05}, 600)
	plantedHeader := make([]byte, 0, 34)
	plantedHeader = append(plantedHeader, []byte("OggS")...)
	plantedHeader = append(plantedHeader, 0x00) // version
	plantedHeader = append(plantedHeader, 0x00) // flags
	granuleBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(granuleBuf, 0xDEAD)
	plantedHeader = append(plantedHeader, granuleBuf...)
	serialBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(serialBuf, 1)
	plantedHeader = append(plantedHeader, serialBuf...)
	plantedHeader = append(plantedHeader, 0x63, 0x00, 0x00, 0x00)
	plantedHeader = append(plantedHeader, 0x00, 0x00, 0x00, 0x00)
	plantedHeader = append(plantedHeader, 0x03)
	plantedHeader = append(plantedHeader, 100, 50, 30)
	audio := append(bytes.Repeat([]byte{0x11}, 2000), plantedHeader...)
	audio = append(audio, bytes.Repeat([]byte{0x12}, 500)...)

	data := oggStream(1, 2468, ident,
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 20)...), setup, audio)

	got := oggLastGranule(bytes.NewReader(data), int64(len(data)), 1)
	if got != 2468 {
		t.Fatalf("oggLastGranule = %d, want 2468 (the true final granule, not the planted 0xDEAD)", got)
	}
}

func TestFileOggUnknownMappingIsUnsupported(t *testing.T) {
	// Ogg FLAC puts each metadata block in its own packet, so a fixed skip of
	// two would leave an embedded picture inside the digest. Declining beats
	// emitting a value that does not survive a retag.
	data := oggStream(1, 100,
		append([]byte("\x7fFLAC"), bytes.Repeat([]byte{0x01}, 30)...),
		bytes.Repeat([]byte{0x02}, 100),
		bytes.Repeat([]byte{0x03}, 1000))
	if _, err := File(writeFixture(t, "flac-in-ogg.ogg", data)); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("File(Ogg FLAC) err = %v, want ErrUnsupported", err)
	}
}

func TestFileOggLostSyncIsAnError(t *testing.T) {
	// Build a stream but remove the EOS flag from its last page so the walk
	// continues to EOF and encounters the trailing garbage.
	data := oggStream(1, 100, vorbisIdent(),
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 20)...),
		bytes.Repeat([]byte{0x05}, 600), bytes.Repeat([]byte{0x11}, 1000))
	// Clear the EOS flag from the last page (byte 5 of the last page header).
	// oggStream puts the EOS flag on the final page; walk backward to find it.
	for i := len(data) - 1; i >= 4; i-- {
		if string(data[i-3:i+1]) == "OggS" {
			data[i+2] &^= 0x04 // clear EOS bit
			break
		}
	}
	data = append(data, bytes.Repeat([]byte{0xFF}, 64)...) // trailing garbage

	_, err := File(writeFixture(t, "garbage.ogg", data))
	if err == nil {
		t.Fatal("trailing garbage where a page header should be must be an error")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Fatalf("got ErrUnsupported, want a parse error: %v", err)
	}
}

// countingReaderAt wraps an io.ReaderAt and counts total bytes read.
type countingReaderAt struct {
	r     *bytes.Reader
	bytes int
}

func (c *countingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := c.r.ReadAt(p, off)
	c.bytes += n
	return n, err
}

func TestReaderStopsAtEndOfStreamAndHonoursTheReadBound(t *testing.T) {
	// Build a chained fixture: a Vorbis stream with ~200 KiB of audio (under the
	// 256 KiB hash bound, so we finish hashing the first stream entirely) and the
	// EOS flag on its final page, concatenated with a second logical stream carrying
	// ~2 MiB of payload. Without the EOS check, the walk continues into the second
	// stream to hash up to the 256 KiB bound, reading far more pages.
	//
	// A chained file's hash differs from the hash of a file containing only its
	// first stream: the target stream has no page ending at end of file (a second
	// logical stream follows it), so its granule comes back 0 instead of the true
	// value. That is deterministic and retag-stable, which is the property that
	// matters. This test pins the chained file's own retag-stability rather than
	// comparing it to a different file.
	ident := vorbisIdent()
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x42, 0x43}, 100*1024) // ~200 KiB

	// Build a second stream on a different serial with ~2 MiB of payload.
	secondIdent := vorbisIdent()
	secondComment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("B"), 40)...)
	secondSetup := bytes.Repeat([]byte{0x06}, 600)
	secondAudio := bytes.Repeat([]byte{0x99}, 1024*1024) // ~2 MiB
	secondStream := oggStream(2, 67890, secondIdent, secondComment, secondSetup, secondAudio)

	// Build the chained fixture twice with different first-stream comment-header sizes.
	chainedShort := append(oggStream(1, 12345, ident,
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...), setup, audio), secondStream...)
	chainedLong := append(oggStream(1, 12345, ident,
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("C"), 3000)...), setup, audio), secondStream...)

	// Hash the chained stream with a counting reader.
	counter := &countingReaderAt{r: bytes.NewReader(chainedShort)}
	hashShort, err := Reader(counter, int64(len(chainedShort)), "song.ogg")
	if err != nil {
		t.Fatalf("Reader(short): %v", err)
	}

	hashLong, err := Reader(bytes.NewReader(chainedLong), int64(len(chainedLong)), "song.ogg")
	if err != nil {
		t.Fatalf("Reader(long): %v", err)
	}

	// Total bytes read must be well under 1 MiB. Without the EOS check, this
	// lands north of 2.3 MiB because the entire second stream is read.
	if counter.bytes >= 1024*1024 {
		t.Fatalf("read %d bytes, want < 1 MiB (the walk must stop at the first stream's EOS)", counter.bytes)
	}

	// The chained file's hash must survive a retag of its first stream.
	if hashShort != hashLong {
		t.Fatalf("retag changed the hash: %q vs %q (only the first stream's comment header differs)", hashShort, hashLong)
	}
}
