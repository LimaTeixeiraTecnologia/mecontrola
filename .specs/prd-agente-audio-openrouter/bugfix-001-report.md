# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 1 (ajuste de teste existente convertido em regressao)
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: BUG-001
- Severidade: minor
- Origem: finding de review de codigo (`internal/platform/llm/stt.go:97`)
- Estado: fixed
- Causa raiz: `DecideSTTPreflightCost` continha um branch `if maxCostMicrousd <= 0 { return nil }` que tratava "sem teto de custo" como aprovado. Esse branch e inalcancavel em producao porque o unico entrypoint publico (`(*openrouterProvider).Transcribe`, `internal/platform/llm/openrouter_stt.go:57`) chama `validateTranscriptionRequest` ANTES de `DecideSTTPreflightCost` (linha 63), e `validateTranscriptionRequest` (linha 133 de `stt.go`) ja rejeita `MaxCostMicrousd<=0` com `ErrSTTMaxCostRequired`. O teste `no_cap_when_max_zero` validava esse caminho morto, criando falsa confiança de cobertura para um comportamento nunca exercitado em produção.
- Arquivos alterados:
  - `internal/platform/llm/stt.go` — removido o branch morto "sem teto" de `DecideSTTPreflightCost`; a função passa a ser puramente fail-closed (sempre calcula `estimated` e compara contra `maxCostMicrousd`, sem tratamento especial para `maxCostMicrousd<=0`).
  - `internal/platform/llm/openrouter_stt_test.go` — teste `no_cap_when_max_zero` renomeado para `fail_closed_when_max_zero` e invertido: agora valida que `DecideSTTPreflightCost(60_000, 0, 34)` retorna `ErrSTTCostPreflightExceeded` (fail-closed), em vez de validar aprovação silenciosa de um caminho inalcançável.
- Teste de regressao: `TestDecideSTTPreflightCost/fail_closed_when_max_zero` (`internal/platform/llm/openrouter_stt_test.go:280-285`) — reproduz a chamada direta com `maxCostMicrousd=0` (unico jeito de alcançar esse branch, já que o entrypoint público bloqueia esse valor antes) e comprova o novo comportamento fail-closed.
- Validacao: build + vet + test -race do pacote `internal/platform/llm` 100% verdes (ver Comandos Executados).

## Comandos Executados
- `go build ./internal/platform/llm/...` -> OK (sem output, exit 0)
- `go vet ./internal/platform/llm/...` -> OK (sem output, exit 0)
- `go test -race -count=1 ./internal/platform/llm/...` -> `ok  github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm  3.063s` (todos os testes, incluindo `TestDecideSTTPreflightCost` e demais suites do pacote, passaram)

## Riscos Residuais
- Nenhum. A mudança de comportamento de `DecideSTTPreflightCost` para `maxCostMicrousd<=0` (antes: aprovado; agora: rejeitado) é estritamente interna à função — o único chamador de produção (`Transcribe`) já garante `MaxCostMicrousd>0` antes de invocá-la via `validateTranscriptionRequest`, portanto não há regressão observável em produção.
