package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type RuntimeSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRuntime(t *testing.T) {
	suite.Run(t, new(RuntimeSuite))
}

func (s *RuntimeSuite) SetupTest() {
	s.ctx = context.Background()
}

type mockSubsystem struct {
	name       string
	startErr   error
	stopErr    error
	startCalls int
	stopCalls  int
}

func (m *mockSubsystem) Name() string { return m.name }

func (m *mockSubsystem) Start(_ context.Context) error {
	m.startCalls++
	return m.startErr
}

func (m *mockSubsystem) Stop(_ context.Context) error {
	m.stopCalls++
	return m.stopErr
}

func (s *RuntimeSuite) TestParseAppMode() {
	scenarios := []struct {
		name    string
		input   string
		want    AppMode
		wantErr bool
		errMsg  string
	}{
		{
			name:  "deve retornar ModeServer para entrada server",
			input: "server",
			want:  ModeServer,
		},
		{
			name:  "deve retornar ModeWorker para entrada worker",
			input: "worker",
			want:  ModeWorker,
		},
		{
			name:    "deve retornar erro quando entrada vazia",
			input:   "",
			wantErr: true,
			errMsg:  "app mode inválido",
		},
		{
			name:    "deve retornar erro quando entrada em maiúsculas",
			input:   "Server",
			wantErr: true,
			errMsg:  "app mode inválido",
		},
		{
			name:    "deve retornar erro para modo all removido na v6",
			input:   "all",
			wantErr: true,
			errMsg:  "app mode inválido",
		},
		{
			name:    "deve retornar erro para migrate que não é AppMode",
			input:   "migrate",
			wantErr: true,
			errMsg:  "app mode inválido",
		},
		{
			name:    "deve retornar erro para entrada com caractere especial",
			input:   "server!",
			wantErr: true,
			errMsg:  "app mode inválido",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			got, err := ParseAppMode(sc.input)
			if sc.wantErr {
				s.Error(err)
				s.ErrorContains(err, sc.errMsg)
				return
			}
			s.NoError(err)
			s.Equal(sc.want, got)
		})
	}
}

func (s *RuntimeSuite) TestAppModeString() {
	s.Equal("server", ModeServer.String())
	s.Equal("worker", ModeWorker.String())
}

func (s *RuntimeSuite) TestAppRun() {
	scenarios := []struct {
		name        string
		subsystems  []*mockSubsystem
		wantErr     bool
		errContains string
	}{
		{
			name:       "deve executar com sucesso sem subsistemas",
			subsystems: nil,
			wantErr:    false,
		},
		{
			name: "deve executar dois subsistemas saudáveis com sucesso",
			subsystems: []*mockSubsystem{
				{name: "alpha"},
				{name: "beta"},
			},
			wantErr: false,
		},
		{
			name: "deve retornar erro quando segundo subsistema falha ao iniciar",
			subsystems: []*mockSubsystem{
				{name: "alpha"},
				{name: "beta", startErr: errors.New("beta boom")},
			},
			wantErr:     true,
			errContains: "iniciando subsistema \"beta\"",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			subs := make([]Subsystem, len(sc.subsystems))
			for i, sub := range sc.subsystems {
				subs[i] = sub
			}
			a := &app{mode: ModeServer, subsystems: subs}

			err := a.Run(s.ctx)
			if sc.wantErr {
				s.Error(err)
				s.ErrorContains(err, sc.errContains)
				return
			}
			s.NoError(err)
			for _, sub := range sc.subsystems {
				s.Equal(1, sub.startCalls)
			}
		})
	}
}

func (s *RuntimeSuite) TestAppShutdownOrdemInversa() {
	scenarios := []struct {
		name string
	}{
		{name: "deve chamar Stop em cada subsistema exatamente uma vez"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			alpha := &mockSubsystem{name: "alpha"}
			beta := &mockSubsystem{name: "beta"}
			gamma := &mockSubsystem{name: "gamma"}

			a := &app{
				mode:       ModeServer,
				subsystems: []Subsystem{alpha, beta, gamma},
			}

			s.Require().NoError(a.Run(s.ctx))
			s.Require().NoError(a.Shutdown(s.ctx))

			s.Equal(1, alpha.stopCalls)
			s.Equal(1, beta.stopCalls)
			s.Equal(1, gamma.stopCalls)
		})
	}
}

func (s *RuntimeSuite) TestAppShutdownAcumulaErros() {
	scenarios := []struct {
		name string
	}{
		{
			name: "deve acumular erros de shutdown e chamar Stop em todos os subsistemas",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			alpha := &mockSubsystem{name: "alpha", stopErr: errors.New("alpha stop fail")}
			beta := &mockSubsystem{name: "beta", stopErr: errors.New("beta stop fail")}

			a := &app{
				mode:       ModeServer,
				subsystems: []Subsystem{alpha, beta},
			}

			err := a.Shutdown(s.ctx)
			s.Error(err)
			s.ErrorContains(err, "erros durante shutdown")
			s.Equal(1, alpha.stopCalls)
			s.Equal(1, beta.stopCalls)
		})
	}
}

func (s *RuntimeSuite) TestBootstrap() {
	scenarios := []struct {
		name        string
		mode        AppMode
		bootstrapOK bool
		runOK       bool
	}{
		{
			name:        "modo worker deve inicializar e executar sem subsistemas",
			mode:        ModeWorker,
			bootstrapOK: true,
			runOK:       true,
		},
		{
			name:        "modo server deve inicializar sem falha no bootstrap",
			mode:        ModeServer,
			bootstrapOK: true,
			// Run falha porque não há DB disponível em teste unitário (esperado).
			runOK: false,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			cfg := minimalValidConfig()
			got, err := Bootstrap(cfg, sc.mode)
			if !sc.bootstrapOK {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.NotNil(got)

			ctx, cancel := context.WithCancel(s.ctx)
			cancel()

			runErr := got.Run(ctx)
			if sc.runOK {
				s.NoError(runErr)
			} else {
				s.Error(runErr)
			}

			_ = got.Shutdown(context.Background())
		})
	}
}

func (s *RuntimeSuite) TestBuildSubsystemsDefaultBranch() {
	scenarios := []struct {
		name string
	}{
		{name: "deve retornar lista vazia para AppMode inválido"},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			cfg := minimalValidConfig()
			subs, err := buildSubsystems(cfg, AppMode("invalid"), Foundation{})
			s.NoError(err)
			s.Empty(subs)
		})
	}
}

func minimalValidConfig() *configs.Config {
	return &configs.Config{
		AppConfig: configs.AppConfig{
			Environment: "local",
		},
		HTTPConfig: configs.HTTPConfig{
			Port:           8080,
			ServiceNameAPI: "mecontrola-test",
		},
		DBConfig: configs.DBConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "testuser",
			Password: "testpassword",
			Name:     "testdb",
			SSLMode:  "disable",
		},
		O11yConfig: configs.O11yConfig{
			TraceSampleRate: 1.0,
			LogLevel:        "info",
			LogFormat:       "json",
		},
	}
}
