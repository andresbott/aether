// libs/audiohash/ogg.go
package audiohash

import (
	"encoding/binary"
	"fmt"
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
)

// oggPage is one parsed Ogg page: its total on-disk length (so a walk can
// advance), the logical stream it belongs to, its lacing values and its payload.
// The CRC is deliberately not read, let alone checked — a retag changes every
// downstream page's CRC, so a hash that depended on it would not survive one.
type oggPage struct {
	total   int64
	serial  uint32
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
		lacing:  lacing,
		payload: payload,
	}, nil
}
