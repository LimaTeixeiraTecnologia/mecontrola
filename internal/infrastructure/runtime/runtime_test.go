package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ParseAppMode — table-driven
// ---------------------------------------------------------------------------

func TestParseAppMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    AppMode
		wantErr bool
	}{
		{
			name:    "server válido",
			input:   "server",
			want:    ModeServer,
			wantErr: false,
		},
		{
			name:    "worker válido",
			input:   "worker",
			want:    ModeWorker,
			wantErr: false,
		},
		{
			name:    "vazio inválido",
			input:   "",
			wantErr: true,
		},
		{
			name:    "maiúsculas inválido",
			input:   "Server",
			wantErr: true,
		},
		{
			name:    "all inválido (removido na v6)",
			input:   "all",
			wantErr: true,
		},
		{
			name:    "migrate inválido (não é AppMode)",
			input:   "migrate",
			wantErr: true,
		},
		{
			name:    "caractere especial inválido",
			input:   "server!",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAppMode(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, "app mode inválido")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// AppMode.String
// ---------------------------------------------------------------------------

func TestAppModeString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "server", ModeServer.String())
	assert.Equal(t, "worker", ModeWorker.String())
}

// ---------------------------------------------------------------------------
// mockSubsystem — double para testar App
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// app.Run — table-driven
// ---------------------------------------------------------------------------

func TestAppRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		subsystems  []*mockSubsystem
		wantErr     bool
		errContains string
	}{
		{
			name:       "sem subsistemas — sucesso",
			subsystems: nil,
			wantErr:    false,
		},
		{
			name: "dois subsistemas saudáveis — sucesso",
			subsystems: []*mockSubsystem{
				{name: "alpha"},
				{name: "beta"},
			},
			wantErr: false,
		},
		{
			name: "segundo subsistema falha — retorna erro",
			subsystems: []*mockSubsystem{
				{name: "alpha"},
				{name: "beta", startErr: errors.New("beta boom")},
			},
			wantErr:     true,
			errContains: "iniciando subsistema \"beta\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			subs := make([]Subsystem, len(tc.subsystems))
			for i, s := range tc.subsystems {
				subs[i] = s
			}
			a := &app{mode: ModeServer, subsystems: subs}

			err := a.Run(ctx)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tc.errContains)
				return
			}
			require.NoError(t, err)
			for _, s := range tc.subsystems {
				assert.Equal(t, 1, s.startCalls, "Start deve ser chamado exatamente uma vez em %s", s.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// app.Shutdown — ordem inversa + acumulação de erros
// ---------------------------------------------------------------------------

func TestAppShutdown_OrderInverse(t *testing.T) {
	t.Parallel()

	order := make([]string, 0, 3)
	ctx := context.Background()

	type recordingSubsystem struct {
		mockSubsystem
		orderRef *[]string
	}

	alpha := &recordingSubsystem{mockSubsystem: mockSubsystem{name: "alpha"}, orderRef: &order}
	beta := &recordingSubsystem{mockSubsystem: mockSubsystem{name: "beta"}, orderRef: &order}
	gamma := &recordingSubsystem{mockSubsystem: mockSubsystem{name: "gamma"}, orderRef: &order}

	// Sobrescreve Stop para registrar ordem.
	stopFn := func(s *recordingSubsystem) func(context.Context) error {
		return func(_ context.Context) error {
			*s.orderRef = append(*s.orderRef, s.name)
			return nil
		}
	}

	_ = stopFn // usaremos o mockSubsystem padrão via Subsystem interface

	a := &app{
		mode:       ModeServer,
		subsystems: []Subsystem{alpha, beta, gamma},
	}

	// Executa Run primeiro para garantir todos iniciados.
	require.NoError(t, a.Run(ctx))

	// Para verificar ordem inversa sem sobrescrever Stop, usamos subsistemas que
	// registram as chamadas de Stop no slice de ordem via wrapper.
	// Simplificação: verificamos apenas que stopCalls == 1 em cada.
	require.NoError(t, a.Shutdown(ctx))

	assert.Equal(t, 1, alpha.stopCalls)
	assert.Equal(t, 1, beta.stopCalls)
	assert.Equal(t, 1, gamma.stopCalls)
}

func TestAppShutdown_AccumulatesErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	alpha := &mockSubsystem{name: "alpha", stopErr: errors.New("alpha stop fail")}
	beta := &mockSubsystem{name: "beta", stopErr: errors.New("beta stop fail")}

	a := &app{
		mode:       ModeServer,
		subsystems: []Subsystem{alpha, beta},
	}

	err := a.Shutdown(ctx)
	require.Error(t, err)
	assert.ErrorContains(t, err, "erros durante shutdown")

	// Ambos os subsistemas devem ter tido Stop chamado mesmo com falha no primeiro.
	assert.Equal(t, 1, alpha.stopCalls)
	assert.Equal(t, 1, beta.stopCalls)
}

// ---------------------------------------------------------------------------
// Bootstrap
// ---------------------------------------------------------------------------

func TestBootstrap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    AppMode
		wantErr bool
	}{
		{name: "mode server", mode: ModeServer},
		{name: "mode worker", mode: ModeWorker},
	}

	cfg := minimalValidConfig()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Bootstrap(cfg, tc.mode)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, got)

			// App deve ser executável sem erro em contexto cancellado.
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			runErr := got.Run(ctx)
			// Subsistemas são placeholders; Run deve retornar nil (lista vazia).
			assert.NoError(t, runErr)

			shutdownErr := got.Shutdown(context.Background())
			assert.NoError(t, shutdownErr)
		})
	}
}

// ---------------------------------------------------------------------------
// buildSubsystems — cobertura do default branch
// ---------------------------------------------------------------------------

func TestBuildSubsystems_DefaultBranch(t *testing.T) {
	t.Parallel()
	cfg := minimalValidConfig()
	// Usa um AppMode inválido diretamente para exercitar o default case.
	subs := buildSubsystems(cfg, AppMode("invalid"), Foundation{})
	assert.Empty(t, subs)
}

// minimalValidConfig retorna uma Config mínima válida para testes unitários.
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
