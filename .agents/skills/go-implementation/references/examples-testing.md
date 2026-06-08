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

## Esqueleto canonico testify/suite (R4 — todos os `_test.go`)
Padrao obrigatorio e inegociavel para todos os arquivos `_test.go`:
`package <pacote>_test`, suite struct com mocks tipados, registrador `suite.Run`, `SetupTest`
reiniciando contexto e mocks, tabela `scenarios` com `args`, `setup func()` e `expect func(...)`,
e SUT real instanciado dentro do loop. Cenarios minimos: happy path, erro de validacao de dominio,
erro de infraestrutura e edge case/idempotencia quando aplicavel.

```go
package services_test

import (
    "context"
    "errors"
    "testing"

    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/application/dtos/input"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/application/dtos/output"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/application/services"
    domainerrors "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/domain/errors"
    repositoryMock "github.com/LimaTeixeiraTecnologia/mecontrola/internal/<modulo>/domain/interfaces/mocks"
)

type CreateUserServiceSuite struct {
    suite.Suite

    ctx            context.Context
    userRepository *repositoryMock.UserRepository
}

func TestCreateUserServiceSuite(t *testing.T) {
    suite.Run(t, new(CreateUserServiceSuite))
}

func (s *CreateUserServiceSuite) SetupTest() {
    s.ctx = context.Background()
    s.userRepository = repositoryMock.NewUserRepository(s.T())
}

func (s *CreateUserServiceSuite) TestExecute() {
    type args struct {
        input *input.CreateUserInput
    }

    scenarios := []struct {
        name   string
        args   args
        setup  func()
        expect func(result *output.UserOutput, err error)
    }{
        {
            name: "deve criar usuario com sucesso",
            args: args{input: &input.CreateUserInput{Name: "Joao", Email: "joao@email.com"}},
            setup: func() {
                s.userRepository.EXPECT().
                    Save(s.ctx, mock.Anything).
                    Return(nil).
                    Once()
            },
            expect: func(result *output.UserOutput, err error) {
                s.NoError(err)
                s.NotNil(result)
            },
        },
        {
            name: "deve retornar erro ao validar input invalido",
            args: args{input: &input.CreateUserInput{Name: "", Email: ""}},
            setup: func() {
                // Nenhum mock necessario: a validacao deve falhar antes de IO.
            },
            expect: func(result *output.UserOutput, err error) {
                s.Error(err)
                s.ErrorIs(err, domainerrors.ErrInvalidInput)
                s.Nil(result)
            },
        },
        {
            name: "deve retornar erro ao salvar no repositorio",
            args: args{input: &input.CreateUserInput{Name: "Joao", Email: "joao@email.com"}},
            setup: func() {
                s.userRepository.EXPECT().
                    Save(s.ctx, mock.Anything).
                    Return(errors.New("falha no banco")).
                    Once()
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
            s.SetupTest()
            scenario.setup()

            service := services.NewCreateUserService(s.userRepository)
            result, err := service.Execute(s.ctx, scenario.args.input)
            scenario.expect(result, err)
        })
    }
}
```

Regras de adaptacao do exemplo:
- Substituir nomes, imports e DTOs pelos packages reais do bounded context; exemplos externos servem
  apenas como forma estrutural.
- Usar mocks gerados por `mockery.yml` com `with-expecter: true`; nao escrever mocks manuais para
  dependencias mockaveis.
- Criar helpers locais pequenos apenas para matchers ou stubs triviais de valor, por exemplo
  `decimalMatcher(expected string) any` com `mock.MatchedBy`.
- Reexecutar `s.SetupTest()` dentro de cada `s.Run` quando os cenarios compartilham a mesma suite e
  configuram expectativas diferentes.
