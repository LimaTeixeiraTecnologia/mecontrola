package media

import (
	"encoding/binary"
	"time"
)

const (
	oggCapturePattern   = "OggS"
	oggPageHeaderMinLen = 27
	opusHeadMagic       = "OpusHead"
	opusClockRate       = 48000
)

type oggPage struct {
	granulePosition int64
	continuedFrom   bool
	payload         []byte
}

func oggOpusDuration(data []byte) (time.Duration, error) {
	pages, err := parseOggPages(data)
	if err != nil || len(pages) == 0 {
		return 0, ErrDurationUndetermined
	}

	preSkip, ok := parseOpusHead(pages[0].payload)
	if !ok {
		return 0, ErrDurationUndetermined
	}

	lastGranule := int64(-1)
	for _, page := range pages {
		if page.granulePosition >= 0 {
			lastGranule = page.granulePosition
		}
	}
	if lastGranule < 0 {
		return 0, ErrDurationUndetermined
	}

	samples := lastGranule - int64(preSkip)
	if samples <= 0 {
		return 0, ErrDurationUndetermined
	}

	seconds := float64(samples) / float64(opusClockRate)
	return time.Duration(seconds * float64(time.Second)), nil
}

func parseOggPages(data []byte) ([]oggPage, error) {
	pages := make([]oggPage, 0, 8)
	offset := 0

	for offset+oggPageHeaderMinLen <= len(data) {
		if string(data[offset:offset+4]) != oggCapturePattern {
			return pages, ErrDurationUndetermined
		}

		headerType := data[offset+5]
		granulePosition := int64(binary.LittleEndian.Uint64(data[offset+6 : offset+14]))
		segmentCount := int(data[offset+26])

		segmentTableStart := offset + oggPageHeaderMinLen
		segmentTableEnd := segmentTableStart + segmentCount
		if segmentTableEnd > len(data) {
			return pages, ErrDurationUndetermined
		}

		payloadLen := 0
		for _, segment := range data[segmentTableStart:segmentTableEnd] {
			payloadLen += int(segment)
		}

		payloadStart := segmentTableEnd
		payloadEnd := payloadStart + payloadLen
		if payloadEnd > len(data) {
			return pages, ErrDurationUndetermined
		}

		pages = append(pages, oggPage{
			granulePosition: granulePosition,
			continuedFrom:   headerType&0x01 != 0,
			payload:         data[payloadStart:payloadEnd],
		})

		offset = payloadEnd
	}

	if len(pages) == 0 {
		return pages, ErrDurationUndetermined
	}

	return pages, nil
}

func parseOpusHead(payload []byte) (preSkip uint16, ok bool) {
	if len(payload) < 12 || string(payload[0:8]) != opusHeadMagic {
		return 0, false
	}
	return binary.LittleEndian.Uint16(payload[10:12]), true
}
