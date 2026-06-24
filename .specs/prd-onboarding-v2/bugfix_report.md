# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 2
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: DRIFT-001
- Severidade: major
- Origem: RF-31
- Estado: fixed
- Causa raiz: O metodo `Find()` em `onboardingSessionRepository` nao detectava nem registrava a condicao de drift onde `state=active` mas `completed_at == nil` no payload JSON. O campo `driftCounter` nao existia na struct e nenhuma logica de deteccao havia sido implementada.
- Arquivos alterados:
  - `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go`
- Teste de regressao:
  - `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository_drift_test.go`
    - `TestFind_DriftDetected_ActiveWithoutCompletedAt` — verifica que counter e warn sao emitidos quando `state=active` e `completed_at==nil`
    - `TestFind_NoDrift_ActiveWithCompletedAt` — verifica que counter nao e incrementado quando `completed_at` esta presente
- Validacao:
  - `go build ./internal/onboarding/...` — PASS
  - `go test ./internal/onboarding/infrastructure/repositories/... -count=1` — PASS (2 novos testes)
  - `grep -rn "onboarding_state_drift_total"` — encontrado em producao e nos testes
  - R-ADAPTER-001.1 (zero comentarios) — PASS

## Comandos Executados

- `go build ./internal/onboarding/...` -> OK (sem output)
- `go test ./internal/onboarding/infrastructure/repositories/... -count=1` -> ok (0.263s)
- `grep -rn "onboarding_state_drift_total" internal/` -> 3 ocorrencias confirmadas
- gate zero comentarios -> OK: zero comentarios

## Riscos Residuais

- Nenhum. O counter usa label unit `"1"` sem label `user_id` (R-TXN-004 respeitado). A deteccao ocorre somente apos `json.Unmarshal` bem-sucedido; payload vazio (`len==0`) resulta em `pj.CompletedAt==nil`, portanto sessoes com payload vazio e `state=active` tambem emitem o drift — comportamento conservador e correto para RF-31.
