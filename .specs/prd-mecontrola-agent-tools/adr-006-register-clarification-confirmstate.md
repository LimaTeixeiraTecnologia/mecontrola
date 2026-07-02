# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Clarificação de registro (categoria/data) reutilizando `ConfirmState` com
  `OperationKind` não-destrutivo `OpConfirmRegister`
- **Data:** 2026-07-02
- **Status:** Aceita
- **Decisores:** Autor da techspec, time de plataforma
- **Relacionados:** PRD `.specs/prd-mecontrola-agent-tools/prd.md` (RF-41, RF-42, RF-43; D-09;
  EP-04); techspec; ADR-001; ADR-005; R-AGENT-WF-001 (.1 / .7 / addendum .7-A); R-ADAPTER-001

## Contexto

A evidência de produção (PRD, `Evidência de Produção`, EP-04) mostrou atrito de confirmação
inconsistente no registro de lançamentos: no primeiro registro o agente pediu categoria e confirmação
de data; em outro registrou "instantâneo" sem categoria. A spec-version 3 reintroduz o fluxo de
clarificação de registro (D-09, RF-41..RF-43): a categoria deve ser perguntada **apenas quando ausente
ou ambígua** e a data deve ser resolvida por **default determinístico sem perguntar**.

Já existe o substrato de estado de espera fechado `ConfirmState`
(`internal/agents/application/workflows/confirm_state.go`) com `AwaitingKind` (`AwaitingNone`/
`AwaitingConfirm`), `OperationKind` (`OpDeleteEntry`/`OpEditEntry`/`OpDeleteCard`) e o workflow
`destructive-confirm` (`destructive_confirm_workflow.go`), que persiste o estado antes de perguntar,
retoma por merge-patch antes do parse e conclui o Run deterministicamente (TTL 5 min, reprompt único).

O addendum R-AGENT-WF-001.7-A marcou como **SUPERSEDED** (como caminho literal) o gate HITL do agent
financeiro anterior (`internal/agent`, removido); seu **contrato comportamental** permanece hard e deve
ser reemitido apontando para os arquivos reais do consumidor que reintroduzir um estado de espera.

## Decisão

Reutilizar o substrato `ConfirmState` para a clarificação de registro, **sem criar um mecanismo HITL
paralelo** (R-AGENT-WF-001.1), estendendo o enum fechado `OperationKind` com um valor **não-destrutivo**
dedicado `OpConfirmRegister` (com `String()`/`IsValid()`/`ParseOperationKind()`). O dispatch de
operações permanece por **mapa** `map[OperationKind]func(...)` — nunca `switch` de domínio — coerente com
ADR-001.

Regras do fluxo:

- **Categoria (RF-41).** Perguntar **apenas quando ausente ou ambígua** (não resolvida com confiança por
  `classify_category`). Quando a categoria é resolvida com confiança, o agente grava sem perguntar
  (RF-21 — pede apenas o dado faltante). O estado de espera de categoria usa `OpConfirmRegister`.
- **Data (RF-42).** Resolver por **default determinístico**: data corrente em `America/Sao_Paulo`,
  inferindo "hoje"/"ontem"/data relativa/data explícita quando o usuário indicar, **sem perguntar**;
  confirmação de data só quando genuinamente ambígua. Não há chamada de LLM para resolver a data.
- **Contrato de pending step (RF-43, R-AGENT-WF-001.7).** Persistir o `ConfirmState` no snapshot do
  kernel **antes** de perguntar; retomar por **merge-patch antes de qualquer parse** do inbound;
  concluir o Run deterministicamente (sem draft órfão), com a mesma semântica de TTL/reprompt/limpeza
  já implementada no workflow único.

`OpConfirmRegister` é não-destrutivo: sua efetivação é a própria escrita do lançamento (via tool de
registro), sujeita à injeção server-side de identidade/idempotência e ao guard de anti-simulação
(ADR-005). Não implica confirmação de operação destrutiva — apenas coleta o dado faltante (categoria)
ou desambigua a data antes de gravar.

Escopo: apenas `internal/agents`. O kernel `internal/platform/workflow` permanece intocado; estados
permanecem tipos fechados (DMMF state-as-type), nunca string livre (RTA-03).

## Reemissão do gate SUPERSEDED (R-AGENT-WF-001.7-A)

Esta decisão **reemite**, apontando para os arquivos reais deste consumidor, o contrato que o addendum
R-AGENT-WF-001.7-A marcou SUPERSEDED como caminho literal (o gate pertencia a `internal/agent`, removido):

