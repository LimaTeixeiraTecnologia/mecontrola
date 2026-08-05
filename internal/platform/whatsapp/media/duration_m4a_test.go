package media_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/media"
)

type ISOBMFFSuite struct {
	suite.Suite
}

func TestISOBMFFSuite(t *testing.T) {
	suite.Run(t, new(ISOBMFFSuite))
}

func (s *ISOBMFFSuite) TestDetermineDuration_M4A_VariacoesDeMime() {
	fixture := buildISOBMFFFixture(48000, 225024)

	scenarios := []string{"audio/mp4", "audio/m4a", "audio/x-m4a"}
	for _, mime := range scenarios {
		s.Run(mime, func() {
			duration, err := media.DetermineDuration(mime, fixture)
			s.Require().NoError(err)
			s.Greater(duration.Seconds(), 4.6)
			s.Less(duration.Seconds(), 4.8)
		})
	}
}

func (s *ISOBMFFSuite) TestDetermineDuration_M4A_SemMoov() {
	fixture := isoBox("ftyp", []byte("M4A mp42isom"))

	_, err := media.DetermineDuration("audio/mp4", fixture)
	s.Error(err)
	s.True(errors.Is(err, media.ErrDurationUndetermined))
}

func (s *ISOBMFFSuite) TestDetermineDuration_M4A_TimescaleZero() {
	fixture := buildISOBMFFFixture(0, 4693)

	_, err := media.DetermineDuration("audio/mp4", fixture)
	s.Error(err)
	s.True(errors.Is(err, media.ErrDurationUndetermined))
}
