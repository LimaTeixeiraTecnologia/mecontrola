# Relatorio de Bugfix

- Total de bugs no escopo: 4
- Corrigidos: 4
- Testes de regressao adicionados: 2 (gate de lint mecanico + mudanca de assinatura compile-time)
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-BUD-001
- Severidade: critical
- Origem: finding de review per-user (sessao 2026-06-12) — analise critica em `~/.claude/plans/analise-de-forma-criteriosa-shiny-book.md`
- Estado: fixed
- Causa raiz: query UPDATE de `budgetRepository.Activate` continha apenas `WHERE id = $5`, ausencia de filtro `user_id` quebra a defesa em profundidade vs baseline `transaction_repository.go:130`. Caller que ler entity de outro usuario ou receber input adulterado pode mutar linha de qualquer dono.
- Arquivos alterados: `internal/budgets/infrastructure/repositories/postgres/budget_repository.go:92-99`
- Teste de regressao: `deployment/scripts/lint-repo-user-id.sh` + receita `task lint:user-isolation` em `taskfiles/lint.yml:69-77` — gate mecanico que falha o CI se qualquer query UPDATE/DELETE em `internal/{budgets,card,transactions}/infrastructure/repositories/postgres/*.go` voltar a omitir `user_id` na clausula WHERE. Excecoes automaticas para jobs de retencao (Purge/Cleanup/Reap + filtro `now() - interval`). Validado por simulacao adversarial (reverter o fix dispara o gate; restaurar volta a verde).
- Validacao: `task lint:user-isolation` PASS; `go test ./internal/budgets/...` PASS; `go vet ./...` PASS; gate detecta regressao quando simulada (validado).

- ID: BUG-BUD-002
- Severidade: critical
- Origem: finding de review per-user (sessao 2026-06-12)
- Estado: fixed
- Causa raiz: query UPDATE de `expenseRepository.Update` continha apenas `WHERE id = $8 AND version = $9 AND deleted_at IS NULL`. Ausencia de `user_id` permite cross-tenant write se caller errar `Get(user_id)` antes do save.
- Arquivos alterados: `internal/budgets/infrastructure/repositories/postgres/expense_repository.go:84-101`
- Teste de regressao: mesmo gate `task lint:user-isolation` (mecanico, cobre todo o subset budgets/card/transactions).
- Validacao: `task lint:user-isolation` PASS; `go test ./internal/budgets/...` PASS.

- ID: BUG-BUD-003
- Severidade: critical
- Origem: finding de review per-user (sessao 2026-06-12)
- Estado: fixed
- Causa raiz: query UPDATE de `expenseRepository.SoftDelete` continha apenas `WHERE id = $5 AND version = $6 AND deleted_at IS NULL`. Mesmo cenario que BUG-BUD-002.
- Arquivos alterados: `internal/budgets/infrastructure/repositories/postgres/expense_repository.go:121-134`
- Teste de regressao: mesmo gate `task lint:user-isolation`.
- Validacao: `task lint:user-isolation` PASS; `go test ./internal/budgets/...` PASS.

- ID: BUG-BUD-004
- Severidade: major
- Origem: finding de review per-user (sessao 2026-06-12)
- Estado: fixed
- Causa raiz: `pendingEventRepository.Transition` aceitava apenas `id uuid.UUID` na assinatura; query operava por `WHERE id = $4` sem `user_id`. Embora caller seja job interno (`RunPendingEventsReaper`) com baixa probabilidade de impersonacao externa, contraria o padrao baseline `transaction_repository.go:130` e abre risco em race conditions concorrentes ou replay de queue.
- Arquivos alterados:
  - `internal/budgets/application/interfaces/pending_event_repository.go:19` (assinatura interface)
  - `internal/budgets/infrastructure/repositories/postgres/pending_event_repository.go:88-101` (query + params)
  - `internal/budgets/application/usecases/run_pending_events_reaper.go:105,111,117` (3 callers passam `evt.UserID()`)
  - `internal/budgets/application/interfaces/mocks/pending_event_repository.go` (regenerado por `task mocks`)
- Teste de regressao: mudanca de assinatura — compilador agora obriga todo caller a fornecer `userID`. Garantia compile-time, mais forte que teste unitario. Gate `task lint:user-isolation` cobre adicionalmente a clausula SQL.
- Validacao: `go build ./...` PASS; `go test ./internal/budgets/...` PASS; `go vet ./...` PASS; `task lint:user-isolation` PASS.

## Comandos Executados

- `go build ./...` -> PASS
- `go vet ./internal/budgets/...` -> PASS
- `go vet ./...` -> PASS
- `cp .mockery.yml mockery.yml && task mocks; rm -f mockery.yml` -> mock `PendingEventRepository` regenerado com nova assinatura
- `go test -count=1 ./internal/budgets/...` -> PASS (todos os pacotes)
- `go test -count=1 ./...` -> PASS (bateria completa do repositorio, sem regressao em outros modulos)
- `gofmt -l internal/budgets/ deployment/scripts/ taskfiles/` -> PASS (saida vazia)
- `bash deployment/scripts/lint-repo-user-id.sh` -> PASS
- `task lint:user-isolation` -> PASS
- Simulacao adversarial: `sed` removeu `AND user_id = $6` de Activate -> gate FALHA com diagnostico claro; revert restaurou -> gate PASS

## Riscos Residuais

- **Integration tests (`*_integration_test.go`) com tag `//go:build integration` em `internal/budgets/infrastructure/repositories/postgres/` estao quebrados na baseline pre-existente** (drift: testes chamam `repo.CreateDraft(ctx, db, budget)` mas a interface real e `CreateDraft(ctx, budget)`). Esse drift e **pre-existente**, foi confirmado por `go vet -tags=integration ./internal/budgets/infrastructure/repositories/postgres/...` antes deste bugfix e nao foi introduzido nem agravado pelas correcoes. Documentado para responsavel pelo modulo budgets corrigir o testutil de integration.
- **Cobertura por testcontainers ausente para os 4 fixes**: dado o drift acima, o teste de regressao final usa (a) gate mecanico de lint que falha o CI em qualquer regressao futura nos 3 fixes de SQL, (b) garantia compile-time da assinatura `Transition(ctx, id, userID, ...)` para o fix 4. Ambos sao mais robustos que testes de integracao isolados que poderiam ser silenciados por mudanca de schema. Quando os integration tests forem fixados, recomenda-se adicionar cenario "user errado nao afeta linha de outro user" em cada repo cobrindo Activate/Update/SoftDelete/Transition.
- **Job `PurgeOld` (alerts) e `PurgeDeleted` (expenses) operam DELETE cross-user por design** (LGPD/retencao). O gate de lint reconhece o padrao via heuristica `func.*Purge|Cleanup|Reap` + filtro `now() - interval`. Caso novo job de retencao seja adicionado fora dessas convencoes de nome, o gate emitira falso positivo legitimo que devera ser tratado adicionando excecao no script ou renomeando a funcao.
- **R-ADAPTER-001.1 (zero comentarios `.go`)**: o gate lint adicionado e em bash (`deployment/scripts/lint-repo-user-id.sh`) — fora do escopo da regra. Os arquivos `.go` alterados nao receberam comentarios novos.
- **Modulo `internal/transactions` ja era conforme** (linha 130 do `transaction_repository.go` ja tinha `user_id`); o gate cobre transactions preventivamente.
- **Modulo `internal/card`**: gate cobre. Auditoria inicial reportou que repositories de card ja filtram `user_id` em todas as queries; gate valida automaticamente em qualquer mudanca futura.
