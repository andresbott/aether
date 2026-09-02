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

	r := bytes.NewReader(page)
	got, err := readOggPage(r, 0, int64(len(page)))
	if err != nil {
		t.Fatalf("readOggPage: %v", err)
	}
	// Parsing the header must not have touched the payload — that is what lets a
	// walk skip the pages it will not digest.
	if got.payload != nil {
		t.Fatalf("readOggPage loaded %d payload bytes; the payload is readPayload's job", len(got.payload))
	}
	if got.payloadLen != len(payload) {
		t.Fatalf("payloadLen = %d, want %d (the lacing table states it exactly)", got.payloadLen, len(payload))
	}
	if err := got.readPayload(r); err != nil {
		t.Fatalf("readPayload: %v", err)
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
	// Audio bytes can spell "OggS" by accident. This asserts only that such a file
	// still survives a retag end to end — nothing more: both fixtures plant the
	// same candidate at the same distance from EOF, so whichever candidate the
	// granule search picks, it picks the same one in both and they compare equal.
	// It therefore does *not* discharge the ends-at-EOF requirement and still
	// passes with that guard deleted;
	// TestOggLastGranuleIgnoresAPlantedHeaderThatDoesNotEndAtEOF is the white-box
	// assertion that does.
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

	got, ok := oggLastGranule(bytes.NewReader(data), int64(len(data)), 1)
	if !ok {
		t.Fatal("oggLastGranule found no qualifying page, but the stream's last page ends at EOF")
	}
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

func TestFileOggWithoutAPageEndingAtEOFIsDeclined(t *testing.T) {
	// `cat a.ogg b.ogg` is a valid Ogg join, and a truncated file or a file
	// carrying any trailer looks the same from here: the target stream has no page
	// ending exactly at end of file, so the granule that serves as the digest's
	// length component cannot be located. Without a length component every such
	// file collapses into one equivalence class keyed on an audio prefix alone —
	// two chains sharing only their first stream hash equal, and so does the first
	// stream plus a handful of trailing bytes. That is a collision class, which is
	// the one failure this hash must not have, so the file is declined instead.
	intro := oggStream(1, 12345, vorbisIdent(),
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...),
		bytes.Repeat([]byte{0x05}, 600), bytes.Repeat([]byte{0x42, 0x43}, 4000))
	episodeA := oggStream(2, 111, vorbisIdent(),
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("B"), 40)...),
		bytes.Repeat([]byte{0x06}, 600), bytes.Repeat([]byte{0x71}, 50<<10))
	episodeB := oggStream(3, 222, vorbisIdent(),
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("C"), 40)...),
		bytes.Repeat([]byte{0x07}, 600), bytes.Repeat([]byte{0x72}, 90<<10))

	chain := func(tail []byte) []byte {
		return append(append([]byte{}, intro...), tail...)
	}
	cases := []struct {
		name string
		data []byte
	}{
		{"chain-a.ogg", chain(episodeA)},
		{"chain-b.ogg", chain(episodeB)},
		{"trailer.ogg", chain(bytes.Repeat([]byte{0x00}, 128))},
		// A file cut short mid-page. The granule lookup runs before the walk, so
		// this is declined rather than reported as a parse error.
		{"truncated.ogg", intro[:len(intro)-64]},
	}

	for _, c := range cases {
		got, err := File(writeFixture(t, c.name, c.data))
		if !errors.Is(err, ErrUnsupported) {
			t.Errorf("File(%s) = %q, err = %v, want ErrUnsupported", c.name, got, err)
		}
	}
}

