# Tarefa 3.0: Repository — JSON dos novos campos + drift no Find

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender `onboardingSessionPayloadJSON` em
`internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go` para
serializar/desserializar `recent_turns`, `welcome_sent_at`, `completed_at` e `objective_profile`, e
adicionar detecção de drift no `Find` (`state=active` sem `completed_at`).

<requirements>
- RF-19: estado do onboarding persistido exclusivamente em `onboarding_sessions`.
- RF-21: nada de estado de onboarding em `agent_sessions`.
- RF-31: drift `state=active` sem `completed_at` é explícito (métrica + warn).
- ADR-001 (sem migração de schema — `payload` já é JSONB), ADR-002.
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar campos JSON (`recent_turns`, `welcome_sent_at`, `completed_at`, `objective_profile`) ao struct e ao mapeamento de/para domínio (Find/Upsert).
- [ ] 3.2 Implementar contador `onboarding_state_drift_total` e log warn no `Find` quando `state=active` e `CompletedAt==nil`.
- [ ] 3.3 Testes de mapeamento (round-trip) e de drift.

## Detalhes de Implementação

Ver techspec.md → "Modelos de Dados" (JSON estendido) e "Conclusão determinística e drift". Não
alterar o schema SQL (coluna `payload` JSONB já existe). Não usar `agent_sessions`.

## Critérios de Sucesso

- Round-trip do payload preserva todos os campos novos (`omitempty`).
- Drift incrementa métrica e loga warn sem mascarar como sucesso nem como erro de leitura.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem). go-implementation (linguagem, auto) aplica-se.

## Testes da Tarefa

- [ ] Testes unitários (round-trip JSON; campos omitempty; drift)
- [ ] Testes de integração (Postgres em T12 — isolamento e drift end-to-end)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Campos novos mapeados em Find e Upsert.
- [ ] Métrica de drift exposta; labels enums fechados (sem `user_id`).
- [ ] Zero comentários no `.go` de produção; zero SQL novo (sem migração).
- [ ] `go build ./internal/onboarding/...` e `go test ./internal/onboarding/infrastructure/...` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/onboarding/... && \
go test ./internal/onboarding/infrastructure/repositories/postgres/... -count=1
# sem agent_sessions no fluxo de onboarding
grep -rn "agent_sessions" internal/onboarding --include="*.go" | grep -v _test && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go` (modificado)
- teste correspondente (novo/modificado)
