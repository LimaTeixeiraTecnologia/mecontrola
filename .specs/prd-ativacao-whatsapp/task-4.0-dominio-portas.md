# Tarefa 4.0: Domínio e portas (janela, query por telefone, throttle, concorrência)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os primitivos de domínio e as portas de persistência que sustentam a correlação por telefone com janela de 24h, a unificação da janela no caminho por token, a robustez de concorrência e o store durável de throttle.

<requirements>
- RF-08/RF-09: estados fechados existentes preservados (`TokenStatus`/`ActivationPath`).
- RF-10: guard puro `MagicToken.IsActivationWindowOpen(now, window)` (DMMF Princípio 6; sem ctx/IO).
- RF-23: query `FindActivableByMobile(ctx, mobileE164, paidAfter)` — `status='PAID' AND customer_mobile_e164=$1 AND paid_at > $2 ORDER BY paid_at DESC LIMIT 1`.
- RF-25/RF-27: `ConsumeMagicToken` passa a exigir `IsActivationWindowOpen` além de `!IsExpiredAt`; `UpdateMarkConsumed` checa `RowsAffected==0` → `AlreadyActive` (concorrência).
- Porta `NoMatchThrottle.AllowReply(ctx, mobileE164, windowStart)` + adapter postgres + housekeeping job.
</requirements>

## Subtarefas

- [ ] 4.1 Adicionar guard puro `IsActivationWindowOpen` em `internal/onboarding/domain/entities/magic_token.go` + testes de borda.
- [ ] 4.2 Adicionar `FindActivableByMobile` à interface `application/interfaces/magic_token_repository.go` e ao adapter postgres (SQL + scan), regenerar mock.
- [ ] 4.3 `ConsumeMagicToken`: exigir também `IsActivationWindowOpen(now, window)`; injetar a janela via config.
- [ ] 4.4 Endurecer `UpdateMarkConsumed` para retornar/checar linhas afetadas e mapear `0` para `ConsumeOutcomeAlreadyActive` (sem segundo `subscription_bound`).
- [ ] 4.5 Criar store de throttle: porta `NoMatchThrottle`, adapter postgres (InsertIfAbsent em `onboarding_activation_nomatch_throttle`) e job de housekeeping (padrão `internal/platform/whatsapp/dedup`).

## Detalhes de Implementação

Ver techspec.md, seções "Interfaces Chave", "Modelos de Dados" e ADR-002 (janela nos dois caminhos). Domínio puro; interface no consumidor; sem comentários em `.go`.

## Critérios de Sucesso

- Guard puro testado dentro/fora da janela, `paidAt` zero e status ≠ PAID.
- `FindActivableByMobile` valida janela, seleção da mais recente (recompra) e ausência de match (integração).
- `UpdateMarkConsumed` com guard de concorrência comprovado em teste (perda da corrida → `AlreadyActive`).
- Throttle durável idempotente por `(mobile, window)`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unitários do guard e do mapeamento de outcome (testify/suite, `fake.NewProvider()`).
- [ ] Integração (testcontainers) de `FindActivableByMobile`, `UpdateMarkConsumed` concorrente e throttle.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/domain/entities/magic_token.go`
- `internal/onboarding/application/interfaces/magic_token_repository.go`
- `internal/onboarding/infrastructure/repositories/postgres/magic_token_repository.go`
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/.../nomatch_throttle` (porta + adapter + job, novos)
