package migrate_test

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/cmd/migrate"
)

type MigrateSuite struct {
	suite.Suite
}

func TestMigrateSuite(t *testing.T) {
	suite.Run(t, new(MigrateSuite))
}

func (s *MigrateSuite) SetupTest() {}

func (s *MigrateSuite) TestCommandFactories() {
	type args struct {
		build func() *cobra.Command
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(cmd *cobra.Command)
	}{
		{
			name: "deve criar comando migrate",
			args: args{
				build: func() *cobra.Command {
					return migrate.New()
				},
			},
			setup: func() {},
			expect: func(cmd *cobra.Command) {
				s.Equal("migrate", cmd.Use)
				s.NotNil(cmd.RunE)
			},
		},
		{
			name: "deve criar comando migrate-down com flag steps",
			args: args{
				build: func() *cobra.Command {
					return migrate.NewDown()
				},
			},
			setup: func() {},
			expect: func(cmd *cobra.Command) {
				s.Equal("migrate-down", cmd.Use)
				s.NotNil(cmd.RunE)
				s.NotNil(cmd.Flags().Lookup("steps"))
			},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		s.Run(scenario.name, func() {
			scenario.setup()

			command := scenario.args.build()
			scenario.expect(command)
		})
	}
}

func (s *MigrateSuite) TestRunDown() {
	type args struct {
		steps int
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func(err error)
	}{
		{
			name:  "deve rejeitar zero steps antes do bootstrap",
			args:  args{steps: 0},
			setup: func() {},
			expect: func(err error) {
				s.Require().Error(err)
				s.ErrorContains(err, "steps deve ser != 0")
			},
		},
	}

	for _, scenario := range scenarios {
		scenario := scenario
		s.Run(scenario.name, func() {
			scenario.setup()

			runDown := migrate.RunDown
			err := runDown(io.Discard, scenario.args.steps)
			scenario.expect(err)
		})
	}
}
