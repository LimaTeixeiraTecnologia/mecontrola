package id

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
)

type UUIDGeneratorSuite struct {
	suite.Suite

	generator UUIDGenerator
}

func TestUUIDGeneratorSuite(t *testing.T) {
	suite.Run(t, new(UUIDGeneratorSuite))
}

func (s *UUIDGeneratorSuite) SetupTest() {
	s.generator = NewUUIDGenerator()
}

func (s *UUIDGeneratorSuite) TestNewID_ReturnsValidUUIDv4() {
	id := s.generator.NewID()

	parsed, err := uuid.Parse(id)
	s.Require().NoError(err, "deve ser um UUID válido")
	s.Equal(uuid.Version(4), parsed.Version(), "deve ser UUID v4")
}

func (s *UUIDGeneratorSuite) TestNewID_100CallsProduceDistinctIDs() {
	const count = 100
	seen := make(map[string]struct{}, count)

	for range count {
		id := s.generator.NewID()
		_, exists := seen[id]
		s.Require().False(exists, "ID duplicado detectado: %s", id)
		seen[id] = struct{}{}
	}
}

func (s *UUIDGeneratorSuite) TestNewID_IsNonEmpty() {
	id := s.generator.NewID()
	s.NotEmpty(id)
}
