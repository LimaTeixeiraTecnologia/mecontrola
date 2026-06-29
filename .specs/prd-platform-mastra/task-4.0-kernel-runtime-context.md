# Tarefa 4.0: Evolução mínima do kernel `internal/platform/workflow` (RuntimeContext)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Evoluir o kernel existente `internal/platform/workflow` de forma mínima e não-disruptiva: tornar explícito o repasse de um `RuntimeContext` tipado (DI) via `context.Context` aos steps, sem persistir, e habilitar agent/tools como `Step[S]` componíveis. Preserva integralmente `R-WF-KERNEL-001` (kernel puro, sem LLM/domínio, estados fechados, merge-patch). O kernel é aproveitado, não reescrito (ADR-001).

<requirements>
- RF-11: workflows por steps tipados com encadeamento determinístico (já existente — preservar).
- RF-12: estado compartilhado `S` entre steps (já existente — preservar).
- RF-13: agents/tools como steps componíveis (suporte explícito).
- RF-14: combinadores `Sequence/Branch/Parallel/Retry` preservando `S` (já existentes — preservar).
- RF-26: runtime context tipado acessível a steps via `context.Context`.
- RF-27: runtime context NÃO persistido no snapshot (`Codec` continua codificando só `S`).
- Sem violar nenhum gate de `R-WF-KERNEL-001`: sem import de domínio/camada superior, sem LLM, estados fechados.
</requirements>

## Subtarefas

- [ ] 4.1 Definir `Runtime`, `WithRuntime(ctx, rc)`, `RuntimeFrom(ctx)` (chave de tipo privada) em pacote da plataforma (kernel ou `internal/platform/agent`, conforme ADR-007).
- [ ] 4.2 Garantir que `Engine.execute/runStep` repassem o `ctx` (já repassam) sem serializar runtime context; `Codec.Encode` continua só sobre `S`.
- [ ] 4.3 Teste de não-persistência: runtime context disponível no step durante execução; ausente após `Resume` a partir do snapshot.

## Detalhes de Implementação

Ver techspec.md "Considerações > ADR-001/ADR-007" e os arquivos atuais `engine.go`, `step.go`, `codec.go`. Mudança aditiva; não alterar assinatura pública de `Step[S]` (usar `context.Context`). Não introduzir LLM nem import de `internal/platform/{agent,memory,llm,scorer,tool}` no kernel.

## Critérios de Sucesso

- Suíte existente do kernel (`engine_test.go`, `codec_test.go`, `combinators_test.go`) permanece verde.
- Novo teste prova que runtime context não aparece no `Snapshot.State` após suspend/resume.
- Gate: `grep -rn "openai\|anthropic\|openrouter\|llm\.\|internal/platform/agent\|internal/platform/memory\|internal/platform/llm\|internal/platform/scorer\|internal/platform/tool" internal/platform/workflow/ --include="*.go" --exclude="*_test.go"` retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — alteração de código Go no kernel (generics/concorrência) obrigatória (CLAUDE.md).
- `mastra` — workflow/Run/suspend-resume são primitivos do padrão Mastra; RuntimeContext espelha runtimeContext.

## Testes da Tarefa

- [ ] Testes unitários: repasse de runtime context ao step; não-persistência pós-resume; regressão dos combinadores.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/workflow/engine.go`, `step.go`, `codec.go`, `combinators.go` — kernel evoluído (aditivo).
- `internal/platform/agent/` (ou pacote `runtime`) — `WithRuntime`/`RuntimeFrom` (ADR-007).
