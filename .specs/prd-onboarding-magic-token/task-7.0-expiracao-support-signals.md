# Tarefa 7.0: Job diario de expiracao e sinais de subscription orfa

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar processo diario de expiracao de tokens vencidos, sinalizacao de subscriptions orfas pagas e limpeza operacional de idempotencia Meta/tentativas auxiliares.

<requirements>
- Cobrir `RF-11`, `RF-12`, `RF-13`, `RF-14`.
- Tokens `PENDING` ou `PAID` vencidos devem ir para `EXPIRED`.
- Token pago expirado deve inserir `support_signals(kind='orphan_expired_subscription')` com payload minimo da techspec.
- A subscription nao deve ser cancelada automaticamente.
- Job deve processar em batches e ser cancelavel via `context.Context`.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 7.1 Implementar `ExpireTokens` com batches e transicoes idempotentes.
- [ ] 7.2 Inserir support signal para token pago expirado.
- [ ] 7.3 Implementar `TokenExpirationJob` via worker job adapter.
- [ ] 7.4 Implementar limpeza de `meta_processed_messages` e tabela auxiliar de tentativas quando existir.
- [ ] 7.5 Emitir metricas de expiracao e signals.
- [ ] 7.6 Testar PENDING expirado, PAID expirado, CONSUMED ignorado e EXPIRED no-op.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 2, 5.2, 6.12, 7.3, 8.5, 9.2 e ADR-006. O fluxo de suporte e passivo e consultavel; nao criar alerta Slack/email.

## Critérios de Sucesso

- Job diario nao cancela assinatura nem altera token `CONSUMED`.
- Support signal contem `external_sale_id`, hash/prefixo do token, instante de expiracao e indicacao de assinatura ativa sem usuario vinculado.
- Execucoes repetidas nao duplicam sinais indevidamente.
- Logs mascaram PII.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitarios de `ExpireTokens`.
- [ ] Testes de repositorio/job para batches e idempotencia.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/usecases/expire_tokens.go`
- `internal/onboarding/infrastructure/jobs/handlers/token_expiration_job.go`
- `internal/onboarding/infrastructure/repositories/postgres/support_signal_repository.go`
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`
