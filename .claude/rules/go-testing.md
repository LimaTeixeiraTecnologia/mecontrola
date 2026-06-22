# Testes Go — Padrao Canonico testify/suite

- Rule ID: R-TESTING-001
- Severidade: hard
- Escopo: `internal/*/application/usecases/*_test.go`

## Objetivo

Garantir que todos os testes de use cases usem o padrao canonico testify/suite com whitebox
package, dependencies struct com IIFE por mock e `fake.NewProvider()` para observabilidade.

## Padrao Canonico Obrigatorio

```go
package usecase // mesmo pacote — whitebox, NAO package usecase_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/suite"
    "github.com/JailtonJunior94/devkit-go/pkg/observability"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

    // dtos, mocks e entidades do bounded context
)

type XxxUseCaseSuite struct {
    suite.Suite
    ctx     context.Context
    obs     observability.Observability
    xxxMock *mocks.XxxRepository
    // ... demais mocks como campos tipados
}

func TestXxxUseCaseSuite(t *testing.T) {
    suite.Run(t, new(XxxUseCaseSuite))
}

func (s *XxxUseCaseSuite) SetupTest() {
    s.obs = fake.NewProvider()
    s.ctx = context.Background()
    s.xxxMock = mocks.NewXxxRepository(s.T())
    // reiniciar TODOS os mocks aqui
}

func (s *XxxUseCaseSuite) TestExecute() {
    type args struct {
        input *dtos.XxxInput
    }
    type dependencies struct {
        xxxMock *mocks.XxxRepository
    }

    scenarios := []struct {
        name         string
        args         args
        dependencies dependencies
        expect       func(output *dtos.XxxOutput, err error)
    }{
        {
            name: "deve executar com sucesso",
            args: args{input: &dtos.XxxInput{...}},
            dependencies: dependencies{
                xxxMock: func() *mocks.XxxRepository {
                    s.xxxMock.EXPECT().
                        Save(s.ctx, mock.AnythingOfType("*entities.Xxx")).
                        Return(nil).
                        Once()
                    return s.xxxMock
                }(),
            },
            expect: func(output *dtos.XxxOutput, err error) {
                s.NoError(err)
                s.NotNil(output)
            },
        },
        {
            name: "deve retornar erro de validacao",
            args: args{input: &dtos.XxxInput{Name: ""}},
            dependencies: dependencies{xxxMock: s.xxxMock},
            expect: func(output *dtos.XxxOutput, err error) {
                s.Error(err)
                s.Nil(output)
            },
        },
        {
            name: "deve retornar erro de infraestrutura",
            args: args{input: &dtos.XxxInput{Name: "valid"}},
            dependencies: dependencies{
                xxxMock: func() *mocks.XxxRepository {
                    s.xxxMock.EXPECT().
                        Save(s.ctx, mock.AnythingOfType("*entities.Xxx")).
                        Return(errors.New("falha no banco")).
                        Once()
                    return s.xxxMock
                }(),
            },
            expect: func(output *dtos.XxxOutput, err error) {
                s.Error(err)
                s.Nil(output)
            },
        },
    }

    for _, scenario := range scenarios {
        s.Run(scenario.name, func() {
            uc := NewXxxUseCase(s.obs, scenario.dependencies.xxxMock)
            output, err := uc.Execute(s.ctx, scenario.args.input)
            scenario.expect(output, err)
        })
    }
}
```

## Regras Hard [HARD]

### R-TESTING-001.1 — Package whitebox obrigatorio

Todo `_test.go` em `internal/*/application/usecases/` DEVE declarar `package <X>` (mesmo pacote
do codigo de producao). Proibido `package <X>_test`.

### R-TESTING-001.2 — Suite struct com obs e mocks tipados

A suite struct DEVE conter:
- `ctx context.Context`
- `obs observability.Observability` (quando o use case recebe observabilidade)
- Um campo por mock, com o tipo concreto gerado pelo mockery

Proibido: mocks declarados como `interface{}` ou instanciados fora da suite.

### R-TESTING-001.3 — SetupTest reinicia tudo com fake.NewProvider()

`SetupTest()` DEVE:
- Atribuir `s.obs = fake.NewProvider()`
- Reiniciar todos os mocks da suite via `mocks.NewXxx(s.T())`
- Atribuir `s.ctx = context.Background()`

Proibido: `noop.NewProvider()` em testes de use case.

### R-TESTING-001.4 — dependencies struct com IIFE por mock

Cada cenario na tabela DEVE usar `type dependencies struct { ... }` com os mocks como campos
tipados. As expectativas de cada mock DEVEM ser configuradas via IIFE (`func() *mocks.Xxx { ... }()`)
dentro da declaracao do cenario.

Proibido: `setup func()` lambda chamado dentro do `s.Run`, `setup func(dependencies)` como campo
do cenario, configurar expectativas de mock fora do IIFE ou depois da declaracao do cenario.

### R-TESTING-001.5 — SUT instanciado dentro do s.Run

O SUT (System Under Test) DEVE ser instanciado dentro de `s.Run(scenario.name, func() { ... })`,
usando `scenario.dependencies.*` como argumentos.

Proibido: instanciar o SUT fora do loop de cenarios.

### R-TESTING-001.6 — Sem double SetupTest

Proibido chamar `s.SetupTest()` manualmente dentro de qualquer metodo de teste ou loop.
O testify/suite ja chama `SetupTest()` automaticamente antes de cada metodo `TestXxx`.

### R-TESTING-001.7 — Sem testes flat em use cases

Proibido usar `func TestXxx(t *testing.T)` sem suite em arquivos de use case. Toda funcao
de teste DEVE ser um metodo da suite.

## Gates de Verificacao

```bash
# R-TESTING-001.1 — sem package blackbox em usecases
grep -rn --include="*_test.go" "^package.*_test$" internal/*/application/usecases/ \
  && echo "FAIL: blackbox package em usecase" && exit 1 || true

# R-TESTING-001.3 — sem noop em usecases
grep -rn --include="*_test.go" "noop.NewProvider" internal/*/application/usecases/ \
  && echo "FAIL: noop.NewProvider em usecase test" && exit 1 || true

# R-TESTING-001.6 — sem double SetupTest
grep -rn --include="*_test.go" "s\.SetupTest()" internal/*/application/usecases/ \
  && echo "FAIL: double SetupTest detectado" && exit 1 || true
```

## Excecoes Documentadas

- Arquivos `*_integration_test.go`: podem usar `package <X>_test` quando testam via HTTP real
  ou banco de dados — fora do escopo desta regra.
- Arquivos de helper puro (ex: `testhelpers_test.go`) sem funcoes `TestXxx`: isentos das
  regras de suite, mas DEVEM usar `package <X>_test` para nao vazar simbolos de teste.
- Testes de dominio puro (`internal/*/domain/`): fora do escopo; nao precisam de `obs`.

## Referencias

- `examples-testing.md` em `.agents/skills/go-implementation/references/` — esqueleto completo
- `go-adapters.md` (R-ADAPTER-001) — regras analogas para camada de adapter
- `governance.md` — precedencia: esta regra prevalece sobre padrao generico Uber para estrutura de testes
