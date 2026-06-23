# Tarefa 2.0: Payload isolado — OnboardingTurn + campos + métodos With*

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender o domínio do onboarding (`internal/onboarding/domain/entities/onboarding_session.go`) para
ser autossuficiente: novo tipo `OnboardingTurn{Role, Text, OccurredAt}` e os campos
`RecentTurns`, `WelcomeSentAt`, `CompletedAt`, `ObjectiveProfile` em `OnboardingSessionPayload`, com
métodos imutáveis `WithCompletion(now)`, `WithAppendedTurn(...)` (bounded em 3 pares) e
`WithWelcomeSent(now)`. Domínio puro (ADR-001, ADR-002).

<requirements>
- RF-20: `recent_turns` bounded em 3 pares; isolado do onboarding.
- RF-22: payload persiste `welcome_sent_at` e `completed_at`.
- RF-35: ao concluir, `recent_turns` é limpo.
- ADR-001 (persistência isolada), ADR-002 (conclusão), DMMF (VO imutável, Decide* puro).
</requirements>

## Subtarefas

- [ ] 2.1 Definir `OnboardingTurn` (VO imutável) e adicionar campos ao `OnboardingSessionPayload`.
- [ ] 2.2 Implementar `WithAppendedTurn(role, text, now)` com bound de 3 pares (descarta o mais antigo).
- [ ] 2.3 Implementar `WithWelcomeSent(now)` (idempotente: não sobrescreve se já setado) e `WithCompletion(now)` (seta `CompletedAt`, zera `RecentTurns`; estado `active` aplicado no usecase).
- [ ] 2.4 Testes unitários puros (bound, idempotência de welcome, limpeza de turns na conclusão).

## Detalhes de Implementação

Ver techspec.md → "Modelos de Dados" (OnboardingTurn + campos) e "Conclusão determinística". Não
alterar a assinatura de `With(state, payload, updatedAt)` existente; os novos métodos compõem sobre
o payload.

## Critérios de Sucesso

- Append nunca excede 3 pares (6 entradas); ordem cronológica preservada.
- `WithWelcomeSent` é idempotente; `WithCompletion` zera `RecentTurns` e seta `CompletedAt`.
- Métodos são puros e retornam novo payload/sessão (sem mutação in-place).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (bound de turns; idempotência welcome; conclusão limpa turns)
- [ ] Testes de integração (não aplicável — domínio puro)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Campos e métodos adicionados sem quebrar a API atual do agregado.
- [ ] Zero comentários no `.go` de produção.
- [ ] `go build ./internal/onboarding/...` e `go test ./internal/onboarding/domain/...` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/onboarding/... && \
go test ./internal/onboarding/domain/entities/... -count=1
grep -rn --include="*.go" --exclude="*_test.go" "^[[:space:]]*//" \
  internal/onboarding/domain/entities/onboarding_session.go \
  | grep -Ev "(//go:|//nolint:|// Code generated)" && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/domain/entities/onboarding_session.go` (modificado)
- `internal/onboarding/domain/entities/onboarding_session_methods_test.go` (modificado/novo)
