package initialize

import (
	"encoding/binary"
)

// OGG CRC32 lookup table (polynomial 0x04C11DB7)
var oggCRC32Table [256]uint32

func init() {
	for i := 0; i < 256; i++ {
		crc := uint32(i) << 24
		for j := 0; j < 8; j++ {
			if crc&0x80000000 != 0 {
				crc = (crc << 1) ^ 0x04C11DB7
			} else {
				crc <<= 1
			}
		}
		oggCRC32Table[i] = crc
	}
}

func oggCRC32(data []byte) uint32 {
	crc := uint32(0)
	for _, b := range data {
		crc = (crc << 8) ^ oggCRC32Table[byte(crc>>24)^b]
	}
	return crc
}

type oggWriter struct {
	buf         []byte
	pageCounter uint32
	granulePos  uint64
	serial      uint32
}

func newOGGWriter(serial uint32) *oggWriter {
	w := &oggWriter{serial: serial}
	w.writeOpusHead()
	w.writeOpusTags()
	return w
}

func (w *oggWriter) writePage(headerType byte, granulePos uint64, data []byte) {
	// Calculate segment table
	segCount := 0
	segSizes := []byte{}
	remaining := len(data)
	for remaining > 255 {
		segSizes = append(segSizes, 255)
		remaining -= 255
		segCount++
	}
	segSizes = append(segSizes, byte(remaining))
	segCount++

	// Build page header (without checksum)
	pageSize := 27 + segCount + len(data)
	page := make([]byte, 27, pageSize)
	copy(page[0:4], "OggS") // capture pattern
	page[4] = 0              // version
	page[5] = headerType     // header type
	binary.LittleEndian.PutUint64(page[6:14], granulePos)
	binary.LittleEndian.PutUint32(page[14:18], w.serial)
	binary.LittleEndian.PutUint32(page[18:22], w.pageCounter)
	// page[22:26] = checksum (fill later)
	page[26] = byte(segCount)

	// Append segment table
	page = append(page, segSizes...)

	// Append data
	page = append(page, data...)

	// Compute CRC over entire page (with checksum bytes zeroed)
	page[22] = 0
	page[23] = 0
	page[24] = 0
	page[25] = 0
	crc := oggCRC32(page)
	page[22] = byte(crc)
	page[23] = byte(crc >> 8)
	page[24] = byte(crc >> 16)
	page[25] = byte(crc >> 24)

	w.buf = append(w.buf, page...)
	w.pageCounter++
}

func (w *oggWriter) writeOpusHead() {
	data := []byte{
		'O', 'p', 'u', 's', 'H', 'e', 'a', 'd', // magic
		1,       // version
		2,       // channels
		0x38, 1, // pre-skip (312)
		0x80, 0xBB, 0, 0, // sample rate (48000)
		0, 0, // output gain
		0, // channel mapping
	}
	w.writePage(0x02, 0, data) // BOS
}

func (w *oggWriter) writeOpusTags() {
	data := []byte{
		'O', 'p', 'u', 's', 'T', 'a', 'g', 's',
		0, 0, 0, 0, // vendor length
		0, 0, 0, 0, // comment count
	}
	w.writePage(0x00, 0, data)
}

func (w *oggWriter) writeOpusFrame(payload []byte) {
	if len(payload) == 0 {
		return
	}
	w.granulePos += 960 // 20ms at 48kHz
	w.writePage(0x00, w.granulePos, payload)
}

func (w *oggWriter) bytes() []byte {
	return w.buf
}
