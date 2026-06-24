# Tarefa 1.0: Gate de governança — addendum R-AGENT-WF-001.7 (AwaitingApproval) + nota merge-patch

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Redigir a regra de governança ANTES de qualquer código do HITL (gate-first), espelhando o padrão
ADR-004/task-1.0 do `prd-workflow-kernel`. Estende `R-AGENT-WF-001.7` para cobrir o estado de espera
fechado `AwaitingApproval` (além do `AwaitingKind` de categoria) e registra a nota do contrato de
resume via JSON merge-patch em `workflow-kernel.md`.

<requirements>
- RF-21: roteamento HITL não pode crescer `case intent.Kind` no switch (codificar no addendum).
- RF-22: Tool/passos finos — sem regra/SQL/branching de domínio; LLM só no parse.
- RF-23: `OperationKind`/`AwaitingApproval` como tipos fechados (DMMF state-as-type).
- RF-24: fronteira kernel-genérico vs agent-semântico preservada (merge-patch não vaza domínio).
- RF-25: cardinalidade de métrica controlada (labels só de enums fechados; sem user_id/category_id).
- RF-27: zero comentários em Go de produção e demais checklists R0–R7 reforçados.
</requirements>

## Subtarefas

- [ ] 1.1 Estender `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001.7): salvar estado de espera `AwaitingApproval` (tipo fechado) antes de retornar confirmação; resume antes do `ParseInbound`; limpeza determinística após efetivar/cancelar/expirar.
- [ ] 1.2 Adicionar nota em `.claude/rules/workflow-kernel.md`: contrato de resume via JSON merge-patch (delta sobre snapshot), genérico, sem domínio.
- [ ] 1.3 Referenciar ADR-001/002/003/004 nos pontos pertinentes das regras.
- [ ] 1.4 Atualizar `.claude/rules/governance.md` (mapa de regras) se necessário para citar o addendum.

## Detalhes de Implementação

Ver `techspec.md` seção "Conformidade com Padrões" e ADR-003 (contrato de confirmação). NÃO duplicar
conteúdo — referenciar os ADRs. O addendum deve ser auto-verificável (gate `grep` quando aplicável,
no padrão dos demais itens R-AGENT-WF-001).

## Critérios de Sucesso

- Addendum R-AGENT-WF-001.7 redigido cobrindo `AwaitingApproval` como tipo fechado, com proibição de
  string livre e exigência de persistência do estado de espera + resume antes do parse.
- Nota de merge-patch presente em `workflow-kernel.md` deixando claro que o resume é genérico (delta
  JSON) e não vaza domínio.
- Nenhuma alteração de código de produção nesta tarefa (apenas regras/governança).
- Coerência com ADR-001..004 (sem contradição).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários — não aplicável (alteração documental/regra); validar gates `grep` do addendum retornam vazio sobre o código atual.
- [ ] Testes de integração — não aplicável.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.claude/rules/agent-workflows-tools.md` (modificado — addendum .7)
- `.claude/rules/workflow-kernel.md` (modificado — nota merge-patch)
- `.claude/rules/governance.md` (modificado se necessário)
- `.specs/prd-agent-platform-evolution/adr-001..004` (referência)
