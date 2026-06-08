package sqlnull_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/sqlnull"
)

type SqlnullSuite struct {
	suite.Suite
}

func TestSqlnullSuite(t *testing.T) {
	suite.Run(t, new(SqlnullSuite))
}

func (s *SqlnullSuite) SetupTest() {}

func (s *SqlnullSuite) TestStr() {
	type args struct {
		input string
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(any)
	}{
		{
			name:  "deve retornar nil para string vazia",
			args:  args{input: ""},
			setup: func() {},
			expect: func(result any) {
				s.Nil(result)
			},
		},
		{
			name:  "deve preservar string nao vazia",
			args:  args{input: "alice@example.com"},
			setup: func() {},
			expect: func(result any) {
				s.Equal("alice@example.com", result)
			},
		},
		{
			name:  "deve preservar espaco em branco",
			args:  args{input: " "},
			setup: func() {},
			expect: func(result any) {
				s.Equal(" ", result)
			},
		},
		{
			name:  "deve preservar string multibyte",
			args:  args{input: "ÁlÍce"},
			setup: func() {},
			expect: func(result any) {
				s.Equal("ÁlÍce", result)
			},
		},
		{
			name:  "deve retornar string concreta",
			args:  args{input: "v"},
			setup: func() {},
			expect: func(result any) {
				value, ok := result.(string)
				s.True(ok)
				s.Equal("v", value)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := sqlnull.Str
			result := sut(scenario.args.input)

			scenario.expect(result)
		})
	}
}

func (s *SqlnullSuite) TestTime() {
	type args struct {
		input time.Time
	}

	now := time.Date(2026, time.June, 5, 12, 0, 0, 0, time.UTC)

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(any)
	}{
		{
			name:  "deve retornar nil para time zero",
			args:  args{input: time.Time{}},
			setup: func() {},
			expect: func(result any) {
				s.Nil(result)
			},
		},
		{
			name:  "deve preservar epoch unix",
			args:  args{input: time.Unix(0, 0).UTC()},
			setup: func() {},
			expect: func(result any) {
				s.Equal(time.Unix(0, 0).UTC(), result)
			},
		},
		{
			name:  "deve preservar time concreto",
			args:  args{input: now},
			setup: func() {},
			expect: func(result any) {
				s.Equal(now, result)
			},
		},
		{
			name:  "deve retornar time concreto",
			args:  args{input: now},
			setup: func() {},
			expect: func(result any) {
				value, ok := result.(time.Time)
				s.True(ok)
				s.True(value.Equal(now))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			sut := sqlnull.Time
			result := sut(scenario.args.input)

			scenario.expect(result)
		})
	}
}