- Estado de espera como **tipo fechado**: `AwaitingKind`/`OperationKind` em
  `internal/agents/application/workflows/confirm_state.go` (agora incluindo `OpConfirmRegister`).
- **Persistir antes de perguntar** e **retomar por merge-patch antes do parse**: workflow
  `internal/agents/application/workflows/destructive_confirm_workflow.go` (mesmo idioma de
  `edit_entry`/`delete_entry`).
- **Conclusão determinística** (sem draft órfão), TTL e reprompt único herdados do workflow único.

O gate executável correspondente a R-AGENT-WF-001.7 passa a valer sobre estes arquivos reais.

## Alternativas Consideradas

- **Criar um workflow de clarificação paralelo ao `destructive-confirm`.** Desvantagem: duplica
  semântica de pending step/TTL/reprompt/limpeza; viola R-AGENT-WF-001.1 (proibido mecanismo HITL
  paralelo) e R-AGENT-WF-001.7. Rejeitada.
- **Representar a clarificação como flag booleano/string no draft.** Desvantagem: viola state-as-type
  (RTA-03) e o contrato de estado fechado. Rejeitada.
- **Sempre perguntar categoria e data.** Desvantagem: é exatamente o atrito inconsistente do EP-04 e
  contraria RF-21/RF-41/RF-42. Rejeitada.
- **`switch` de domínio no dispatch por `OperationKind`.** Desvantagem: R-AGENT-WF-001.1 prefere mapa.
  Rejeitada.

## Consequências

### Benefícios Esperados

- Fim do atrito inconsistente (EP-04): categoria só quando necessário, data por default.
- Reuso de um pending step já endurecido (persistência antes de perguntar, resume antes do parse,
  limpeza determinística), sem nova superfície HITL.
- Aderência a R-AGENT-WF-001 (estados fechados, dispatch por mapa, resume antes do parse).

### Trade-offs e Custos

- O enum `OperationKind` e o mapa de dispatch crescem com um valor não-destrutivo (coesão vs. tamanho).
- É preciso distinguir, no dispatch, operação de confirmação **destrutiva** de **clarificação de
  registro** não-destrutiva, sem ramificar por domínio além do mapa.

### Riscos e Mitigações

- **Risco:** tratar `OpConfirmRegister` como destrutivo e exigir confirmação onde não deve.
  **Mitigação:** o valor é explicitamente não-destrutivo; sua efetivação é a escrita normal do
  lançamento; testes cobrem o caminho sem gate destrutivo.
- **Risco:** data resolvida errada por timezone. **Mitigação:** default determinístico em
  `America/Sao_Paulo`, sem LLM; teste de "hoje"/"ontem"/explícita.
- **Rollback:** remover a constante e a entrada do mapa reverte sem afetar as operações destrutivas
  existentes.

## Plano de Implementação

1. Estender `OperationKind` com `OpConfirmRegister` + `String()`/`IsValid()`/`ParseOperationKind()` em
   `confirm_state.go`.
2. Adicionar a entrada não-destrutiva ao mapa de dispatch (efetivação = escrita do lançamento),
   reutilizando o workflow `destructive-confirm` para persistência/resume/limpeza.
3. Resolver categoria via `classify_category` (só perguntar quando ausente/ambígua) e data por default
   determinístico `America/Sao_Paulo`.
4. Garantir injeção server-side de identidade/idempotência e guard de anti-simulação na escrita final
   (ADR-005).

## Monitoramento e Validação

- Cenários do harness real-LLM cobrindo: categoria confiável (grava sem perguntar), categoria ausente/
  ambígua (pergunta uma vez, resume antes do parse, grava), data "hoje"/"ontem"/explícita.
- Assert de linha real no banco após a confirmação (RF-29, ADR-005); nenhum draft órfão; Run concluído
  deterministicamente.
- Métrica de operações com cardinalidade controlada (RF-28); nenhum estado de espera representado como
  string livre.

## Impacto em Documentação e Operação

- Runbook do agente: documentar quando o agente pergunta categoria vs. grava direto, e a resolução de
  data por default.
- Atualizar o mapa capacidade→tool e o gate R-AGENT-WF-001.7 para apontar aos arquivos reais deste
  consumidor.

## Revisão Futura

- Reavaliar se o número de `OperationKind` no workflow único ultrapassar ~8 (sinal para segmentar,
  coerente com ADR-001).
- Reavaliar a política de desambiguação de data se surgirem formatos/idiomas adicionais.
