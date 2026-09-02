// libs/audiohash/ogg.go
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

// oggPage is one parsed Ogg page: its total on-disk length (so a walk can
// advance), the logical stream it belongs to, its header-type flags, its granule
// position, its lacing values and its payload. The CRC is deliberately not read,
// let alone checked — a retag changes every downstream page's CRC, so a hash
// that depended on it would not survive one.
type oggPage struct {
	total   int64
	serial  uint32
	flags   byte // the page's header-type flags. Only the end-of-stream bit is consulted, and only to stop the walk — a page's flags never enter the digest.
	granule uint64
	lacing  []byte
	payload []byte
}

// readOggPage parses the page starting at off. It validates the capture pattern
// and the version, so a file that is not Ogg — or a walk that has lost sync —
// fails loudly instead of being hashed over garbage.
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
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := f.ReadAt(payload, bodyAt); err != nil {
			return oggPage{}, fmt.Errorf("audiohash: read Ogg page payload at %d: %w", off, err)
		}
	}

	return oggPage{
		total:   oggHeaderFixed + int64(len(lacing)) + int64(payloadLen),
		serial:  binary.LittleEndian.Uint32(head[14:18]),
		flags:   head[5],
		granule: binary.LittleEndian.Uint64(head[6:14]),
		lacing:  lacing,
		payload: payload,
	}, nil
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
// payload length the other formats mix into their digest.
//
// A candidate must match the capture pattern, the version and the serial *and*
// end exactly at end of file. That last requirement is what makes the search
// safe: audio bytes can spell "OggS" by accident, and since a retag shifts every
// absolute offset, accepting such a match could pick a different one before and
// after the retag — exactly the instability this hash exists to avoid.
//
// A chained file's target stream has no page ending at end of file — a second
// logical stream follows it — so its granule comes back 0. This is deterministic
// and therefore retag-stable, which is the property that matters. The granule is
// a discriminator rather than a requirement, so losing it costs a little digest
// strength and nothing else.
//
// It returns 0 rather than an error when nothing qualifies. 0 is a deterministic
// stand-in that keeps the file's hash stable instead of failing it.
func oggLastGranule(f io.ReaderAt, size int64, serial uint32) uint64 {
	window := int64(oggTailWindow)
	if window > size {
		window = size
	}
	buf := make([]byte, window)
	if _, err := f.ReadAt(buf, size-window); err != nil {
		return 0
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
		return binary.LittleEndian.Uint64(buf[i+6 : i+14])
	}
	return 0
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
// remux-invariant, and one bounded read from the end of the file.
//
// The walk stops when the target stream's end-of-stream page is reached, so a
// chained file's trailing streams are not read. A stream that omits its EOS
// flag falls back to walking to EOF, which fails safe.
func oggHash(f io.ReaderAt, size int64) (string, error) {
	first, err := readOggPage(f, 0, size)
	if err != nil {
		return "", err
	}
	serial := first.serial
	headerPackets, err := oggHeaderPackets(first.payload)
	if err != nil {
		return "", err
	}

	h := fnv.New64a()
	var granule [8]byte
	binary.BigEndian.PutUint64(granule[:], oggLastGranule(f, size, serial))
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
		pos := 0
		for _, lace := range page.lacing {
			seg := page.payload[pos : pos+int(lace)]
			pos += int(lace)
			if packets >= headerPackets && written < maxHashBytes {
				if room := maxHashBytes - written; len(seg) > room {
					seg = seg[:room]
				}
				n, _ := h.Write(seg)
				written += n
			}
			if lace < 255 {
				packets++ // a lacing value below 255 terminates a packet
			}
		}
		if page.flags&oggFlagEOS != 0 {
			break // the target stream is over; anything after belongs to another stream
		}
	}
	return fmt.Sprintf("oggfnv1a64:%016x", h.Sum64()), nil
}
