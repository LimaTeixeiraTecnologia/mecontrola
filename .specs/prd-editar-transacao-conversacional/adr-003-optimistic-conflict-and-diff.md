# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Conflito de controle otimista (re-ler + re-confirmar) e resumo de confirmação em diff antes→depois
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da techspec, aprovação do solicitante (múltipla escolha, recomendação aceita)
- **Relacionados:** PRD (D-07, D-09, RF-16, RF-20..22), techspec.md, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.7)

## Contexto

A edição grava via `UpdateTransaction` com `version` (optimistic lock): `UpdateWithVersion ... WHERE version = expected`. No fluxo conversacional, o alvo é lido (obtendo `version`) e a confirmação ocorre depois, possivelmente minutos após — a `version` pode mudar nesse intervalo (ex.: outra edição do próprio usuário). Sobrescrever à força perderia mudanças silenciosamente. Além disso, o resumo de confirmação atual (`buildConfirmSummary`, `pending_entry_workflow.go:889-909`) mostra apenas o estado resultante, sem evidenciar o que muda — subótimo para revisar uma edição.

## Decisão

1. Ao gravar, se `UpdateTransaction` retornar erro de incompatibilidade de `version`, o step de escrita **re-lê** a transação atual (nova `version` + valores), reconstrói o resumo e **re-suspende** o run em `AwaitingSlotConfirmation` com o diff atualizado. Sem last-writer-wins, sem sobrescrita silenciosa.
2. Edição no-op (valores confirmados idênticos aos atuais, detectado em `DecideUpdate`) não grava, não incrementa `version` e informa que nada mudou (D-09).
3. O resumo de confirmação da edição usa formato **antes→depois** (diff dos campos alterados), carregando os valores atuais no `PendingEntryState` (`Prev*`), ex.: `Confirma? Valor R$ 50,00 → R$ 65,00; Categoria *Restaurante* → *Transporte*`.

## Alternativas Consideradas

- **Last-writer-wins (re-ler version e gravar):** mais fluido, mas pode sobrescrever mudanças concorrentes sem o usuário perceber. Rejeitada.
- **Falhar e pedir para recomeçar:** simples, porém pouco fluido e perde o contexto coletado. Rejeitada.
- **Resumo só do estado resultante:** menos código, mas não evidencia o que muda; maior chance de confirmação errônea. Rejeitada.

## Consequências

### Benefícios Esperados

- Preserva a segurança do optimistic lock e evita perda silenciosa (RF-21).
- Diff antes→depois reduz confirmação equivocada; no-op evita escrita/evento/version inúteis.

### Trade-offs e Custos

- Campos `Prev*` adicionais no estado (aditivos) e um segundo caminho de resumo (`buildEditConfirmSummary`).
- Re-suspend após conflito adiciona uma rodada extra de conversa quando há concorrência (rara para um único usuário).

### Riscos e Mitigações

- Risco: loop de re-confirmação sob concorrência persistente. Mitigação: TTL de confirmação (5 min) e reprompt único já existentes encerram o run.
- Rollback: reverter para `buildConfirmSummary` (estado resultante) e falha simples em conflito.

## Plano de Implementação

1. `PendingEntryState.Prev*` + `buildEditConfirmSummary`.
2. Tratamento de conflito no `executeWithIdempotency`/resume (re-ler + re-suspend).
3. No-op em `DecideUpdate` + resposta dedicada.
4. Testes unitários (conflito, no-op, diff) + integração de conflito.

## Monitoramento e Validação

- Métrica de escrita por `operation=edit_entry` com desfecho; span de conflito de version.
- Sucesso: conflito produz nova confirmação com valores correntes; no-op não gera evento.

## Impacto em Documentação e Operação

- Instruções do agente descrevem o resumo antes→depois; runbook cobre o caso de conflito.

## Revisão Futura

- Reavaliar se a taxa de conflito real justifica auto-merge de campos não conflitantes.
