package configs_test

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Testes de Validate()
// ---------------------------------------------------------------------------

func TestValidate_TableDriven(t *testing.T) {
	t.Parallel()

	prod := "production"
	local := "local"

	tests := []struct {
		name    string
		cfg     *configs.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "config válida local",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local, AppMode: "server"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "qualquer", User: "user"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 1.0},
			},
			wantErr: false,
		},
		{
			name: "environment inválido",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "dev", AppMode: "server"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "ENVIRONMENT inválido",
		},
		{
			name: "environment vazio",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "", AppMode: "server"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "ENVIRONMENT inválido",
		},
		{
			name: "port zero",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 0},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "PORT inválido",
		},
		{
			name: "port acima de 65535",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 65536},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "PORT inválido",
		},
		{
			name: "port mínimo válido (1)",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 1},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: false,
		},
		{
			name: "port máximo válido (65535)",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 65535},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: false,
		},
		{
			name: "TraceSampleRate negativo",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: -0.1},
			},
			wantErr: true,
			errMsg:  "OTEL_TRACE_SAMPLE_RATE inválido",
		},
		{
			name: "TraceSampleRate acima de 1",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 1.1},
			},
			wantErr: true,
			errMsg:  "OTEL_TRACE_SAMPLE_RATE inválido",
		},
		{
			name: "TraceSampleRate zero válido",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.0},
			},
			wantErr: false,
		},
		{
			name: "TraceSampleRate um válido",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 1.0},
			},
			wantErr: false,
		},
		{
			name: "production com senha curta (< 16 chars)",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: prod},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "short", User: "dbuser"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
			},
			wantErr: true,
			errMsg:  "DB_PASSWORD deve ter ao menos 16 caracteres",
		},
		{
			name: "production com senha com exatamente 16 chars (válida)",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: prod},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "exactly16chars!!", User: "dbuser"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
			},
			wantErr: false,
		},
		{
			name: "production com placeholder CHANGE_ME na senha",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: prod},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "CHANGE_ME_USE_STRONG_PASSWORD", User: "dbuser"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
			},
			wantErr: true,
			errMsg:  "placeholder inseguro",
		},
		{
			name: "production com your_secret_key na senha",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: prod},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "your_secret_key", User: "dbuser"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
			},
			wantErr: true,
			errMsg:  "placeholder inseguro",
		},
		{
			name: "production com financial@password na senha",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: prod},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "financial@password", User: "dbuser"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
			},
			wantErr: true,
			errMsg:  "placeholder inseguro",
		},
		{
			name: "staging não exige senha longa",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "staging"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "short", User: "user"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: false,
		},
		{
			name: "múltiplos erros acumulados",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "invalid"},
				HTTPConfig: configs.HTTPConfig{Port: 0},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 2.0},
			},
			wantErr: true,
			errMsg:  "ENVIRONMENT inválido",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.cfg.Validate()

			if tt.wantErr {
				require.Error(t, err, "esperava erro mas Validate() retornou nil")
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg,
						"mensagem de erro não contém %q", tt.errMsg)
				}
			} else {
				assert.NoError(t, err, "não esperava erro mas Validate() retornou: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Testes de LoadConfig()
// ---------------------------------------------------------------------------

func TestLoadConfig_DevWithValidEnvFile(t *testing.T) {
	t.Parallel()

	cfg, err := configs.LoadConfig("./testdata/valid")

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "local", cfg.AppConfig.Environment)
	assert.Equal(t, 8080, cfg.HTTPConfig.Port)
	assert.Equal(t, "localhost", cfg.DBConfig.Host)
	assert.Equal(t, 1.0, cfg.O11yConfig.TraceSampleRate)
}

func TestLoadConfig_DevWithoutEnvFile_ReturnsError(t *testing.T) {
	t.Parallel()

	// Diretório sem .env — simula dev sem arquivo
	dir := t.TempDir()

	cfg, err := configs.LoadConfig(dir)

	require.Error(t, err, "esperava erro ao ausência de .env em dev")
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), ".env não encontrado")
}

func TestLoadConfig_ProdWithoutEnvFile_UsesEnvVars(t *testing.T) {
	// Configura env vars que simulam Fly secrets
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("PORT", "8080")
	t.Setenv("DB_HOST", "db.fly.internal")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_USER", "mecontrola")
	t.Setenv("DB_PASSWORD", "productionStrongPassword123!")
	t.Setenv("DB_NAME", "mecontrola_db")
	t.Setenv("DB_SSL_MODE", "require")
	t.Setenv("OTEL_TRACE_SAMPLE_RATE", "0.2")
	t.Setenv("SERVICE_NAME_API", "mecontrola-api")

	// Diretório sem .env — em production deve funcionar via env vars
	dir := t.TempDir()

	cfg, err := configs.LoadConfig(dir)

	require.NoError(t, err, "production sem .env deve usar env vars")
	require.NotNil(t, cfg)
	assert.Equal(t, "production", cfg.AppConfig.Environment)
	assert.Equal(t, "db.fly.internal", cfg.DBConfig.Host)
}

func TestLoadConfig_InsecureProd_ReturnsError(t *testing.T) {
	t.Parallel()

	cfg, err := configs.LoadConfig("./testdata/insecure-prod")

	require.Error(t, err, "esperava erro com placeholder inseguro em production")
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "placeholder inseguro")
}

// ---------------------------------------------------------------------------
// Testes de DSN() e SafeDSN()
// ---------------------------------------------------------------------------

