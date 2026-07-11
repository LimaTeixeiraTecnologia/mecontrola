# Tarefa 4.0: Workflow card-update-confirm com escrita idempotente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o workflow dedicado durável `card-update-confirm`, simétrico ao `card-create-confirm`: confirmação humana, escrita idempotente `operation="update_card"`, revalidação de versão no commit, classificação de erro de domínio e no-false-success (ADR-001/ADR-002/ADR-003).

<requirements>
- `BuildCardUpdateConfirmWorkflow(idem, cards)` durável (`Durable:true, MaxAttempts:1`), chave `:card-update`.
- Suspende com a pergunta de-para; no resume, decide via `DecideCardUpdateConfirmation`.
- `executeUpdateCard` grava via `IdempotentWriter.Execute(..., "update_card", "card", ...)` com o payload completo (incluindo `due_day`) e `ExpectedVersion` capturada.
- Erros de domínio (apelido em uso, vencimento inválido, não encontrado, conflito de versão) → mensagem determinística e run completa; erro não-domínio → `StepStatusFailed` sem falso sucesso.
- Cobre RF-08, RF-09, RF-18, RF-20, RF-21, RF-22, RF-23, RF-24, RF-26.
</requirements>

## Subtarefas

- [ ] 4.1 Criar `internal/agents/application/workflows/card_update_confirm_workflow.go`: `CardUpdateConfirmWorkflowID`, `CardUpdateKey`, `BuildCardUpdateConfirmWorkflow`, step de avaliação (suspende sem `ResumeText`; decide no resume).
- [ ] 4.2 Implementar `executeUpdateCard`: monta `interfaces.CardUpdate` com todos os campos novos (incluindo `DueDay` e `ClosingDay` quando presentes no estado) + `ExpectedVersion`; envolve com `IdempotentWriter` (wamid = `state.MessageID`, `itemSeq=0`, `operation="update_card"`, `resourceKind="card"`), retry transiente (`maxWriteAttempts`), no-false-success.
- [ ] 4.3 Implementar `isCardUpdateDomainError` e `cardUpdateDomainErrorMessage` cobrindo `ErrNicknameConflict`, `ErrInvalidNickname`, `ErrInvalidDueDay`, `ErrInvalidBank`, `ErrCardNotFound`, `ErrCardVersionConflict`.
- [ ] 4.4 Mensagens determinísticas de sucesso e replay reusando o padrão do create (RF-26).

## Detalhes de Implementação

Ver `techspec.md` seções "Interfaces Chave", "Abordagem de Testes" e ADR-001. Espelhar `card_create_confirm_workflow.go` (idempotência, retry, replay). Depende de 2.0 e 3.0.

## Critérios de Sucesso

- Aceite grava e persiste corretamente o novo `due_day`; conflito de versão retorna mensagem determinística sem gravar.
- Reenvio do mesmo wamid não aplica a edição duas vezes (replay idempotente).
- Falha não-domínio nunca reportada como sucesso (`StepStatusFailed`).
- Estados fechados; LLM ausente no workflow; zero comentários em `.go`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — cria workflow durável consumindo o kernel `internal/platform/workflow` e o `IdempotentWriter`, sem recriar primitivo; call-sites sancionadas.
- `domain-modeling-production` — classificação de erros de domínio e no-false-success como decisões de negócio explícitas sobre estados fechados.

## Testes da Tarefa

- [ ] Testes unitários: suite testify com `wfStore` in-memory + `NewEngine[CardUpdateState]` + `fakeIdempotentWriter` + `CardManager` mock (suspende/de-para, aceita/grava, cancela, ambíguo, expira, replay, erros de domínio incl. conflito de versão, falha transitória).
- [ ] Testes de integração: `//go:build integration` com testcontainers Postgres, módulo card real, `NewPostgresStore`, `NewIdempotentWrite` — resume durável, replay por wamid, conflito de versão real.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/card_update_confirm_workflow.go`
- `internal/agents/application/workflows/card_create_confirm_workflow.go` (molde)
- `internal/agents/application/workflows/pending_entry_workflow.go` (contrato `IdempotentWriter`)
- `internal/agents/application/workflows/transient_error.go`