func TestReaderSkipsTheBytesOfALargeOggMetadataPacket(t *testing.T) {
	// A METADATA_BLOCK_PICTURE inside a Vorbis comment is how Ogg carries cover
	// art, so a multi-MiB metadata packet is ordinary rather than adversarial.
	// Nothing on those pages reaches the digest, so nothing on them may be read: a
	// page's lacing table states its exact payload length without a byte of the
	// payload being touched, and the packet counter says whether any of its
	// segments will be consumed. A fully skipped page must therefore cost its
	// header and segment table and nothing more.
	ident := vorbisIdent()
	comment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("P"), 6<<20)...)
	setup := bytes.Repeat([]byte{0x05}, 900)
	audio := bytes.Repeat([]byte{0x42, 0x43}, 150<<10) // 300 KiB

	data := oggStream(1, 7654321, ident, comment, setup, audio)
	counter := &countingReaderAt{r: bytes.NewReader(data)}
	if _, err := Reader(counter, int64(len(data)), "cover.ogg"); err != nil {
		t.Fatalf("Reader: %v", err)
	}

	// The honest budget is one bounded tail read for the granule, the 256 KiB the
	// digest is capped at (rounded up to a page boundary), and one header plus
	// segment table per page walked. Reading the payload of every page instead
	// costs the whole file.
	const budget = oggTailWindow + 2*maxHashBytes
	t.Logf("read %d bytes of a %d-byte file (budget %d)", counter.bytes, len(data), budget)
	if counter.bytes > budget {
		t.Errorf("read %d bytes of a %d-byte file, want <= %d — the metadata packet's pages must be skipped unread",
			counter.bytes, len(data), budget)
	}
}

func TestFileOggLostSyncIsAnError(t *testing.T) {
	// Build a stream but remove the EOS flag from its last page so the walk
	// continues past it and runs into the garbage spliced in behind it.
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
	data = append(data, bytes.Repeat([]byte{0xFF}, 64)...) // garbage mid-file
	// Close with a real page on the target serial so the granule lookup succeeds
	// and the walk actually runs; without one the file has no page ending at EOF
	// and is declined before a single page is walked.
	data = append(data, oggPageBytes(1, 99, 100, oggFlagEOS, [][]byte{{0x11}})...)

	_, err := File(writeFixture(t, "garbage.ogg", data))
	if err == nil {
		t.Fatal("garbage where a page header should be must be an error")
	}
	if errors.Is(err, ErrUnsupported) {
		t.Fatalf("got ErrUnsupported, want a parse error: %v", err)
	}
}

// oggPinnedCases are the fixtures whose emitted digest is pinned as a literal.
// They are built by one helper so the bytes cannot drift between the run that
// captured the literals and the runs that check them.
//
// The set covers one fixture per supported mapping (Vorbis and Opus), a stream
// whose metadata packet is large enough that its pages are skipped without their
// payload being read, and a stream whose audio overruns the 256 KiB cap part-way
// through a page — the fiddliest branch of the segment walk.
func oggPinnedCases() []struct {
	name string
	data []byte
	want string
} {
	ident := vorbisIdent()
	comment := append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...)
	setup := bytes.Repeat([]byte{0x05}, 900)

	return []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "vorbis.ogg",
			data: oggStream(0x0BADF00D, 7654321, ident, comment, setup,
				bytes.Repeat([]byte{0x42, 0x43}, 4000)),
			want: "oggfnv1a64:899f9d6c56d39569",
		},
		{
			// Opus has exactly two metadata packets where Vorbis has three, so a
			// different set of packets reaches the digest.
			name: "opus.opus",
			data: oggStream(42, 999, opusHead(),
				append([]byte("OpusTags"), bytes.Repeat([]byte("A"), 30)...),
				bytes.Repeat([]byte{0x77, 0x88}, 4000)),
			want: "oggfnv1a64:1ba0032c6c2a0aa7",
		},
		{
			// A 3 MiB comment header: METADATA_BLOCK_PICTURE is how Ogg carries
			// cover art. Every page of it is skipped, and the digest must not
			// notice that its payload was never read.
			name: "big-picture.ogg",
			data: oggStream(7, 4242, ident,
				append([]byte("\x03vorbis"), bytes.Repeat([]byte("P"), 3<<20)...),
				setup, bytes.Repeat([]byte{0x11, 0x22, 0x33}, 40000)),
			want: "oggfnv1a64:caeea76eb042109b",
		},
		{
			// 400 KiB of audio: the digest stops at the 256 KiB cap in the middle
			// of a page's segment run.
			name: "capped.ogg",
			data: oggStream(11, 555555, ident, comment, setup,
				bytes.Repeat([]byte{0xA5, 0x5A}, 200*1024)),
			want: "oggfnv1a64:ad817d0d90d89b92",
		},
	}
}

