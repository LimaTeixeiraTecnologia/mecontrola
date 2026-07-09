# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Scorers comportamentais — intrínsecos em produção, oracle-dependente no golden, captura de args e redefinição de tool-call-accuracy
- **Data:** 2026-07-09
- **Status:** Aceita
- **Decisores:** Plataforma / dono do agente MeControla
- **Relacionados:** `prd.md` (RF-29..RF-34, RF-42, RF-49, RF-50), `techspec.md`, US-001

## Contexto

Os 3 scorers atuais (`tool-call-accuracy`, `completeness` por keyword genérica, `categorization`
LLM-judged) têm baseline produtiva (0,304 / 0,149 / 0,565) e rodam em produção com `AlwaysSample`,
persistidos em `platform_scorer_results` de forma assíncrona (`scorer/runner.go`, workers=8). A US pede
9 checks comportamentais. A decisão do usuário: **code-based em produção + reuso no golden**, mantendo os
3 atuais para continuidade de baseline.

Duas restrições técnicas descobertas na investigação:

1. **`ScoringHooks.AfterTool` descarta os args da tool:** a assinatura é
   `AfterTool(ctx, _, toolID string, _ []byte, err error)` e registra
   `scorer.ToolCallRecord{ID: toolID, Name: toolID}` — o `argsJSON` (`_ []byte`) é descartado, então
   `RunSample.ToolCalls[].Args` chega vazio. Scorers que inspecionam argumentos (`required_args`,
   `month_reference_correctness`) precisam desse dado.
2. **Alguns checks precisam de gabarito (oracle):** `expected_tool` só faz sentido com a tool esperada
   por caso, que existe no golden set, não em produção. `tool-call-accuracy` conta 0 em runs
   clarify/chat legítimos (ruído).

## Decisão

1. **Manter os 3 scorers atuais** registrados por continuidade de baseline (RF-29).
2. **Adicionar scorers comportamentais code-based**, separados por dependência de gabarito:
   - **Intrínsecos (produção `AlwaysSample` + reuso no golden):** `no_empty_answer`, `whatsapp_format`,
     `no_internal_terms`, `verbatim_required` (compara `Content` com o `verbatimText` da tool),
     `no_duplicate_write` (write-tools bem-sucedidos não-replay em `ToolCalls`), `no_hallucination`
     (marcador de sucesso sem write-tool bem-sucedido → 0), `required_args` (write-tool sem args de
     domínio obrigatórios → penaliza), `month_reference_correctness` (se tool de mês foi chamada,
     `monthRefKind` presente e consistente; mês nomeado sem ano ⇒ `named_without_year`).
   - **Oracle-dependente (golden apenas):** `expected_tool` (usa `RunSample.Metadata["expected_tool"]`
     do fixture).
3. **Capturar args em `AfterTool`:** preencher `ToolCallRecord.Args` a partir do `argsJSON` (hoje
   descartado), habilitando os scorers de argumento.
4. **Redefinir `tool-call-accuracy` no gate pós-deploy (RF-42):** a métrica do gate é computada por
   **consulta de agregação** sobre `platform_runs`+`platform_scorer_results`, com denominador excluindo
   runs de `outcome ∈ {clarify, replay}` (multi-item, pendência, saudação, idempotência). O scorer
   code-based por-run permanece; a redefinição vive na camada de agregação/runbook, não no scorer.
5. **Promoção/rollback usa ambos** os conjuntos (atuais + comportamentais), RF-31.

## Alternativas Consideradas

- **Só golden set (offline), sem scorers em prod:** rejeitada pelo usuário — deixaria o gate pós-deploy
  cego ao comportamento contínuo.
- **Prod amostrado (sampling < 100%):** rejeitada pelo usuário — mais ruído estatístico no gate; os
  scorers code-based são baratos, `AlwaysSample` é viável.
- **Substituir `completeness` genérico pelo conjunto comportamental:** rejeitada — quebraria a
  continuidade da baseline 0,149 declarada na US.
- **Rodar `expected_tool` em prod com heurística de intenção:** rejeitada — sem gabarito confiável em
  prod geraria falso sinal; melhor mantê-lo no golden.

## Consequências

### Benefícios Esperados

- Sinal comportamental contínuo em produção, persistido por `run_id` sem custo de LLM extra (intrínsecos
  são code-based).
- Baselines preservadas e comparáveis (RF-29/50).
- Gate pós-deploy medindo comportamento real, não presença de palavras (RF-30).

### Trade-offs e Custos

- Pequeno overhead por-run (scorers síncronos no worker async do runner — já tolerante a falha).
- `AfterTool` passa a serializar/armazenar args no context da observação (custo marginal de memória).

### Riscos e Mitigações

- **Risco:** `no_hallucination`/marcadores de sucesso com falso positivo/negativo. **Mitigação:** lista
  conservadora de marcadores + revisão via golden; é sinal de medição, não enforcement (o enforcement é
  o PostGuard de ADR-001).
- **Risco:** `tool-call-accuracy` redefinida ainda ser proxy. **Mitigação:** amostra mínima + margem +
  decisão humana rastreável (ADR-005).

## Plano de Implementação

1. Ajustar `ScoringHooks.AfterTool` para preencher `ToolCallRecord.Args`.
2. Implementar scorers intrínsecos em `internal/agents/application/scorers/` e registrar em
   `BuildMeControlaScorers` (`AlwaysSample`).
3. Implementar `expected_tool` como scorer usado só pelo harness golden (não registrado em prod).
4. Especificar a query de agregação do `tool-call-accuracy` redefinido no runbook (ADR-005).
5. Testes de scorer table-driven (RunSample fixo → ScoreResult).

Concluído quando: scorers em prod persistem, golden usa `expected_tool`, args disponíveis, baseline
mantida.

## Monitoramento e Validação

- `scorer_runs_total{scorer_id, kind, outcome}` e `scorer_duration_seconds` (existentes) cobrem os novos.
- Gate pós-deploy: scorers comportamentais acima da margem; escrita duplicada = 0 (`no_duplicate_write`).

## Impacto em Documentação e Operação

- Runbook/dashboards: painéis dos novos scorers; query do `tool-call-accuracy` redefinido.

## Revisão Futura

Rever o conjunto de marcadores de `no_hallucination` e os args de domínio de `required_args` conforme o
golden evoluir.
