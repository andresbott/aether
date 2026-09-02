package audiohash

import (
	"bytes"
	"encoding/binary"
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
