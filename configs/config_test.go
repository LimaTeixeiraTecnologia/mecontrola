package configs_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
)

type ConfigSuite struct {
	suite.Suite
	ctx context.Context
}

func TestConfig(t *testing.T) {
	suite.Run(t, new(ConfigSuite))
}

func (s *ConfigSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ConfigSuite) TestValidate() {
	prod := "production"
	local := "local"

	scenarios := []struct {
		name    string
		cfg     *configs.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "deve validar config local com sucesso",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local, AppMode: "server"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "qualquer", User: "user"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 1.0},
			},
			wantErr: false,
		},
		{
			name: "deve retornar erro quando environment inválido",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "dev", AppMode: "server"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "ENVIRONMENT inválido",
		},
		{
			name: "deve retornar erro quando environment vazio",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "", AppMode: "server"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "ENVIRONMENT inválido",
		},
		{
			name: "deve retornar erro quando port zero",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 0},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "PORT inválido",
		},
		{
			name: "deve retornar erro quando port acima de 65535",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 65536},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: true,
			errMsg:  "PORT inválido",
		},
		{
			name: "deve aceitar port mínimo válido igual a 1",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 1},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: false,
		},
		{
			name: "deve aceitar port máximo válido igual a 65535",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 65535},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: false,
		},
		{
			name: "deve retornar erro quando TraceSampleRate negativo",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: -0.1},
			},
			wantErr: true,
			errMsg:  "OTEL_TRACE_SAMPLE_RATE inválido",
		},
		{
			name: "deve retornar erro quando TraceSampleRate acima de 1",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 1.1},
			},
			wantErr: true,
			errMsg:  "OTEL_TRACE_SAMPLE_RATE inválido",
		},
		{
			name: "deve aceitar TraceSampleRate zero como válido",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.0},
			},
			wantErr: false,
		},
		{
			name: "deve aceitar TraceSampleRate um como válido",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: local},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 1.0},
			},
			wantErr: false,
		},
		{
			name: "deve retornar erro quando production com senha curta",
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
			name: "deve aceitar production com senha de exatamente 16 chars",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: prod},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "exactly16chars!!", User: "dbuser"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
			},
			wantErr: false,
		},
		{
			name: "deve retornar erro quando production com placeholder CHANGE_ME na senha",
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
			name: "deve retornar erro quando production com your_secret_key na senha",
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
			name: "deve retornar erro quando production com financial@password na senha",
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
			name: "deve validar staging sem exigir senha longa",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "staging"},
				HTTPConfig: configs.HTTPConfig{Port: 8080},
				DBConfig:   configs.DBConfig{Password: "short", User: "user"},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 0.5},
			},
			wantErr: false,
		},
		{
			name: "deve acumular múltiplos erros de validação",
			cfg: &configs.Config{
				AppConfig:  configs.AppConfig{Environment: "invalid"},
				HTTPConfig: configs.HTTPConfig{Port: 0},
				O11yConfig: configs.O11yConfig{TraceSampleRate: 2.0},
			},
			wantErr: true,
			errMsg:  "ENVIRONMENT inválido",
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			err := sc.cfg.Validate()
			if sc.wantErr {
				s.Error(err)
				if sc.errMsg != "" {
					s.Contains(err.Error(), sc.errMsg)
				}
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *ConfigSuite) TestLoadConfigComArquivoValido() {
	cfg, err := configs.LoadConfig("./testdata/valid")

	s.NoError(err)
	s.NotNil(cfg)
	s.Equal("local", cfg.AppConfig.Environment)
	s.Equal(8080, cfg.HTTPConfig.Port)
	s.Equal("localhost", cfg.DBConfig.Host)
	s.Equal(1.0, cfg.O11yConfig.TraceSampleRate)
}

func (s *ConfigSuite) TestLoadConfigSemArquivoEnvRetornaErro() {
	dir := s.T().TempDir()

	cfg, err := configs.LoadConfig(dir)

	s.Error(err)
	s.Nil(cfg)
	s.Contains(err.Error(), ".env não encontrado")
}

func (s *ConfigSuite) TestLoadConfigProductionSemArquivoUsaEnvVars() {
	s.T().Setenv("ENVIRONMENT", "production")
	s.T().Setenv("PORT", "8080")
	s.T().Setenv("DB_HOST", "db.fly.internal")
	s.T().Setenv("DB_PORT", "5432")
	s.T().Setenv("DB_USER", "mecontrola")
	s.T().Setenv("DB_PASSWORD", "productionStrongPassword123!")
	s.T().Setenv("DB_NAME", "mecontrola_db")
	s.T().Setenv("DB_SSL_MODE", "require")
	s.T().Setenv("OTEL_TRACE_SAMPLE_RATE", "0.2")
	s.T().Setenv("SERVICE_NAME_API", "mecontrola-api")

	dir := s.T().TempDir()

	cfg, err := configs.LoadConfig(dir)

	s.NoError(err)
	s.NotNil(cfg)
	s.Equal("production", cfg.AppConfig.Environment)
	s.Equal("db.fly.internal", cfg.DBConfig.Host)
}

func (s *ConfigSuite) TestLoadConfigProductionInseguroRetornaErro() {
	cfg, err := configs.LoadConfig("./testdata/insecure-prod")

	s.Error(err)
	s.Nil(cfg)
	s.Contains(err.Error(), "placeholder inseguro")
}

func (s *ConfigSuite) TestSafeDSNNuncaContemSenha() {
	passwords := generateRandomPasswords(5)

	for i, pwd := range passwords {
		s.Run(fmt.Sprintf("senha_%d", i+1), func() {
			db := &configs.DBConfig{
				Host:     "localhost",
				Port:     5432,
				User:     "user",
				Password: pwd,
				Name:     "dbname",
				SSLMode:  "disable",
			}

			safeDSN := db.SafeDSN()

			s.NotContains(safeDSN, pwd)
			s.Contains(safeDSN, "***")
		})
	}
}

func (s *ConfigSuite) TestDSNContemSenha() {
	db := &configs.DBConfig{
		Host:     "db.example.com",
		Port:     5432,
		User:     "mecontrola",
		Password: "supersecretpassword",
		Name:     "mecontrola_db",
		SSLMode:  "require",
	}

	dsn := db.DSN()

	s.Contains(dsn, "supersecretpassword")
	s.Contains(dsn, "postgres://mecontrola:supersecretpassword@db.example.com:5432/mecontrola_db?sslmode=require")
}

func (s *ConfigSuite) TestSafeDSNFormato() {
	db := &configs.DBConfig{
		Host:     "db.example.com",
		Port:     5432,
		User:     "mecontrola",
		Password: "anypassword",
		Name:     "mecontrola_db",
		SSLMode:  "require",
	}

	safeDSN := db.SafeDSN()

	s.Equal("postgres://mecontrola:***@db.example.com:5432/mecontrola_db?sslmode=require", safeDSN)
}

func (s *ConfigSuite) TestInsecurePlaceholdersNaoVazio() {
	s.NotEmpty(configs.InsecurePlaceholders)
}

func (s *ConfigSuite) TestInsecurePlaceholdersContemValoresConhecidos() {
	known := []string{
		"CHANGE_ME_USE_STRONG_PASSWORD",
		"CHANGE_ME_GENERATE_SECURE_SECRET_KEY_MIN_64_CHARS",
		"your_secret_key",
		"financial@password",
	}

	for _, v := range known {
		s.Contains(configs.InsecurePlaceholders, v)
	}
}

func (s *ConfigSuite) TestValidateProductionUsuarioInseguro() {
	cfg := &configs.Config{
		AppConfig:  configs.AppConfig{Environment: "production"},
		HTTPConfig: configs.HTTPConfig{Port: 8080},
		DBConfig: configs.DBConfig{
			Password: "productionStrongPassword123!",
			User:     "your_secret_key",
		},
		O11yConfig: configs.O11yConfig{TraceSampleRate: 0.2},
	}

	err := cfg.Validate()
	s.Error(err)
	s.Contains(err.Error(), "DB_USER contém placeholder inseguro")
}

func (s *ConfigSuite) TestValidateProductionOTLPHeadersInseguro() {
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
	s.Error(err)
	s.Contains(err.Error(), "OTEL_EXPORTER_OTLP_HEADERS contém placeholder inseguro")
}

func (s *ConfigSuite) TestLoadConfigPortInvalidaRetornaErro() {
	s.T().Setenv("ENVIRONMENT", "local")
	s.T().Setenv("PORT", "99999")

	dir := s.T().TempDir()
	envContent := "ENVIRONMENT=local\nPORT=99999\nOTEL_TRACE_SAMPLE_RATE=1.0\n"
	err := os.WriteFile(dir+"/.env", []byte(envContent), 0600)
	s.Require().NoError(err)

	cfg, err := configs.LoadConfig(dir)

	s.Error(err)
	s.Nil(cfg)
	s.Contains(err.Error(), "PORT inválido")
}

func (s *ConfigSuite) TestLoadConfigTraceSampleRateInvalidoRetornaErro() {
	dir := s.T().TempDir()
	envContent := "ENVIRONMENT=local\nPORT=8080\nOTEL_TRACE_SAMPLE_RATE=2.5\n"
	err := os.WriteFile(dir+"/.env", []byte(envContent), 0600)
	s.Require().NoError(err)

	cfg, err := configs.LoadConfig(dir)

	s.Error(err)
	s.Nil(cfg)
	s.Contains(err.Error(), "OTEL_TRACE_SAMPLE_RATE inválido")
}

const passwordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"

func generateRandomPasswords(n int) []string {
	passwords := make([]string, n)
	for i := range passwords {
		length := 20 + rand.Intn(20) //nolint:gosec
		var sb strings.Builder
		for j := 0; j < length; j++ {
			sb.WriteByte(passwordChars[rand.Intn(len(passwordChars))]) //nolint:gosec
		}
		passwords[i] = sb.String()
	}
	return passwords
}
