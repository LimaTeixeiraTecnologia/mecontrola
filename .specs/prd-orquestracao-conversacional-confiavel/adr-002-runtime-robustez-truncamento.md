# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Robustez do runtime — truncamento como estado fechado, falha-segura e observabilidade dos gaps de persistência
- **Data:** 2026-07-09
- **Status:** Aceita
- **Decisores:** Plataforma / dono do agente MeControla
- **Relacionados:** `prd.md` (RF-22..RF-28, RF-47), `techspec.md`, US-001

## Contexto

A investigação confirmou gaps operacionais silenciosos no runtime da plataforma agente:

- **Truncamento por length ignorado no runtime:** `llm.Response.TruncatedByLength` (`llm/types.go:44-51`)
  é propagado até `agent.Result.TruncatedByLength` (`agent/ports.go:56-63`, preenchido em
  `agent/agent.go:145-152`) e apenas logado com `Warn` em `agent.go:154-159`; o runtime nunca consulta o
  campo — uma resposta truncada (ex.: resumo C1–C7 longo) é tratada como sucesso.
- **`RunStore.Update` engolido:** `runtime.go:308` faz `_ = r.runs.Update(ctx, run)` e ainda emite as
  métricas de run como se o estado tivesse persistido (`runtime.go:309-315`).
- **`MessageStore.Append` sem métrica:** `runtime.go:168-193` apenas loga `Warn` em falha; sem métrica,
  sem critério de alerta.
- **Só o primeiro erro de tool:** `firstToolErrorContent(result.ToolCalls)` captura apenas a primeira
  falha (`runtime.go:201-210`).
- **Teto de tokens baixo:** `mecontrolaAgentDefaultMaxTokens = 1536` (`mecontrola_agent.go:13`) causa
  truncamento falso em respostas longas legítimas.

`ToolOutcome` e `RunStatus` são tipos fechados (`types.go:48-101`, iota+1). A decisão do usuário para
truncamento: **falha-segura + elevar o teto de tokens**.

## Decisão

Endurecer `internal/platform/agent` por **extensão aditiva** (sem reescrever o substrato; o kernel
`internal/platform/workflow` permanece intocado):

1. **Truncamento como estado fechado:** adicionar `ToolOutcomeTruncated` ao enum fechado (`types.go`),
   com `String()="truncated"`, `IsValid()` e caso em `ParseToolOutcome`. No `runtime.go`, consultar
   `result.TruncatedByLength`: quando `true` → `RunStatus=Failed`, `ToolOutcome=ToolOutcomeTruncated`,
   `errStr="resposta truncada por length"`, e o WhatsApp recebe fallback seguro curto (via PostGuard de
   empty/fallback ou texto determinístico), nunca a resposta truncada com valor/categoria/sucesso
   (RF-22/23/47).
2. **Teto de tokens elevado e configurável:** o teto passa a ser lido de variável de ambiente
   (`AGENT_MECONTROLA_MAX_TOKENS`, default **3072**, ~2×), seguindo o padrão de config já usado no
   onboarding (`AGENT_ONBOARDING_LLM_MODEL`); ops ajusta sem novo deploy. `WithDefaultMaxTokens` recebe o
   valor resolvido. Reduz truncamento falso; a falha-segura permanece quando ainda ocorrer (RF-24).
3. **Observabilidade dos gaps de persistência:**
   - `MessageStore.Append` em falha → `agent_message_append_errors_total{agent_id, role}` + log (RF-25).
   - `RunStore.Update` em falha → capturar o erro, log `Error`, `agent_run_update_errors_total{agent_id}`
     e **não** incrementar `agent_runs_total` para o run (não reportar sucesso de estado não persistido —
     RF-26).
   - Erros de múltiplas tools → agregar (bounded, sanitizado) em `errStr` em vez de só o primeiro,
     preservando cardinalidade (RF-27).
4. **Métrica de truncamento:** `agent_run_truncated_total{agent_id}` (RF-23/28).

## Alternativas Consideradas

- **Retry com continuação ao truncar:** rejeitada pelo usuário — mais custo, latência e complexidade de
  estado; anti-alucinação é prioridade sobre completude de resposta longa.
- **Só falha-segura sem elevar teto:** rejeitada — resumos longos legítimos falhariam com frequência.
- **Modelar truncamento como `RunStatus` em vez de `ToolOutcome`:** rejeitada — `RunStatus` já é
  Failed; o `ToolOutcome` é o eixo semântico de causa e já é o tipo fechado consultado por métricas/
  auditoria; adicionar constante é o caminho idiomático (R-AGENT-WF-001.3).
- **Reescrever o runtime:** rejeitada — fora de escopo do PRD; as mudanças são aditivas e locais.

## Consequências

### Benefícios Esperados

- Truncamento deixa de virar sucesso silencioso; causa auditável por `ToolOutcomeTruncated` (RF-23/47).
- Estado inconsistente (Update falho) e perda de mensagem passam a ser observáveis e alertáveis
  (RF-25/26).
- Menos truncamento falso em respostas longas (RF-24).

### Trade-offs e Custos

- Teto de tokens maior aumenta custo/latência marginais em respostas longas (monitorado por
  `agent_llm_tokens_total`/latência).
- Mudança em `internal/platform/agent` (aditiva) exige cuidado de não alterar contrato público.

### Riscos e Mitigações

- **Risco:** não incrementar `agent_runs_total` em Update falho distorcer contagem total. **Mitigação:**
  `agent_run_update_errors_total` cobre o delta; dashboards somam ambos. **Rollback:** reverter para o
  comportamento anterior é trivial (voltar `_ =` e remover consulta a `TruncatedByLength`), embora
  reintroduza os gaps.

## Plano de Implementação

1. `types.go`: adicionar `ToolOutcomeTruncated` (+ String/IsValid/Parse).
2. `runtime.go`: consultar `TruncatedByLength`; observar `Update`/`Append`; agregar erros de tool;
   métricas novas.
3. `mecontrola_agent.go`/wiring: resolver `AGENT_MECONTROLA_MAX_TOKENS` (default 3072) → `WithDefaultMaxTokens`.
4. Testes de runtime cobrindo cada caminho (truncamento, Update falho, Append falho, múltiplos erros).

Concluído quando: truncamento falha-seguro, gaps observáveis, testes verdes, kernel intocado.

## Monitoramento e Validação

- `agent_run_truncated_total`, `agent_run_update_errors_total`, `agent_message_append_errors_total`.
- Gate pós-deploy (ADR-005): sem aumento de truncamento vs baseline (RF-50/53).
- Rever teto de tokens se latência/custo subirem além do aceitável ou se truncamento persistir.

## Impacto em Documentação e Operação

- Runbook: critérios de alerta para os três contadores novos.
- Dashboards: painel de truncamento e de erros de persistência.

## Revisão Futura

Revisitar o valor do teto de tokens após medir a distribuição real de tamanho de resposta em produção.
