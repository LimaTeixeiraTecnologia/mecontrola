# Tarefa 12.0: [ambos] Validação integração + E2E + gates de fronteira/conformidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Validar o fluxo Onboarding V2 fim-a-fim e travar a conformidade arquitetural: testes de integração
(Postgres/outbox) e E2E (`ATIVAR → intro → 4 etapas → 1ª transação → conclusão → handoff`), além dos
gates de fronteira (ADR-006) e conformância (zero comentários, sem SQL/`buildAutoSplits` em
`internal/agent`). Valida também que a Parte 1 (já implementada) permanece íntegra.

<requirements>
- RF-01..04, RF-06, RF-07: Parte 1 íntegra (auto-start, LLM mandatório).
- RF-27: mensagens só p/ onboarding enquanto InProgress. RF-28: pós-conclusão não reabre.
- RF-33: agentes isolados (gate de fronteira ADR-006).
- Cobre os edge cases EB-01..14 em integração/E2E.
</requirements>

## Subtarefas

- [ ] 12.1 Integração (testcontainers, `//go:build integration`): isolamento (sem onboarding em `agent_sessions`), conclusão determinística (`completed_at` + evento), retomada, drift, idempotência do greeting, WM assíncrona.
- [ ] 12.2 E2E (`internal/onboarding/e2e/support_runtime_test.go`): fluxo completo + handoff (RF-27/28) + off-topic (RF-36).
- [ ] 12.3 Gates de fronteira/conformidade (script): sem SQL/`buildAutoSplits`/import de domínio de outro módulo em `internal/agent`; zero comentários; switch de domínio não cresce.
- [ ] 12.4 Validar Parte 1 íntegra (config mandatória; auto-start; allowlist OpenRouter).

## Detalhes de Implementação

Ver techspec.md → "Abordagem de Testes" (Integração/E2E), "Cobertura de Edge Cases" e
ADR-006 (gates). Reusar padrão de `subscription_bound_integration_test.go`.

## Critérios de Sucesso

- Fluxo E2E verde; edge cases EB-01..14 exercitados.
- Gates de fronteira retornam vazio; Parte 1 validada.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — valida o fluxo do agente (Thread→Run, consumers, handoff) end-to-end no `internal/agent`.

go-implementation (linguagem, auto) e agent-governance (governança, auto) também se aplicam.

## Testes da Tarefa

- [ ] Testes de integração (testcontainers: isolamento, conclusão, retomada, drift, idempotência, WM)
- [ ] Testes E2E (fluxo completo + handoff + off-topic)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] Integração + E2E passam cobrindo RF-27/28 e EB-01..14.
- [ ] Gates de fronteira/conformidade retornam OK.
- [ ] Parte 1 validada íntegra.
- [ ] `go build ./...` e `go test ./...` (incl. `-tags=integration` onde aplicável) passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./... && go test ./internal/onboarding/... ./internal/budgets/... ./internal/agent/... ./configs/... -count=1
# Gate de fronteira (ADR-006) — devem retornar vazio:
grep -rn "buildAutoSplits" internal/agent --include="*.go" | grep -v _test && echo FAIL || echo OK
grep -rn "QueryContext\|ExecContext\|db\.Query\|tx\.Exec" \
  internal/agent/infrastructure/messaging/database/consumers/onboarding_bound_consumer.go \
  internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_consumer.go && echo FAIL || echo OK
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)" && echo FAIL || echo OK
# LLM flag removido (Parte 1)
grep -rn "OnboardingLLMEnabled" internal/ configs/ --include="*.go" && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/onboarding/e2e/support_runtime_test.go` (modificado/novo)
- testes de integração em `internal/onboarding/...`, `internal/budgets/...`, `internal/agent/...`
- script de gates de fronteira (CI/review)
