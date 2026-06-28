# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Catálogo canônico de capabilities como fonte única de verdade
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** Time de plataforma (agent)
- **Relacionados:** PRD `.specs/prd-agent-capability-catalog/prd.md` (RF-01..06), techspec `.specs/prd-agent-capability-catalog/techspec.md`, roadmap `docs/plans/2026-06-25-mastra-gap-map-mecontrola.md` (Fase 2), `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001), `.claude/rules/workflow-kernel.md` (R-WF-KERNEL-001).

## Contexto

O `internal/agent` descreve cada `intent.Kind` em múltiplos lugares: o registry de execução (`buildRegistry()` → `IntentRegistry`), o switch de auditoria (`workflowFor`/`toolFor`), o mapa destrutivo (`intentToOperationKind`) e a descrição mínima em `ToolSpec{Name,IntentKind,Description}`. Não há um lugar único que descreva uma capability com sua classificação operacional (leitura/escrita, confirmação humana, suspend/resume, canais, chave de métrica). Isso gera drift (ver ADR-002), DX fraca para adicionar capability e impossibilidade de listar programaticamente as capacidades do agente para auditoria/console/evals.

Restrições: a semântica de capability é exclusiva de `internal/agent` (R-WF-KERNEL-001 proíbe vazar para o kernel genérico); estado deve ser tipo fechado (DMMF / R-AGENT-WF-001.3); sem novo `case intent.Kind` de domínio (R-AGENT-WF-001.1); zero comentários em Go (R-ADAPTER-001.1).

## Decisão

Introduzir um **catálogo canônico** em `internal/agent/application/capability/`:

- `CapabilitySpec` — struct rica: `ID`, `Description`, `Kind`, `WorkflowID`, `ToolName`, `Mode` (tipo fechado `CapabilityMode`), `RequiresConfirmation`, `SupportsSuspend`, `SupportsResume`, `Channels`, `MetricsKey`.
- `CapabilityMode` — tipo fechado (`ModeRead`/`ModeWrite`) com `String`/`IsValid`/`ParseCapabilityMode`.
- `Catalog` — coleção imutável construída via smart constructor `NewCatalog(specs...)` (valida unicidade de `ID`/`Kind`, `Mode` válido, `WorkflowID` não-vazio); expõe `Lookup(kind)`, `List()`, `Classify(kind)`.
- `BuildCatalog(...)` — declara as 24 capabilities (19 roteáveis + `KindUnknown` + 5 destrutivas HITL), invocada no **mesmo seam de wiring** que monta o registry (`module.go`/`buildRegistry`), garantindo registro único alimentando execução e classificação.

O catálogo é a fonte única de verdade para classificação operacional; o `IntentRegistry` permanece responsável apenas pela resolução de execução, derivando do mesmo conjunto de `WorkflowID`.

## Alternativas Consideradas

- **Anexar `CapabilitySpec` diretamente a `KindTool`/`NewIntentWorkflow`:** acopla metadata ao construtor de execução; menos flexível para incluir kinds destrutivos (que não passam pelo registry). Rejeitada no MVP — mantida como evolução futura (unificar declaração num ponto só).
- **Enriquecer o `ToolSpec` existente (`tools/registry.go`):** `ToolSpec` é por-tool e não cobre workflow owner nem kinds destrutivos; ampliá-lo misturaria responsabilidades. Rejeitada.
- **Manter o status quo (switch + mapas):** perpetua o drift documentado em ADR-002 e impede introspecção. Rejeitada (é o problema que motiva o PRD).

## Consequências

### Benefícios Esperados
- Fonte única de verdade; kinds novos herdam classificação correta automaticamente.
- Listagem programática habilita auditoria de cobertura, console e evals (fases futuras) sem reabrir governança.
- DX melhor: adicionar capability = declarar uma `CapabilitySpec`.

### Trade-offs e Custos
- Mais um artefato a manter junto ao wiring; mitigado por teste de consistência registry↔catálogo (ADR-002, R2).
- Pequena indireção no boot (construção do catálogo).

### Riscos e Mitigações
- **Risco:** catálogo e registry divergirem no `WorkflowID`. **Mitigação:** teste de consistência para todo kind roteável; ambos derivam dos mesmos IDs. **Rollback:** reverter para `workflowFor`/`toolFor` é trivial (a derivação é isolada no `AgentRuntime`).

## Plano de Implementação
1. `CapabilityMode` + `CapabilitySpec` + `NewCatalog` (domínio puro, testado).
2. `BuildCatalog` com as 24 specs + teste-guard de cobertura e consistência.
3. Wiring injeta o catálogo no `AgentRuntime` (ADR-002) e no `DailyLedgerAgent` (ADR-003).

## Monitoramento e Validação
- Teste-guard de cobertura (`routableKinds()` ⊆ catálogo) verde.
- Teste de consistência registry↔catálogo verde.
- Log de boot listando o catálogo (contagem por workflow/mode).

## Impacto em Documentação e Operação
- Skill `mastra` atualizada (RF-14/15) com o catálogo como passo obrigatório de extensão.
- Sem mudança de runbook/infra.

## Revisão Futura
- Revisitar quando a Fase 3 (console) ou Fase 4 (evals) consumir o catálogo, ou se a manutenção de duas declarações (catálogo + registry) provar atrito — então unificar a declaração num ponto único.
