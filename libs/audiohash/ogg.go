package audiohash

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"
)

const (
	// oggHeaderFixed is the size of an Ogg page header up to and including the
	// segment count, before the variable-length segment table.
	oggHeaderFixed = 27
	// oggTailWindow bounds the backward search for the stream's last page, whose
	// granule position stands in for the payload length. The final page sits
	// within one page of EOF in any well-formed stream; the window is generous.
	oggTailWindow = 64 << 10
	// oggFlagEOS marks the last page of a logical stream.
	oggFlagEOS = 0x04
)

// oggPage is one parsed Ogg page *header*: its total on-disk length (so a walk
// can advance), where its payload lives and how long it is, the logical stream it
// belongs to, its header-type flags and its lacing values.
//
// The payload is deliberately not part of parsing. readOggPage reads the header
// and the segment table only, and readPayload fetches the bytes — because the
// lacing table states the payload's exact length without a byte of it being
// touched, so a walk can decide from the header alone whether the page has
// anything the digest will consume. A comment header carrying a cover image is
// megabytes that reach the digest through nothing; reading them would make an
// ordinary Ogg file cost its full size to hash. See oggHash.
//
// The CRC is deliberately not read, let alone checked — a retag changes every
// downstream page's CRC, so a hash that depended on it would not survive one.
type oggPage struct {
	total      int64
	body       int64 // file offset of the payload, just past the segment table
	payloadLen int
	serial     uint32
	flags      byte // the page's header-type flags. Only the end-of-stream bit is consulted, and only to stop the walk — a page's flags never enter the digest.
	lacing     []byte
	payload    []byte // nil until readPayload has been called
}

// readOggPage parses the header and segment table of the page starting at off,
// without reading its payload. It validates the capture pattern and the version,
// so a file that is not Ogg — or a walk that has lost sync — fails loudly instead
// of being hashed over garbage. It also rejects a page whose payload would run
// past end of file, so the bounds readPayload relies on are established here even
// for a page whose payload is never fetched.
func readOggPage(f io.ReaderAt, off, size int64) (oggPage, error) {
	head := make([]byte, oggHeaderFixed)
	if _, err := f.ReadAt(head, off); err != nil {
		return oggPage{}, fmt.Errorf("audiohash: read Ogg page header at %d: %w", off, err)
	}
	if string(head[0:4]) != "OggS" {
		return oggPage{}, fmt.Errorf("audiohash: lost Ogg page sync at offset %d", off)
	}
	if head[4] != 0 {
		return oggPage{}, fmt.Errorf("audiohash: unsupported Ogg page version %d", head[4])
	}

	lacing := make([]byte, int(head[26]))
	if len(lacing) > 0 {
		if _, err := f.ReadAt(lacing, off+oggHeaderFixed); err != nil {
			return oggPage{}, fmt.Errorf("audiohash: read Ogg segment table at %d: %w", off, err)
		}
	}
	payloadLen := 0
	for _, lace := range lacing {
		payloadLen += int(lace)
	}

	bodyAt := off + oggHeaderFixed + int64(len(lacing))
	if bodyAt+int64(payloadLen) > size {
		return oggPage{}, fmt.Errorf("audiohash: Ogg page at %d runs past end of file", off)
	}

	return oggPage{
		total:      oggHeaderFixed + int64(len(lacing)) + int64(payloadLen),
		body:       bodyAt,
		payloadLen: payloadLen,
		serial:     binary.LittleEndian.Uint32(head[14:18]),
		flags:      head[5],
		lacing:     lacing,
	}, nil
}

// readPayload reads the page's payload into p.payload. Only pages with at least
// one segment the digest will consume are worth this read; readOggPage already
// checked that the payload lies inside the file.
func (p *oggPage) readPayload(f io.ReaderAt) error {
	p.payload = make([]byte, p.payloadLen)
	if p.payloadLen == 0 {
		// A zero-length ReadAt at end of file is allowed to report io.EOF, so the
		// empty slice is the whole answer here.
		return nil
	}
	if _, err := f.ReadAt(p.payload, p.body); err != nil {
		return fmt.Errorf("audiohash: read Ogg page payload at %d: %w", p.body, err)
	}
	return nil
}

