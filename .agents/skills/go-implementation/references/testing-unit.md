# Testing Unit

<!-- TL;DR
Teste unitario deve ser proporcional ao risco: suite e mockery sao obrigatorios apenas quando a superficie e a estrategia de teste realmente os exigirem.
Keywords: testing, unit, suite, mockery
Load complete when: tarefa altera ou cria teste unitario.
-->

## Objetivo
Reduzir falso positivo sem abrir mao de consistencia.

## Regime de severidade
- `mockery.yml` e `[HARD contextual]` quando houver mock gerado de interface.
- `testify/suite` e `[HARD contextual]` quando o pacote alterado ja usa suite stateful ou quando a nova cobertura exigir setup/teardown compartilhado.
- Table-driven tests continuam preferidos para variacoes de comportamento, mas nao exigem suite quando um teste simples e suficiente.

## Regras
- Priorizar teste de comportamento observavel.
- Usar mock apenas quando a fronteira precisar ser isolada.
- Se o pacote ja tiver padrao consolidado com `suite`, preserva-lo.
- Se a mudanca for pequena e puramente funcional, um teste simples com `require/assert` pode ser suficiente.

## Exemplo
Teste simples sem suite quando nao ha estado compartilhado:

```go
func TestNormalizePhone(t *testing.T) {
    got := valueobjects.NormalizePhone(" 5511999999999 ")
    require.Equal(t, "5511999999999", got)
}
```

Teste com suite quando o pacote ja opera com mocks/state reset:

```go
type ActivateSubscriptionSuite struct {
    suite.Suite
    repo *mocks.SubscriptionRepository
}

func TestActivateSubscriptionSuite(t *testing.T) {
    suite.Run(t, new(ActivateSubscriptionSuite))
}

func (s *ActivateSubscriptionSuite) SetupTest() {
    s.repo = mocks.NewSubscriptionRepository(s.T())
}
```

Exemplo ruim:
- exigir suite para funcao pura de 3 linhas;
- usar mockery quando um fake local resolveria melhor;
- testar detalhe interno em vez do efeito observavel.

## Validacao Minima
- `go test -count=1` no pacote alterado.
- `mockery --config mockery.yml --dry-run` somente quando mocks gerados fizerem parte do diff ou a interface testada mudar.

## Proibido
- Introduzir mockery onde um fake local simples resolve melhor.
- Exigir suite para teste trivial sem estado compartilhado so por uniformidade cosmetica.
