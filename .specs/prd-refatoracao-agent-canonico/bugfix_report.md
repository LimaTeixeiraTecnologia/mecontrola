# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 0 (correcao remove cenarios e2e stale; a regressao e coberta pela suite e2e WhatsApp-only ja existente e pelo guard `internal/identity/domain/valueobjects/channel_test.go:26`)
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: BUG-TELEGRAM-E2E-STALE
- Severidade: major
- Origem: RF-02 / RF-04 (prd-refatoracao-agent-canonico), task-2.0/3.0 (eliminacao 100% do canal Telegram)
- Estado: fixed
- Causa raiz: A Channel VO passou a rejeitar "telegram" (`ErrChannelUnknown`, asserido em `channel_test.go:26`). Cenarios `.feature` e2e remanescentes ainda vinculavam/resolviam o canal "telegram" (identity) ou exercitavam ativacao/bloqueio de Telegram no onboarding. Em identity os cenarios falhariam de verdade (steps batem no banco via `LinkChannelToUser`/`ResolvePrincipalByIdentity` com canal rejeitado). No onboarding os cenarios referenciavam steps inexistentes ("o processor do Telegram...", "telegram-welcome", "telegram-requires-whatsapp") — dead/undefined steps remanescentes de funcionalidade ja removida pela RF-02.
- Arquivos alterados:
  - `internal/identity/e2e/features/f05_identity_vinculacao_canal.feature` — 4 cenarios reescritos de "telegram" para "whatsapp" (external_ids distintos do numero de cadastro), preservando a intencao de mecanica de vinculacao: vinculo+assercao no banco, canal preferido, rejeicao de duplicata, rejeicao de external_id ja associado a outro usuario.
  - `internal/identity/e2e/features/f10_identity_resolve_principal.feature` — 2 cenarios reescritos para "whatsapp": resolucao de principal por canal vinculado retorna UserID correto; resolucao por external_id sem vinculo retorna erro (preserva o caminho de erro antes ancorado em canal desconhecido).
  - `internal/onboarding/e2e/features/activation_processors.feature` — removido cenario "Ativar via Telegram direto" (RF-02 eliminou ativacao Telegram; cenario WhatsApp mantido).
  - `internal/onboarding/e2e/features/robustness.feature` — removido cenario "Ativacao direta no Telegram bloqueada por falta de dados" (dead; sem step definido).
- Teste de regressao: nenhum novo arquivo de teste criado. Justificativa: o defeito e cenario e2e stale, nao um defeito de codigo de producao. A protecao de regressao e dupla e ja existente — (a) guard unitario `internal/identity/domain/valueobjects/channel_test.go:26` garante que "telegram" continua rejeitado; (b) as suites e2e identity/onboarding agora compilam e ficam WhatsApp-only, sem steps undefined. Reintroduzir um cenario telegram voltaria a quebrar uma dessas barreiras.
- Validacao:
  - `grep -rni "telegram" internal/identity/e2e internal/onboarding/e2e` -> vazio (exit 1, sem matches).
  - `go test -tags e2e -run xxx_none ./internal/identity/e2e/... ./internal/onboarding/e2e/...` -> ambas as suites compilam: `ok ... identity/e2e [no tests to run]`, `ok ... onboarding/e2e [no tests to run]`.
  - `go test ./internal/identity/domain/valueobjects/ -run TestNewChannel` -> ok.

## Comandos Executados
- `grep -rni "telegram" internal/identity/e2e internal/onboarding/e2e` -> vazio (grep-exit:1, nenhum match)
- `go test -tags e2e -run xxx_none ./internal/identity/e2e/... ./internal/onboarding/e2e/...` -> ok (no tests to run) em ambos os pacotes; compilacao verde
- `go test ./internal/identity/domain/valueobjects/ -run TestNewChannel` -> ok

## Riscos Residuais
- Investigacao RF-02 confirmada: zero step definition de Telegram nos `*_steps_test.go` do onboarding e identity (`grep -rn 'Telegram\|telegram' internal/onboarding/e2e/*.go internal/identity/e2e/*.go` -> sem matches). Os cenarios Telegram do onboarding referenciavam steps inexistentes, ou seja, ja eram dead — nenhum processor Telegram de producao foi tocado nem encontrado.
- Os cenarios identity foram reescritos (nao deletados) para preservar cobertura de mecanica de vinculacao/resolucao de canal; a execucao real depende de banco de e2e disponivel (nao executada aqui — apenas compilacao no-op conforme escopo). Risco baixo: steps sao os mesmos ja usados por outros cenarios WhatsApp.
- Fora do escopo e intencionalmente nao tocado: `internal/agent`, budgets, transactions, migrations, codigo de producao, e `ALERT_TELEGRAM_*` em `.env.example` (decisao separada).