// digestsAudio reports whether any segment on the page belongs to an audio packet
// — that is, whether the page's payload is needed at all — given the number of
// packets already completed before the page and how many leading packets are
// metadata.
//
// Only the last segment need be tested. A lacing value below 255 terminates a
// packet, so the completed-packet count rises monotonically across the page:
// if the last segment is still inside the header packets, every earlier one is
// too. Counting the terminators among all *but* the last segment gives the count
// in force when that last segment is reached.
func (p oggPage) digestsAudio(packets, headerPackets int) bool {
	if len(p.lacing) == 0 {
		return false
	}
	for _, lace := range p.lacing[:len(p.lacing)-1] {
		if lace < 255 {
			packets++
		}
	}
	return packets >= headerPackets
}

// hashSegments feeds the page's audio segments to h, stopping at the maxHashBytes
// cap, and returns the completed-packet count and the digested-byte total as they
// stand after the page. Metadata segments are skipped but still counted, because
// it is the packet count that tells the walk where the audio starts.
//
// It must only be called on a page whose payload has been read when
// digestsAudio reports true for the same (packets, headerPackets). When that
// reports false no segment on the page clears the header-packet threshold, so the
// payload is never indexed and a nil payload is harmless.
func (p oggPage) hashSegments(h io.Writer, packets, headerPackets, written int) (int, int) {
	pos := 0
	for _, lace := range p.lacing {
		next := pos + int(lace)
		if packets >= headerPackets && written < maxHashBytes {
			seg := p.payload[pos:next]
			if room := maxHashBytes - written; len(seg) > room {
				seg = seg[:room]
			}
			n, _ := h.Write(seg)
			written += n
		}
		pos = next
		if lace < 255 {
			packets++ // a lacing value below 255 terminates a packet
		}
	}
	return packets, written
}

// oggHeaderPackets reports how many leading packets of a logical stream are
// metadata rather than audio, identified from the first bytes of its first
// packet.
//
// Both supported mappings answer two, for different reasons. Opus has exactly
// two metadata packets, OpusHead and OpusTags. Vorbis has three —
// identification, comment and setup — and skipping only the first two is
// deliberate: the setup header holds the codebooks, which a retag never
// rewrites, so leaving it in the digest is free discriminating power.
//
// Anything else is reported unsupported rather than hashed on a guess. Ogg FLAC
// in particular gives each metadata block its own packet, so a fixed skip count
// would leave an embedded picture inside the digest and the value would not
// survive an art edit.
func oggHeaderPackets(firstPacket []byte) (int, error) {
	switch {
	case bytes.HasPrefix(firstPacket, []byte("\x01vorbis")),
		bytes.HasPrefix(firstPacket, []byte("OpusHead")):
		return 2, nil
	default:
		return 0, ErrUnsupported
	}
}

// oggLastGranule returns the granule position of the last page belonging to
// serial — the stream's total decoded sample count, which stands in for the
// payload length the other formats mix into their digest — and whether such a
// page was found at all.
//
// A candidate must match the capture pattern, the version and the serial *and*
// end exactly at end of file. That last requirement is what makes the search
// safe: audio bytes can spell "OggS" by accident, and since a retag shifts every
// absolute offset, accepting such a match could pick a different one before and
// after the retag — exactly the instability this hash exists to avoid.
//
// Nothing qualifies when the target stream has no page ending at end of file: a
// chained file (`cat a.ogg b.ogg` is a valid Ogg join), a truncated file, a file
// carrying any trailer. Reporting false there rather than substituting a
// deterministic 0 is the point of the second return value. A stand-in would be
// stable, but stability is not the test that matters — every file in that shape
// would share the same missing length component and so collapse into one
// equivalence class keyed on an audio prefix alone. Two podcast episodes sharing
// an intro chain, or the same stream with a byte of padding, would hash equal,
// and an equal hash can move one track's play history and playlist memberships
// onto another. oggHash therefore declines such a file, the same conservatism the
// package already applies to an Ogg file carrying an unrecognised mapping.
func oggLastGranule(f io.ReaderAt, size int64, serial uint32) (uint64, bool) {
	window := int64(oggTailWindow)
	if window > size {
		window = size
	}
	buf := make([]byte, window)
	if _, err := f.ReadAt(buf, size-window); err != nil {
		return 0, false
	}
	for i := len(buf) - oggHeaderFixed; i >= 0; i-- {
		if string(buf[i:i+4]) != "OggS" || buf[i+4] != 0 {
			continue
		}
		if binary.LittleEndian.Uint32(buf[i+14:i+18]) != serial {
			continue
		}
		nSegments := int(buf[i+26])
		if i+oggHeaderFixed+nSegments > len(buf) {
			continue
		}
		payloadLen := 0
		for _, lace := range buf[i+oggHeaderFixed : i+oggHeaderFixed+nSegments] {
			payloadLen += int(lace)
		}
		if i+oggHeaderFixed+nSegments+payloadLen != len(buf) {
			continue // not the page that ends the file
		}
		return binary.LittleEndian.Uint64(buf[i+6 : i+14]), true
	}
	return 0, false
}

