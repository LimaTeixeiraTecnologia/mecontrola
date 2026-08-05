package media_test

import (
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/media"
)

type DurationSuite struct {
	suite.Suite
}

func TestDurationSuite(t *testing.T) {
	suite.Run(t, new(DurationSuite))
}

func (s *DurationSuite) TestDetermineDuration_MimeNaoSuportado() {
	_, err := media.DetermineDuration("audio/mpeg", []byte("qualquer"))
	s.Error(err)
	s.True(errors.Is(err, media.ErrUnsupportedMimeType))
}

func (s *DurationSuite) TestDetermineDuration_OggOpus_Sucesso() {
	preSkip := uint16(312)
	sampleCount := int64(96000)
	fixture := buildOggOpusFixture(preSkip, int64(preSkip)+sampleCount)

	duration, err := media.DetermineDuration("audio/ogg; codecs=opus", fixture)
	s.Require().NoError(err)
	s.InDelta(2*time.Second, duration, float64(5*time.Millisecond))
}

func (s *DurationSuite) TestDetermineDuration_OggOpus_Corrompido() {
	fixture := []byte("nao e um arquivo ogg valido")

	_, err := media.DetermineDuration("audio/ogg", fixture)
	s.Error(err)
	s.True(errors.Is(err, media.ErrDurationUndetermined))
}

func (s *DurationSuite) TestDetermineDuration_OggOpus_SemOpusHead() {
	page := buildOggPage(0x02, 0, []byte("payload sem opus head"))

	_, err := media.DetermineDuration("audio/ogg", page)
	s.Error(err)
	s.True(errors.Is(err, media.ErrDurationUndetermined))
}

func (s *DurationSuite) TestDetermineDuration_M4A_ISOBMFF_Sucesso() {
	fixture := buildISOBMFFFixture(1000, 4693)

	duration, err := media.DetermineDuration("audio/mp4", fixture)
	s.Require().NoError(err)
	s.InDelta(4693*time.Millisecond, duration, float64(2*time.Millisecond))
}

func (s *DurationSuite) TestDetermineDuration_AAC_ADTS_Sucesso() {
	fixture := buildADTSFixture(3, 1)

	duration, err := media.DetermineDuration("audio/aac", fixture)
	s.Require().NoError(err)
	s.InDelta(float64(1024)/float64(48000)*float64(time.Second), duration, float64(time.Millisecond))
}

func (s *DurationSuite) TestDetermineDuration_ADTS_MPEG2_TambemSincroniza() {
	mpeg4 := buildADTSFixture(3, 1)
	mpeg2 := append([]byte(nil), mpeg4...)
	mpeg2[1] = 0xF9

	duration, err := media.DetermineDuration("audio/aac", mpeg2)
	s.Require().NoError(err, "ADTS MPEG-2 (syncword 0xFFF9) deve sincronizar como o MPEG-4 (0xFFF1)")

	expected, err4 := media.DetermineDuration("audio/aac", mpeg4)
	s.Require().NoError(err4)
	s.Equal(expected, duration, "o bit de versao MPEG nao pode alterar a duracao extraida")
}

func (s *DurationSuite) TestDetermineDuration_M4A_Indeterminavel() {
	fixture := []byte("nao e um container mp4 valido")

	_, err := media.DetermineDuration("audio/mp4", fixture)
	s.Error(err)
	s.True(errors.Is(err, media.ErrDurationUndetermined))
}

func (s *DurationSuite) TestCheckMaxDuration() {
	s.NoError(media.CheckMaxDuration(30*time.Second, 60*time.Second))
	s.NoError(media.CheckMaxDuration(60*time.Second, 60*time.Second))

	err := media.CheckMaxDuration(61*time.Second, 60*time.Second)
	s.Error(err)
	s.True(errors.Is(err, media.ErrAudioDurationExceeded))
}

func buildOggOpusFixture(preSkip uint16, finalGranule int64) []byte {
	headPayload := make([]byte, 19)
	copy(headPayload[0:8], []byte("OpusHead"))
	headPayload[8] = 1
	headPayload[9] = 2
	binary.LittleEndian.PutUint16(headPayload[10:12], preSkip)
	binary.LittleEndian.PutUint32(headPayload[12:16], 48000)
	binary.LittleEndian.PutUint16(headPayload[16:18], 0)
	headPayload[18] = 0

	idPage := buildOggPage(0x02, 0, headPayload)
	dataPage := buildOggPage(0x04, finalGranule, []byte("opus-frame-bytes"))

	fixture := make([]byte, 0, len(idPage)+len(dataPage))
	fixture = append(fixture, idPage...)
	fixture = append(fixture, dataPage...)
	return fixture
}

func buildOggPage(headerType byte, granule int64, payload []byte) []byte {
	segments := lacingValues(len(payload))

	page := make([]byte, 0, 27+len(segments)+len(payload))
	page = append(page, []byte("OggS")...)
	page = append(page, 0)
	page = append(page, headerType)

	granuleBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(granuleBytes, uint64(granule))
	page = append(page, granuleBytes...)

	page = append(page, 0, 0, 0, 0)
	page = append(page, 0, 0, 0, 0)
	page = append(page, 0, 0, 0, 0)
	page = append(page, byte(len(segments)))
	page = append(page, segments...)
	page = append(page, payload...)
	return page
}

func lacingValues(payloadLen int) []byte {
	values := make([]byte, 0, payloadLen/255+1)
	remaining := payloadLen
	for remaining >= 255 {
		values = append(values, 255)
		remaining -= 255
	}
	values = append(values, byte(remaining))
	return values
}

func buildISOBMFFFixture(timescale, duration uint32) []byte {
	mvhd := make([]byte, 0, 100)
	mvhd = append(mvhd, 0, 0, 0, 0)
	mvhd = append(mvhd, 0, 0, 0, 0)
	mvhd = append(mvhd, 0, 0, 0, 0)

	timescaleBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(timescaleBytes, timescale)
	mvhd = append(mvhd, timescaleBytes...)

	durationBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(durationBytes, duration)
	mvhd = append(mvhd, durationBytes...)

	mvhd = append(mvhd, make([]byte, 4+2+2+8+36+24+4)...)

	mvhdBox := isoBox("mvhd", mvhd)
	moovBox := isoBox("moov", mvhdBox)
	ftypBox := isoBox("ftyp", []byte("M4A mp42isom"))

	fixture := make([]byte, 0, len(ftypBox)+len(moovBox))
	fixture = append(fixture, ftypBox...)
	fixture = append(fixture, moovBox...)
	return fixture
}

func isoBox(boxType string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	box := make([]byte, 0, size)

	sizeBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(sizeBytes, size)
	box = append(box, sizeBytes...)
	box = append(box, []byte(boxType)...)
	box = append(box, payload...)
	return box
}

func buildADTSFixture(samplingFreqIndex byte, rawDataBlocks byte) []byte {
	frame := make([]byte, 7)
	frame[0] = 0xFF
	frame[1] = 0xF1
	frame[2] = 0x40 | (samplingFreqIndex << 2)
	frameLength := uint16(len(frame))
	frame[3] = 0x0C | byte(frameLength>>11)
	frame[4] = byte(frameLength >> 3)
	frame[5] = byte(frameLength<<5) | 0x1F
	frame[6] = 0xFC | (rawDataBlocks - 1)
	return frame
}
