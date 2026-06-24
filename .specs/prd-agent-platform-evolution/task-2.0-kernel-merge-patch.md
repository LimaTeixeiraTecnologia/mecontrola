# Tarefa 2.0: Kernel merge-patch no resume (ADR-001)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Corrigir o defeito latente de perda de estado no `Engine.Resume` do kernel
(`internal/platform/workflow`): trocar a **substituição** do estado inteiro pelo payload de resume
por **JSON merge-patch** (delta aplicado sobre `Snapshot.State`). Fundacional para o suspend/resume
correto do HITL e benéfico para todos os consumidores do kernel. Mantém o kernel genérico
(R-WF-KERNEL-001).

<requirements>
- RF-09: resume idempotente e sobrevivente a restart/crash — não duplica efeitos confirmados.
- RF-10: segurança sob resume concorrente (lock otimista por versão preservado).
- RF-24: kernel permanece genérico — merge opera sobre JSON, sem import/regra de domínio.
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar `Codec[S].MergePatch(base, patch []byte) ([]byte, error)` em `codec.go` (merge recursivo de objetos JSON; chave `null` remove; genérico).
- [ ] 2.2 Substituir o bloco de replace em `Engine.Resume` (`engine.go`) por `MergePatch(snap.State, resume)` + `Decode`; resume vazio permanece no-op.
- [ ] 2.3 Teste de regressão do defeito: suspender com estado rico → resume com `{"ResumeText":"x"}` → asserts de que os campos originais sobrevivem.
- [ ] 2.4 Garantir `parity_test.go` e o fluxo de clarificação de categoria (caminho kernel) verdes.

## Detalhes de Implementação

Ver `techspec.md` seção "Interfaces Chave" (snippet de `MergePatch` e do novo bloco de Resume) e
ADR-001 (decisão, alternativas rejeitadas incluindo `ResumeApplier[S]`). O CAS de versão
(`saveSnap`/`Save`) NÃO muda. Zero comentários (R-ADAPTER-001.1).

## Critérios de Sucesso

- `Engine.Resume` aplica merge-patch; estado suspenso sobrevive ao resume com delta mínimo.
- Resume vazio (`len(resume)==0`) é no-op idêntico ao comportamento anterior.
- Teste de regressão do defeito passa; `parity_test.go` e testes existentes do kernel verdes.
- Gate `R-WF-KERNEL-001` retorna vazio (sem domínio, sem SQL fora do adapter, estados fechados,
  cardinalidade controlada).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — `codec_test.go` (merge/sobrescrita/null-remove/vazio) e `engine_test.go` (regressão do defeito).
- [ ] Testes de integração — round-trip real no store (estende `store_integration_test.go`): suspende → resume com delta → efeito único.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/workflow/codec.go` (modificado)
- `internal/platform/workflow/engine.go` (modificado — bloco de Resume)
- `internal/platform/workflow/codec_test.go`, `engine_test.go` (novos/estendidos)
- `internal/platform/workflow/infrastructure/postgres/store_integration_test.go` (estendido)
- `internal/agent/application/workflow/parity_test.go` (verde — não regressão)
