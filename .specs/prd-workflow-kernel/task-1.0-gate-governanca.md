# Tarefa 1.0: Gate de governança (R-WF-KERNEL-001 + addendum R-AGENT-WF-001)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Redigir a governança que distingue "kernel genérico de workflow" (mecanismo, permitido em
`internal/platform`) de "workflow de intent + Thread/Run/WorkingMemory/PendingStep semânticos"
(exclusivos de `internal/agent`). É **gate bloqueante**: nenhum código do kernel pode começar antes.

<requirements>
- RF-27: criar regra hard `R-WF-KERNEL-001` para o kernel genérico em `internal/platform`.
- RF-28: aditar `R-AGENT-WF-001` (.6 e .8) distinguindo kernel genérico de workflow de intent semântico.
- RF-29: a redação é gate — concluída antes de qualquer `.go` do kernel.
- Ver ADR-004 (`adr-004-governance-gate.md`).
</requirements>

## Subtarefas

- [ ] 1.1 Criar `.claude/rules/workflow-kernel.md` (`R-WF-KERNEL-001`, hard): kernel sem import de
  domínio (`intent`/`agent`/`transactions`), sem regra/SQL/branching de domínio, estados fechados
  (`RunStatus`/`StepStatus`/`SuspendReason`), cardinalidade controlada (sem `user_id`/`correlation_key`/
  `category_id`), LLM proibido. Incluir gates de verificação (grep) executáveis.
- [ ] 1.2 Aditar `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.6/.8) com addendum que mantém
  os 4 conceitos semânticos exclusivos do agent e permite o agent **consumir** o kernel.
- [ ] 1.3 Referenciar a nova regra em `.claude/rules/governance.md` (seção "Regras de Modulo") e em
  `CLAUDE.md`/`AGENTS.md` (referência de módulo).

## Detalhes de Implementação

Ver techspec.md → "Considerações Técnicas / Conformidade com Padrões" e ADR-004. Espelhar o estilo e os
gates de verificação de `.claude/rules/go-adapters.md` e `.claude/rules/agent-workflows-tools.md`.

## Critérios de Sucesso

- `R-WF-KERNEL-001` existe com gates grep executáveis (imports de domínio, comentários, SQL direto, labels).
- Addendum em R-AGENT-WF-001.6/.8 distingue mecanismo (kernel) de semântica (agent), sem reabrir brecha.
- `governance.md` lista a nova regra; precedência DMMF preservada.
- Nenhum arquivo `.go` do kernel criado nesta tarefa (gate documental).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Validação: executar os gates grep declarados em `R-WF-KERNEL-001` e confirmar retorno vazio no estado
atual; revisar que os gates de `R-AGENT-WF-001` continuam passando.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.claude/rules/workflow-kernel.md` (novo)
- `.claude/rules/agent-workflows-tools.md` (addendum .6/.8)
- `.claude/rules/governance.md` (índice de Regras de Modulo)
- `CLAUDE.md`, `AGENTS.md` (referência de módulo)