func TestFileOggPinnedDigests(t *testing.T) {
	for _, c := range oggPinnedCases() {
		got, err := File(writeFixture(t, c.name, c.data))
		if err != nil {
			t.Fatalf("File(%s): %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("File(%s) = %q, want %q", c.name, got, c.want)
		}
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

func TestReaderDeclinesAChainedFileCheaply(t *testing.T) {
	// A chained file — a Vorbis stream with ~200 KiB of audio followed by a second
	// logical stream carrying ~2 MiB — has no page of the target serial ending at
	// end of file, so its granule cannot be found and the file is declined. That
	// costs nothing worth measuring: the granule lookup runs *before* the page
	// walk, so the whole attempt is one page read plus one bounded tail read, and
	// not a byte of the 2 MiB second stream is touched.
	ident := vorbisIdent()
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x42, 0x43}, 100<<10) // ~200 KiB

	second := oggStream(2, 67890, vorbisIdent(),
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("B"), 40)...),
		bytes.Repeat([]byte{0x06}, 600), bytes.Repeat([]byte{0x99}, 1<<20))
	chained := append(oggStream(1, 12345, ident,
		append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...), setup, audio), second...)

	counter := &countingReaderAt{r: bytes.NewReader(chained)}
	got, err := Reader(counter, int64(len(chained)), "song.ogg")
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Reader(chained) = %q, err = %v, want ErrUnsupported", got, err)
	}

	// One page header plus its short first packet, and one oggTailWindow read.
	const budget = oggTailWindow + 1<<10
	t.Logf("read %d bytes of a %d-byte file (budget %d)", counter.bytes, len(chained), budget)
	if counter.bytes > budget {
		t.Fatalf("declining read %d bytes, want <= %d — the granule lookup must come before the walk",
			counter.bytes, budget)
	}
}

func TestFileOggSameSerialChainStopsAtTheEndOfStreamPage(t *testing.T) {
	// `cat a.ogg a.ogg` reuses one serial number across both links, which the Ogg
	// spec forbids but a shell does not. It is the shape that keeps the walk's
	// end-of-stream break alive: the second copy's final page carries the target
	// serial and ends at end of file, so the granule lookup succeeds and the file
	// is walked rather than declined. Breaking at the first copy's EOS page is what
	// keeps the second copy's comment header — a retaggable region — out of the
	// digest. Delete the break and this fixture stops surviving a retag.
	ident := vorbisIdent()
	setup := bytes.Repeat([]byte{0x05}, 600)
	audio := bytes.Repeat([]byte{0x42, 0x43}, 4000)
	tail := bytes.Repeat([]byte{0x99}, 1<<20) // makes "not walked" measurable

	link := func(comment []byte) []byte {
		return oggStream(1, 12345, ident, comment, setup, append(audio, tail...))
	}
	short := link(append([]byte("\x03vorbis"), bytes.Repeat([]byte("A"), 40)...))
	long := link(append([]byte("\x03vorbis"), bytes.Repeat([]byte("C"), 3000)...))

	doubled := append(append([]byte{}, short...), short...)
	doubledRetagged := append(append([]byte{}, long...), long...)

	counter := &countingReaderAt{r: bytes.NewReader(doubled)}
	a, err := Reader(counter, int64(len(doubled)), "doubled.ogg")
	if err != nil {
		t.Fatalf("Reader(doubled): %v", err)
	}
	b, err := Reader(bytes.NewReader(doubledRetagged), int64(len(doubledRetagged)), "doubled.ogg")
	if err != nil {
		t.Fatalf("Reader(doubled, retagged): %v", err)
	}
	if a != b {
		t.Fatalf("retag changed the hash of a same-serial chain: %q vs %q — the end-of-stream break must keep the second link out of the digest", a, b)
	}

	// The digest caps at 256 KiB and the break stops the walk at the first link's
	// last page, so the second link is never read.
	const budget = oggTailWindow + 2*maxHashBytes
	t.Logf("read %d bytes of a %d-byte file (budget %d)", counter.bytes, len(doubled), budget)
	if counter.bytes > budget {
		t.Fatalf("read %d bytes, want <= %d (the walk must stop at the first link's EOS page)", counter.bytes, budget)
	}
}
