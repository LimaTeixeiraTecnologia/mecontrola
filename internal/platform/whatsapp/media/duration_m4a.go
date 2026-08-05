package media

import (
	"encoding/binary"
	"time"
)

const (
	isoBoxHeaderLen     = 8
	isoBoxHeaderLen64   = 16
	adtsSyncMask        = 0xFFF0
	adtsSyncValue       = 0xFFF0
	adtsHeaderLen       = 7
	adtsSamplesPerBlock = 1024
)

var adtsSampleRates = [16]int{
	96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050,
	16000, 12000, 11025, 8000, 7350, 0, 0, 0,
}

func m4aAACDuration(data []byte) (time.Duration, error) {
	if duration, ok := isoBMFFDuration(data); ok {
		return duration, nil
	}
	if duration, ok := adtsDuration(data); ok {
		return duration, nil
	}
	return 0, ErrDurationUndetermined
}

func isoBMFFDuration(data []byte) (time.Duration, bool) {
	moovPayload, ok := findISOBox(data, "moov")
	if !ok {
		return 0, false
	}

	mvhdPayload, ok := findISOBox(moovPayload, "mvhd")
	if !ok {
		return 0, false
	}

	if len(mvhdPayload) < 1 {
		return 0, false
	}

	version := mvhdPayload[0]

	var timescale uint32
	var durationUnits uint64

	switch version {
	case 1:
		if len(mvhdPayload) < 4+8+8+4+8 {
			return 0, false
		}
		timescale = binary.BigEndian.Uint32(mvhdPayload[20:24])
		durationUnits = binary.BigEndian.Uint64(mvhdPayload[24:32])
	default:
		if len(mvhdPayload) < 4+4+4+4+4 {
			return 0, false
		}
		timescale = binary.BigEndian.Uint32(mvhdPayload[12:16])
		durationUnits = uint64(binary.BigEndian.Uint32(mvhdPayload[16:20]))
	}

	if timescale == 0 || durationUnits == 0 {
		return 0, false
	}

	seconds := float64(durationUnits) / float64(timescale)
	return time.Duration(seconds * float64(time.Second)), true
}

func findISOBox(data []byte, boxType string) ([]byte, bool) {
	offset := 0

	for offset+isoBoxHeaderLen <= len(data) {
		size := int64(binary.BigEndian.Uint32(data[offset : offset+4]))
		typ := string(data[offset+4 : offset+8])

		headerLen := isoBoxHeaderLen
		switch size {
		case 1:
			if offset+isoBoxHeaderLen64 > len(data) {
				return nil, false
			}
			size = int64(binary.BigEndian.Uint64(data[offset+8 : offset+16]))
			headerLen = isoBoxHeaderLen64
		case 0:
			size = int64(len(data) - offset)
		}

		if size < int64(headerLen) || offset+int(size) > len(data) {
			return nil, false
		}

		payload := data[offset+headerLen : offset+int(size)]
		if typ == boxType {
			return payload, true
		}

		offset += int(size)
	}

	return nil, false
}

func adtsDuration(data []byte) (time.Duration, bool) {
	offset := 0
	totalSamples := int64(0)
	frameCount := 0
	sampleRate := 0

	for offset+adtsHeaderLen <= len(data) {
		syncWord := uint16(data[offset])<<8 | uint16(data[offset+1])
		if syncWord&adtsSyncMask != adtsSyncValue {
			offset++
			continue
		}

		samplingFreqIndex := (data[offset+2] >> 2) & 0x0F
		rate := adtsSampleRates[samplingFreqIndex]
		if rate == 0 {
			offset++
			continue
		}

		frameLength := int(data[offset+3]&0x03)<<11 | int(data[offset+4])<<3 | int(data[offset+5]>>5)
		if frameLength < adtsHeaderLen || offset+frameLength > len(data) {
			break
		}

		rawBlocks := int(data[offset+6]&0x03) + 1
		totalSamples += int64(rawBlocks * adtsSamplesPerBlock)
		frameCount++
		sampleRate = rate

		offset += frameLength
	}

	if frameCount == 0 || sampleRate == 0 {
		return 0, false
	}

	seconds := float64(totalSamples) / float64(sampleRate)
	return time.Duration(seconds * float64(time.Second)), true
}
