# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

> **Feature:** Catálogo canônico de capabilities do `internal/agent` — fonte única de verdade para roteamento, auditoria, métricas e documentação do agente, eliminando o drift entre o registry de execução e o mapeamento manual de auditoria/métricas.
>
> **Fonte canônica de produto:** `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md` (Fase 1 — "corrigir a base de verdade"; Fase 2 — "capability catalog introspectável").
> **Fonte canônica de arquitetura/governança:** `AGENTS.md`, `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001), `.claude/rules/go-adapters.md` (R-ADAPTER-001).
> **Status:** discovery concluído; pronto para techspec.

---

## Visão Geral

O `internal/agent` já tem um núcleo de runtime forte: `Thread`/`Run` auditáveis, workflow durável com suspend/resume, tools como adapters finos e gates de confirmação humana. O gap não está no core de execução — está na **fonte de verdade** que descreve as capacidades do agente.

Hoje existem **duas fontes desacopladas** para a mesma informação:

1. **Execução** — `IntentRegistry.Resolve(kind)` resolve qual workflow/tool atende cada `intent.Kind` (`internal/agent/application/workflow/registry.go`).
2. **Auditoria/métricas** — funções manuais `workflowFor(kind)` e `toolFor(kind)` (`internal/agent/application/services/agent_runtime.go`) derivam, por um `switch` paralelo, os labels `workflow`/`tool` gravados no `Run` e nas métricas Prometheus.

Esses dois mecanismos **não são sincronizados**. Quando um novo `intent.Kind` é adicionado ao registry de execução mas esquecido no `switch` de auditoria, o agente executa corretamente mas **mente nas métricas e no audit trail** — e o review passa se o teste não cobrir aquele kind específico. Além disso, conhecimento operacional crítico (uma capability é escrita? exige confirmação humana? pode suspender/resumir? em quais canais?) está **espalhado** por funções ad hoc (`isDestructiveKind`, o map `intentToOperationKind`, branching no `daily_ledger_agent.go`), sem um lugar único e introspectável.

Por fim, a skill `mastra` — a documentação canônica para estender o agente — está em **drift**: afirma que `buildRegistry()` é o único seam de extensão, quando o código real já exige entender também o kernel de escrita, o confirmation engine, o plan executor e a ordem da cadeia de resume. Quem segue a skill pode implementar no lugar errado mesmo "seguindo a regra".

Este PRD define o produto-alvo: um **catálogo canônico de capabilities** (`CapabilitySpec` rico, registrado junto ao binding de cada tool/workflow) que vira a **fonte única de verdade**. O runtime passa a **derivar** classificação de auditoria/métricas desse catálogo (matando o `switch` paralelo); o catálogo é **listável programaticamente** (base para auditoria, futuros console e evals); e a skill `mastra` + um checklist de extensão objetivo passam a refletir os seams reais. É a fundação ("corrigir a base de verdade") sobre a qual as fases seguintes do roadmap (console, evals, memory 2.0, multi-agent, capabilities externas) serão construídas — e que **não** entram neste PRD.

**Valor:** elimina drift entre execução, observabilidade e documentação; faz kinds novos herdarem auditoria correta automaticamente; reduz custo de manutenção e risco de leitura errada de produção; e prepara terreno introspectável para console e evals sem reabrir regras de governança.

---

## Objetivos

- **O1 — Fonte única de verdade:** uma só estrutura canônica descreve cada capability do agente (id, descrição, kind, workflow, tool, modo leitura/escrita, confirmação, suspend/resume, canais, chave de métrica). Nenhuma informação operacional de capability vive duplicada em `switch`/maps ad hoc.
- **O2 — Auditoria e métricas derivadas (zero drift):** os labels `workflow`/`tool` do `Run` e das métricas passam a derivar do catálogo. Nenhum `intent.Kind` roteável pode ficar fora do mapeamento de auditoria/métricas — adicionar um kind sem `CapabilitySpec` falha em teste, não silenciosamente em produção.
- **O3 — Introspecção programática:** é possível listar todas as capabilities do agente em runtime (com sua classificação operacional mínima), servindo de base para auditoria, testes de cobertura e futuras superfícies (console/evals).
- **O4 — Documentação fiel ao código:** a skill `mastra` deixa de afirmar que `buildRegistry()` é o único seam e passa a declarar os seams reais; existe um checklist objetivo de extensão para os pontos de evolução do agente.
- **O5 — Conformidade arquitetural e não-regressão:** a mudança respeita R-AGENT-WF-001 (roteamento canônico, tipos fechados, sem novo `case` de domínio, cardinalidade de métricas), R-WF-KERNEL-001 (semântica de domínio fora do kernel), R-ADAPTER-001 (zero comentários, adapters finos) e DMMF (state-as-type). Todos os testes de agent/workflow permanecem verdes; nenhum gate HITL existente regride.

### Métricas de sucesso (mensuráveis)

- **MS-01 — Cobertura total:** 100% dos `intent.Kind` roteáveis (`routableKinds()`) têm `CapabilitySpec` no catálogo, verificado por teste-guard que falha o build em ausência.
- **MS-02 — Eliminação do drift estrutural:** 0 ocorrências de fonte paralela de classificação de auditoria/métrica fora do catálogo (`workflowFor`/`toolFor` removidos ou reduzidos a derivação do catálogo); verificado por inspeção de código e ausência de `switch intent.Kind` de auditoria.
- **MS-03 — Estabilidade de labels:** os valores de label `workflow`/`tool` emitidos para cada kind existente permanecem idênticos antes/depois (sem quebra de dashboards/alertas), validado por teste de equivalência por kind.
- **MS-04 — Cardinalidade controlada:** nenhum label de métrica do agente carrega `user_id`/`correlation_key`/`category_id`; labels restritos a `agent_id`/`channel`/`workflow`/`status`/`tool`/`outcome` (R-AGENT-WF-001.5 / R-TXN-004).
- **MS-05 — Fidelidade da skill:** a skill `mastra` cita os 5 seams reais (registry, kernel write, confirm, plan, resume chain) e o checklist de extensão cobre os 6 pontos de evolução; verificável por leitura da skill atualizada.
- **MS-06 — Não-regressão:** suíte completa de testes de `internal/agent` e `internal/platform/workflow` permanece verde após a mudança.

---

## Histórias de Usuário

> Feature de plataforma/DX (developer & operator experience). O "usuário" é o time de engenharia e operação do agente.

- **US-01 (Engenheiro adicionando capability):** Como engenheiro que adiciona um novo `intent.Kind`, quero declarar a capability num único lugar canônico, para que execução, auditoria, métricas e docs fiquem corretas automaticamente, sem precisar atualizar um `switch` paralelo escondido.
- **US-02 (Operador/SRE):** Como operador, quero confiar que o label `workflow`/`tool` das métricas e do audit trail reflete o que realmente executou, para diagnosticar produção sem suspeitar de classificação mentirosa.
- **US-03 (Auditoria de cobertura):** Como mantenedor, quero que o build falhe se um kind roteável não tiver classificação operacional, para que drift de auditoria não passe em review por falta de teste.
- **US-04 (Introspecção):** Como engenheiro de plataforma, quero listar programaticamente todas as capabilities e sua classificação (leitura/escrita, confirmação, resumível, canais), para alimentar auditoria, testes e futuras superfícies (console/evals) a partir de uma fonte só.
- **US-05 (Onboarding técnico):** Como novo integrante do time, quero que a skill `mastra` descreva os seams reais de extensão (registry, kernel, confirm, plan, resume chain) e um checklist objetivo, para implementar no lugar certo sem falso positivo de governança.

---

## Funcionalidades Core

### FC-01 — Catálogo canônico de capabilities (`CapabilitySpec`)
Cada tool/workflow roteável é registrado com uma especificação rica e tipada contendo, no mínimo: `id`, `description`, `intent.Kind`, `workflowID`, `toolName`, `mode` (leitura/escrita — tipo fechado), `requiresConfirmation`, `supportsSuspend`, `supportsResume`, `channels`, `metricsKey`. O catálogo é populado a partir do mesmo wiring que hoje monta o registry (`buildRegistry()`), de modo que **um único registro** alimenta execução e classificação — sem segunda fonte.

### FC-02 — Consulta e listagem programática
O catálogo expõe um ponto de consulta por `intent.Kind` (`Lookup`) e uma listagem completa das capabilities registradas, com sua classificação operacional. Essa superfície é a base canônica para auditoria de cobertura, testes e futuras superfícies (não construídas aqui).

### FC-03 — Runtime deriva auditoria/métricas do catálogo
O `AgentRuntime` passa a obter `workflow`/`tool` (labels de `Run`, span e métricas) consultando o catálogo, eliminando `workflowFor`/`toolFor` como fonte de verdade. Kinds não encontrados caem num default conversacional conhecido, preservando os valores de label atuais por kind existente.

### FC-04 — Migração do conhecimento operacional espalhado
Sinais hoje dispersos (ex.: se um kind é destrutivo/sensível — `isDestructiveKind`/`intentToOperationKind`; se é escrita) passam a derivar do `CapabilitySpec` (`requiresConfirmation`, `mode`), sem regressão dos gates HITL e do kernel de escrita existentes.

### FC-05 — Skill `mastra` fiel + checklist de extensão
A skill `mastra` é atualizada para declarar os seams reais de evolução e um checklist objetivo de extensão cobrindo: novo kind, nova tool, novo workflow, novo pending state, novo gate de confirmação, novo plan step.

### FC-06 — Guarda de cobertura (teste-guard)
Um teste garante que todo kind roteável (`routableKinds()`) tem `CapabilitySpec` e que os labels derivados batem com a classificação esperada — fechando o caminho para drift futuro em review.

---

## Requisitos Funcionais

**Fase 2 — Catálogo canônico introspectável**

- RF-01: O sistema DEVE definir uma estrutura canônica `CapabilitySpec` com, no mínimo, os campos: `ID`, `Description`, `Kind` (`intent.Kind`), `WorkflowID`, `ToolName`, `Mode` (leitura/escrita), `RequiresConfirmation`, `SupportsSuspend`, `SupportsResume`, `Channels`, `MetricsKey`.
- RF-02: `Mode` (e qualquer enum de classificação introduzido) DEVE ser um tipo fechado (DMMF state-as-type) com constantes enumeradas e validação; nunca `string` livre em assinatura pública.
- RF-03: O sistema DEVE registrar uma `CapabilitySpec` por capability roteável a partir do mesmo wiring que monta o registry de execução, garantindo registro único que alimenta execução e classificação.
- RF-04: O catálogo DEVE expor consulta por `intent.Kind` retornando a `CapabilitySpec` e um indicador de presença (`Lookup(kind) (CapabilitySpec, bool)`).
- RF-05: O catálogo DEVE expor listagem programática de todas as capabilities registradas, com sua classificação operacional mínima.
- RF-06: Cada `CapabilitySpec` registrada DEVE carregar a classificação operacional mínima: modo leitura/escrita, exigência de confirmação humana, capacidade de suspender e de resumir.

**Fase 1 — Runtime deriva da fonte canônica (zero drift)**

- RF-07: O `AgentRuntime` DEVE derivar os labels `workflow` e `tool` (para `Run`, span e métricas) consultando o catálogo canônico, e NÃO mais de um `switch`/mapa manual paralelo.
- RF-08: As funções de classificação manual `workflowFor`/`toolFor` (ou equivalentes) DEVEM ser removidas como fonte de verdade; se mantidas, apenas como finos delegadores que derivam do catálogo.
- RF-09: Para todo `intent.Kind` existente, o valor de label `workflow`/`tool` emitido após a mudança DEVE ser idêntico ao emitido antes (sem quebra de dashboards/alertas existentes).
- RF-10: Nenhum `intent.Kind` roteável (`routableKinds()`) pode ficar sem `CapabilitySpec`; a ausência DEVE ser detectada por teste-guard que falha o build, não silenciosamente em produção.
- RF-11: Kinds não-roteáveis/desconhecidos DEVEM cair num default conversacional conhecido, preservando o comportamento atual de classificação para o caminho conversacional.
- RF-12: A classificação de operação destrutiva/sensível e de escrita usada pelos gates HITL e pelo kernel de escrita DEVE derivar do `CapabilitySpec` (ex.: `RequiresConfirmation`, `Mode`), sem regressão dos gates existentes nem novo `case intent.Kind` de domínio no `daily_ledger_agent.go`.
- RF-13: As métricas do agente DEVEM manter cardinalidade controlada: labels restritos a `agent_id`, `channel`, `workflow`, `status`, `tool`, `outcome`; proibido `user_id`, `correlation_key` ou `category_id`.

**Documentação e governança de extensão**

- RF-14: A skill `mastra` DEVE ser atualizada para deixar de afirmar que `buildRegistry()` é o único seam e passar a declarar os seams reais de evolução: (1) registry, (2) kernel write path, (3) confirmation engine, (4) plan executor, (5) resume chain ordering.
- RF-15: A documentação de extensão DEVE conter um checklist objetivo com passos para: novo kind, nova tool, novo workflow, novo pending state, novo gate de confirmação, novo plan step — incluindo o passo obrigatório de registrar a `CapabilitySpec`.

**Não-regressão e verificação**

- RF-16: A suíte de testes de `internal/agent` e `internal/platform/workflow` DEVE permanecer verde após a mudança.
- RF-17: DEVE existir teste de equivalência por kind validando que os labels derivados do catálogo correspondem aos labels legados para todos os kinds existentes (suporte a MS-03).

---

## Restrições Técnicas de Alto Nível

- **Bounded context:** toda a mudança vive em `internal/agent`. O catálogo e sua semântica são **exclusivos do agente** — proibido vazar para o kernel genérico `internal/platform/workflow` (R-WF-KERNEL-001: sem import/semântica de domínio no kernel).
- **Roteamento canônico (R-AGENT-WF-001.1):** nenhum novo `case intent.Kind` de domínio no `switch` de `daily_ledger_agent.go`; a resolução continua via registry, e o catálogo é a metadata que acompanha o mesmo wiring.
- **Tipos fechados (DMMF / R-AGENT-WF-001.3):** `Mode` e quaisquer enums de classificação são tipos fechados; `ToolOutcome`/`RunStatus` permanecem fechados.
- **Cardinalidade de métricas (R-AGENT-WF-001.5 / R-TXN-004):** labels permitidos apenas os enumerados; sem alta cardinalidade.
- **Zero comentários em Go de produção (R-ADAPTER-001.1):** todos os arquivos `.go` novos/alterados sem comentários, salvo exceções sancionadas.
- **Compatibilidade de observabilidade:** valores de label por kind preservados; mudança não pode exigir reconfiguração de dashboards/alertas existentes.
- **LLM inalterado:** esta mudança não toca o ponto de parse LLM (R-AGENT-WF-001.4); é puramente estrutural de classificação/roteamento e documentação.

---

## Fora de Escopo

As fases 3–7 do roadmap `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md` **não** fazem parte deste PRD:

- **Console operacional do agent** (UI/studio para threads/runs/replay/resume manual) — Fase 3.
- **Evals semânticos first-class** (datasets versionados, runner, thresholds) — Fase 4.
- **Memory 2.0** (separar perfil/thread/fatos derivados, TTL/curadoria) — Fase 5.
- **Workspaces / multi-agent** (desacoplar o `DailyLedgerAgent`) — Fase 6.
- **Runtime uniforme para capabilities externas** (estilo MCP: timeout/retry/auth/policy padronizados) — Fase 7.
- **Observabilidade semântica de IA** (dashboards por confidence/clarify/confirm/cost) além do que o catálogo habilita estruturalmente.

Não-objetivos globais herdados do roadmap (linhas 673–681): não reescrever o agent em TypeScript; não copiar APIs do Mastra; não enfraquecer bounded context/DDD; não mover semântica de domínio para o kernel genérico; não substituir WhatsApp por abstração genérica prematuramente.

(Nota: riscos de implementação técnica — ex.: drift entre `Resolve` de execução e `Lookup` de auditoria, estratégia de migração de `isDestructiveKind`/`intentToOperationKind` — serão detalhados na Especificação Técnica via ADRs.)

---

## Decisões Travadas (questões resolvidas — 2026-06-25)

Todas as questões em aberto foram resolvidas com o usuário (múltipla escolha); o SDD está sem lacunas:

- **D-01 (topologia — S-01):** O catálogo é declarado em `BuildCatalog` **paralelo** ao registry, alimentado pelo mesmo wiring, com **teste de consistência** garantindo `catalog.WorkflowID(kind) == owner do registry` para todo kind roteável (fecha o risco de dupla fonte, R2). Rejeitadas: `Spec` dentro de `KindTool` (refatora assinatura do registry) e enriquecer `ToolSpec` (mistura responsabilidades). Detalhe em ADR-001.
- **D-02 (drift — S-02):** Os 4 kinds com drift confirmado (`KindQueryIncomeSummary`→`transactions`, `KindBudgetRecurrence`→`budget`, `KindDeleteTransactionByRef`/`KindEditTransactionByRef`→destrutivo) têm o label **corrigido** para o workflow real (não preservado). A correção é explícita via teste de equivalência por kind (lista de exceções) e comunicada no PR. Demais kinds: label idêntico ao atual (RF-09). Detalhe em ADR-002.
- **D-03 (channels — S-03):** `Channels` no MVP é `["whatsapp"]` (canal único atual), mantido como campo para evolução futura sem reabrir contrato.
- **D-04 (metricsKey — Q-01):** `MetricsKey` **espelha** `ToolName` por padrão (preserva MS-03 e o label `tool` atual). Campo permanece para divergência futura, sem uso divergente no MVP.
- **D-05 (legado — RF-08):** `workflowFor`/`toolFor` são **removidos de vez**; o `AgentRuntime` usa exclusivamente `catalog.Classify`. Sem superfície redundante (MS-02).
- **D-06 (destrutivo — Q-02):** `isDestructiveKind` deriva de `CapabilitySpec.RequiresConfirmation`; o mapa `intentToOperationKind` permanece como tradução `intent.Kind → OperationKind` (tipo fechado, R-AGENT-WF-001.7-A), com teste de consistência catálogo↔mapa e sem regressão dos gates HITL. Detalhe em ADR-003.
