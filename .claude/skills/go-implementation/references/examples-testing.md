# Exemplos: Testes e Validacao

<!-- TL;DR
Exemplos de testes em Go: construtores com invariantes, table-driven tests, mocks, fakes e testes de integração com TempDir.
Keywords: exemplo, teste, table-driven, mock, fake, invariante, integração
Load complete when: tarefa requer exemplos concretos de testes unitários, de integração ou uso de fakes/mocks em Go.
-->

## Construtor com invariantes
```go
type Config struct {
    timeout time.Duration
}

func NewConfig(timeout time.Duration) (Config, error) {
    if timeout <= 0 {
        return Config{}, fmt.Errorf("timeout must be positive")
    }
    return Config{timeout: timeout}, nil
}
```

## Interface no consumidor
```go
type clock interface {
    Now() time.Time
}
```

## Table-driven test com testify/suite
```go
package valueobjects_test

import (
    "testing"

    "github.com/stretchr/testify/suite"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/domain/valueobjects"
)

type NormalizeSuite struct {
    suite.Suite
}

func TestNormalizeSuite(t *testing.T) {
    suite.Run(t, new(NormalizeSuite))
}

func (s *NormalizeSuite) SetupTest() {}

func (s *NormalizeSuite) TestNormalize() {
    scenarios := []struct {
        name string
        in   string
        want string
    }{
        {name: "deve remover espacos ao redor", in: " a ", want: "a"},
        {name: "deve manter string vazia", in: "", want: ""},
    }

    for _, scenario := range scenarios {
        s.Run(scenario.name, func() {
            got := valueobjects.Normalize(scenario.in)
            s.Equal(scenario.want, got)
        })
    }
}
```

## Esqueleto canonico testify/suite (R-TESTING-001 — use cases)
Padrao obrigatorio e inegociavel para `internal/*/application/usecases/*_test.go`:
`package <pacote>` (whitebox, mesmo pacote), suite struct com `obs observability.Observability`
e mocks tipados, `SetupTest` com `fake.NewProvider()` reiniciando todos os mocks,
tabela `scenarios` com `args`, `dependencies struct` + IIFE por mock e `expect func(...)`,
e SUT real instanciado dentro do `s.Run`. Cenarios minimos: happy path, erro de validacao de
dominio, erro de infraestrutura e edge case/idempotencia quando aplicavel.

Anti-padroes proibidos:
- `noop.NewProvider()` — use `fake.NewProvider()`
- `setup func()` como campo do cenario — use `dependencies struct` com IIFE
- `s.SetupTest()` manual dentro de loop ou metodo — testify ja chama automaticamente
- `package <X>_test` — use `package <X>` (whitebox)

```go
package usecases // mesmo pacote — whitebox

import (
    "context"
    "errors"
    "testing"

    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/application/dtos/input"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/application/dtos/output"
    domainerrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/domain/errors"
    repositoryMock "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/infrastructure/repositories/mocks"
)

type CreateUserUseCaseSuite struct {
    suite.Suite

    ctx            context.Context
    obs            observability.Observability
    userRepository *repositoryMock.UserRepository
}

func TestCreateUserUseCaseSuite(t *testing.T) {
    suite.Run(t, new(CreateUserUseCaseSuite))
}

func (s *CreateUserUseCaseSuite) SetupTest() {
    s.obs = fake.NewProvider()
    s.ctx = context.Background()
    s.userRepository = repositoryMock.NewUserRepository(s.T())
}

func (s *CreateUserUseCaseSuite) TestExecute() {
    type args struct {
        input *input.CreateUserInput
    }

    type dependencies struct {
        userRepository *repositoryMock.UserRepository
    }

    scenarios := []struct {
        name         string
        args         args
        dependencies dependencies
        expect       func(result *output.UserOutput, err error)
    }{
        {
            name: "deve criar usuario com sucesso",
            args: args{input: &input.CreateUserInput{Name: "Joao", Email: "joao@email.com"}},
            dependencies: dependencies{
                userRepository: func() *repositoryMock.UserRepository {
                    s.userRepository.EXPECT().
                        Save(s.ctx, mock.AnythingOfType("*entities.User")).
                        Return(nil).
                        Once()
                    return s.userRepository
                }(),
            },
            expect: func(result *output.UserOutput, err error) {
                s.NoError(err)
                s.NotNil(result)
            },
        },
        {
            name: "deve retornar erro ao validar input invalido",
            args: args{input: &input.CreateUserInput{Name: "", Email: ""}},
            dependencies: dependencies{userRepository: s.userRepository},
            expect: func(result *output.UserOutput, err error) {
                s.Error(err)
                s.ErrorIs(err, domainerrors.ErrInvalidInput)
                s.Nil(result)
            },
        },
        {
            name: "deve retornar erro ao salvar no repositorio",
            args: args{input: &input.CreateUserInput{Name: "Joao", Email: "joao@email.com"}},
            dependencies: dependencies{
                userRepository: func() *repositoryMock.UserRepository {
                    s.userRepository.EXPECT().
                        Save(s.ctx, mock.AnythingOfType("*entities.User")).
                        Return(errors.New("falha no banco")).
                        Once()
                    return s.userRepository
                }(),
            },
            expect: func(result *output.UserOutput, err error) {
                s.Error(err)
                s.Contains(err.Error(), "falha no banco")
                s.Nil(result)
            },
        },
    }

    for _, scenario := range scenarios {
        s.Run(scenario.name, func() {
            uc := NewCreateUserUseCase(s.obs, scenario.dependencies.userRepository)
            result, err := uc.Execute(s.ctx, scenario.args.input)
            scenario.expect(result, err)
        })
    }
}
```

Regras de adaptacao do exemplo:
- Substituir nomes, imports e DTOs pelos packages reais do bounded context.
- Usar mocks gerados por `mockery.yml` com `with-expecter: true`; nao escrever mocks manuais.
- Quando o use case nao recebe `obs`, omitir o campo `obs` da suite e remover `fake.NewProvider()`.
- Arquivos `*_integration_test.go` podem usar `package <X>_test` e nao precisam seguir este esqueleto.
- Ver `.claude/rules/go-testing.md` para regras completas e gates de verificacao.