func TestSafeDSN_NeverContainsPassword(t *testing.T) {
	t.Parallel()

	// 5 senhas distintas geradas aleatoriamente
	passwords := generateRandomPasswords(5)

	for i, pwd := range passwords {
		t.Run(fmt.Sprintf("senha_%d", i+1), func(t *testing.T) {
			t.Parallel()

			db := &configs.DBConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: pwd,
				Name:     "dbname",
				SSLMode:  "disable",
			}

			safeDSN := db.SafeDSN()

			assert.NotContains(t, safeDSN, pwd,
				"SafeDSN() não deve conter a senha real %q", pwd)
			assert.Contains(t, safeDSN, "***",
				"SafeDSN() deve mascarar a senha com ***")
		})
	}
}

func TestDSN_ContainsPassword(t *testing.T) {
	t.Parallel()

	db := &configs.DBConfig{
		Host:     "db.example.com",
		Port:     5432,
		User:     "mecontrola",
		Password: "supersecretpassword",
		Name:     "mecontrola_db",
		SSLMode:  "require",
	}

	dsn := db.DSN()

	assert.Contains(t, dsn, "supersecretpassword", "DSN() deve conter a senha")
	assert.Contains(t, dsn, "postgres://mecontrola:supersecretpassword@db.example.com:5432/mecontrola_db?sslmode=require")
}

func TestSafeDSN_Format(t *testing.T) {
	t.Parallel()

	db := &configs.DBConfig{
		Host:     "db.example.com",
		Port:     5432,
		User:     "mecontrola",
		Password: "anypassword",
		Name:     "mecontrola_db",
		SSLMode:  "require",
	}

	safeDSN := db.SafeDSN()

	expected := "postgres://mecontrola:***@db.example.com:5432/mecontrola_db?sslmode=require"
	assert.Equal(t, expected, safeDSN, "SafeDSN() deve seguir o formato esperado")
}

// ---------------------------------------------------------------------------
// Testes de InsecurePlaceholders
// ---------------------------------------------------------------------------

func TestInsecurePlaceholders_NotEmpty(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, configs.InsecurePlaceholders,
		"InsecurePlaceholders não deve estar vazio")
}

func TestInsecurePlaceholders_ContainsKnownValues(t *testing.T) {
	t.Parallel()

	known := []string{
		"CHANGE_ME_USE_STRONG_PASSWORD",
		"CHANGE_ME_GENERATE_SECURE_SECRET_KEY_MIN_64_CHARS",
		"your_secret_key",
		"financial@password",
	}

	for _, v := range known {
		assert.Contains(t, configs.InsecurePlaceholders, v,
			"InsecurePlaceholders deve conter %q", v)
	}
}

func TestValidate_ProductionInsecureUser(t *testing.T) {
	t.Parallel()

	cfg := &configs.Config{
		AppConfig:  configs.AppConfig{Environment: "production"},
		HTTPConfig: configs.HTTPConfig{Port: 8080},
		DBConfig: configs.DBConfig{
			// Senha longa válida mas usuário com placeholder inseguro
			Password: "productionStrongPassword123!",
			User:     "your_secret_key",
		},
		O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DB_USER contém placeholder inseguro")
}

func TestValidate_ProductionInsecureOTLPHeaders(t *testing.T) {
	t.Parallel()

	cfg := &configs.Config{
		AppConfig:  configs.AppConfig{Environment: "production"},
		HTTPConfig: configs.HTTPConfig{Port: 8080},
		DBConfig: configs.DBConfig{
			Password: "productionStrongPassword123!",
			User:     "mecontrola",
		},
		O11yConfig: configs.O11yConfig{
			TraceSampleRate: 0.2,
			OTLPHeaders:     "CHANGE_ME_GENERATE_SECURE_SECRET_KEY_MIN_64_CHARS",
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OTEL_EXPORTER_OTLP_HEADERS contém placeholder inseguro")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const passwordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

func generateRandomPasswords(n int) []string {
	passwords := make([]string, n)
	for i := range passwords {
		length := 20 + rand.Intn(20) //nolint:gosec // não é criptografia
		var sb strings.Builder
		for j := 0; j < length; j++ {
			sb.WriteByte(passwordChars[rand.Intn(len(passwordChars))]) //nolint:gosec
		}
		passwords[i] = sb.String()
	}
	return passwords
}

// ---------------------------------------------------------------------------
// Testes de configuração com variáveis de ambiente
// ---------------------------------------------------------------------------

func TestLoadConfig_InvalidPortFromEnv(t *testing.T) {
	t.Setenv("ENVIRONMENT", "local")
	t.Setenv("PORT", "99999")

	// Cria um .env mínimo no diretório temp para passar a detecção do arquivo
	dir := t.TempDir()
	envContent := "ENVIRONMENT=local\nPORT=99999\nOTEL_TRACE_SAMPLE_RATE=1.0\n"
	err := os.WriteFile(dir+"/.env", []byte(envContent), 0600)
	require.NoError(t, err)

	cfg, err := configs.LoadConfig(dir)

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "PORT inválido")
}

func TestLoadConfig_InvalidTraceSampleRateFromFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	envContent := "ENVIRONMENT=local\nPORT=8080\nOTEL_TRACE_SAMPLE_RATE=2.5\n"
	err := os.WriteFile(dir+"/.env", []byte(envContent), 0600)
	require.NoError(t, err)

	cfg, err := configs.LoadConfig(dir)

	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "OTEL_TRACE_SAMPLE_RATE inválido")
}