// oggHash hashes the audio packets of an Ogg stream — Vorbis or Opus — with the
// stream's total granule position as the length component.
//
// It cannot hash raw file bytes. A retag rewrites the comment header packet,
// which changes its length, which shifts every following page boundary and
// renumbers the pages — each page header carries a sequence number and a CRC.
// Reassembling packet payloads and hashing those is what makes the value survive
// a retag, and a remux with it: remuxing changes page boundaries, never packet
// contents.
//
// The exact payload length the other formats mix in is not cheaply available
// here: no header states it, and totalling it would mean walking every page
// header in the file, defeating the maxHashBytes read bound. The last page's
// granule position is the better substitute anyway — tag-invariant,
// remux-invariant, and one bounded read from the end of the file. A file whose
// target stream has no page ending at end of file has no such value, and is
// declined rather than hashed without a length component; see oggLastGranule for
// why that is a collision class and not merely a weaker digest.
//
// That lookup deliberately comes *before* the page walk, so a declined file costs
// one page read plus one bounded tail read and nothing else — a chained or
// truncated file is never walked.
//
// Read cost is bounded by the digest, not by the file: the walk fetches a page's
// payload only when at least one of its segments will be hashed, so a multi-MiB
// cover image in the comment header costs its page headers and lacing tables
// alone. A file whose target stream contributes no audio bytes at all is declined
// too — a digest over the granule alone would be one more shared value.
//
// The walk stops when the target stream's end-of-stream page is reached. That is
// unreachable for a well-formed file, whose EOS page is its last page, but it
// still earns its place: `cat a.ogg a.ogg` reuses the same serial number in both
// links, so the second copy's final page ends at EOF and satisfies the granule
// lookup. Without this break the walk would run on into the second copy and
// digest its comment header, destroying retag invariance.
func oggHash(f io.ReaderAt, size int64) (string, error) {
	first, err := readOggPage(f, 0, size)
	if err != nil {
		return "", err
	}
	// The first page's payload is always needed: its leading magic is what names
	// the mapping.
	if err := first.readPayload(f); err != nil {
		return "", err
	}
	serial := first.serial
	headerPackets, err := oggHeaderPackets(first.payload)
	if err != nil {
		return "", err
	}

	lastGranule, ok := oggLastGranule(f, size, serial)
	if !ok {
		return "", ErrUnsupported
	}

	h := fnv.New64a()
	var granule [8]byte
	binary.BigEndian.PutUint64(granule[:], lastGranule)
	_, _ = h.Write(granule[:])

	written := 0
	packets := 0
	for off := int64(0); off+oggHeaderFixed <= size && written < maxHashBytes; {
		page, err := readOggPage(f, off, size)
		if err != nil {
			return "", err
		}
		off += page.total
		if page.serial != serial {
			continue // another logical stream, multiplexed or chained in
		}
		if page.digestsAudio(packets, headerPackets) {
			if err := page.readPayload(f); err != nil {
				return "", err
			}
		}
		packets, written = page.hashSegments(h, packets, headerPackets, written)
		if page.flags&oggFlagEOS != 0 {
			break // the target stream is over; anything after belongs to another link
		}
	}
	if written == 0 {
		return "", ErrUnsupported
	}
	return fmt.Sprintf("oggfnv1a64:%016x", h.Sum64()), nil
}
