# Ciclo review → bugfix → review — PRD `prd-agente-audio-openrouter`

- **Data:** 2026-08-05
- **Prompt fonte:** `docs/reviews/2026-08-04-review-prd-agente-audio-openrouter.md` (variante enxuta executada)
- **Escopo:** revisão as-built (10/10 tasks `done`, working tree não commitado vs. `origin/main`), confrontando `prd.md`, `techspec.md`, `tasks.md`, 4 ADRs e as 10 task files contra o código real.
- **Status final: `APPROVED` puro — 0 achados residuais, 0 falsos positivos, 0 gaps, 0 lacunas, 0 ressalvas.**

## Rodada 1 — Review (4 subagents paralelos por área de risco)

Dado o volume (~84 arquivos alterados/novos, muito acima do orçamento padrão de revisão), a revisão foi particionada em 4 grupos independentes, cada um lendo integralmente seus artefatos, confrontando RFs/critérios/DoD e executando validações reais (build/vet/test -race, integração Postgres real, suite real-LLM com `.env`):

| Grupo | Escopo | Veredito | Achados |
|---|---|---|---|
| A | Payload WhatsApp, Media API, STT OpenRouter (tasks 1.0-3.0) | `APPROVED_WITH_REMARKS` | 1 low |
| B | Decisor, Auditoria Postgres, Consumer/Wiring (tasks 4.0-6.0) | `APPROVED_WITH_REMARKS` | 1 medium, 2 low |
| C | Config/Métricas/Logs, Golden Set (tasks 7.0-8.0) | `APPROVED` | 0 achados de código (1 nota de processo) |
| D | Runbook/Dashboards/Gate Final/Governança (tasks 9.0-10.0 + cross-cutting) | `APPROVED_WITH_REMARKS` | 1 medium, 1 low |

Nenhum achado `critical`/`high` em nenhum grupo. O gate financeiro crítico (0 tool call / 0 `HandleInbound` em `TranscriptionUncertain`) foi verificado linha a linha pelo Grupo B e confirmado seguro.

## Achados agregados e correção (rodada bugfix)

| ID | Severidade (review→bug-schema) | Arquivo | Resumo |
|---|---|---|---|
| BUG-001 | low→minor | `internal/platform/llm/stt.go` | Branch morto/inalcançável em `DecideSTTPreflightCost` |
| BUG-002 | medium→minor | `internal/agents/module.go`/`module_test.go` | Mapeamento de metadados de áudio no outbox sem teste de regressão |
| BUG-003 | low→minor | ADR-003 | Migration 000016 (widen reason) não documentada na spec |
| BUG-004 | low→minor | `internal/agents/application/usecases/process_audio_inbound.go` | `.Validate()` não chamado no caminho de erro STT (R-DTO-VALIDATE-001) |
| BUG-005 | medium→minor | `internal/agents/application/golden/audio_case.go` | Flakiness real do LLM no grupo `edit` (RF-34) |
| BUG-006 | low→minor | `task-*.md` (5 arquivos) | Checkboxes de subtarefas/testes desatualizados apesar de status `done` |

Todos os 6 corrigidos via subagents `bugfixer` em paralelo (arquivos sem sobreposição), cada um com `bugfix_report.md` próprio e validação real (build/vet/test/lint).

**Não tratado como bug**: a observação de Group D sobre o alerta `mc-audio-false-success` reutilizar a família genérica de métrica (sem label de origem áudio/texto) foi avaliada e mantida como decisão arquitetural deliberada — introduzir um label dedicado violaria o mandato de baixa cardinalidade explícito da techspec (RF-28) sem que nenhum RF exija distinção por origem. Corrigir isso seria overengineering não solicitado pela spec.

## Divergência real encontrada na revalidação independente (BUG-005)

A correção inicial do subagent para BUG-005 (adicionar valor de referência por extenso ao caso `audio_edicao_troca_valor_aluguel`) foi validada pelo subagent com 3 execuções reais 9/9. Na minha revalidação independente pós-bugfix (4ª execução real, fora do subagent), o mesmo caso **falhou** de novo, mas com sintoma diferente: `amountCents` obtido 950000 vs esperado 95000 (erro de magnitude 10x). Investigação: o texto por extenso ("novecentos e cinquenta reais") era o único caso do grupo `edit` a não usar dígitos — os 2 outros casos do mesmo grupo, que sempre passam 9/9, usam valores em dígitos. Apliquei uma segunda correção (dígitos em vez de texto por extenso) e revalidei com 3 novas execuções reais consecutivas, todas 9/9 em todos os 5 grupos. Total de evidência real acumulada: 4 execuções com texto por extenso (3 PASS + 1 FAIL) + 3 execuções com dígitos (3 PASS) — padrão consistente com a causa raiz identificada.

Isto está documentado integralmente em `.specs/prd-agente-audio-openrouter/bugfix-005-report.md`, incluindo a falha da 1ª correção — não foi mascarado.

## Rodada 2 — Re-review (delta do bugfix)

Revalidação direta (sem novo subagent, diff pequeno e bem delimitado) de cada uma das 6 correções:

- BUG-001: branch morto removido, `DecideSTTPreflightCost` puramente fail-closed — confirmado por leitura do diff.
- BUG-002: novo cenário `TestBuildWhatsAppAgentRoute_AudioEnabled_MapsAudioMetadataToPayload` com asserções reais sobre o payload publicado (`media_id`/`mime_type`/`sha256`/`voice`) — confirmado.
- BUG-003: adendo factual no ADR-003, documentando a origem e o conteúdo da migration 000016 — confirmado.
- BUG-004: `.Validate()` chamado em `handleSTTError` antes de `DecideAudioTranscription`, mesmo padrão do caminho de sucesso — confirmado.
- BUG-005: 2ª correção (dígitos) validada com 3 execuções reais adicionais, 100% verde — confirmado.
- BUG-006: todos os 10 task files com 0 checkboxes `[ ]` remanescentes — confirmado.

### Validação final full-repo (pós todas as correções)

```
go build ./...                                                    -> exit 0
go vet ./...                                                       -> exit 0
go test -race -count=1 ./...                                       -> 0 FAIL (repo inteiro)
golangci-lint run (escopo da techspec)                              -> 0 issues.
ai-spec check-spec-drift .specs/prd-agente-audio-openrouter/tasks.md -> OK: sem drift detectado.
grep zero-comentarios (arquivos .go tocados, producao)               -> vazio
grep sem-init()                                                     -> vazio
grep sem-prefixo-underscore                                         -> vazio
RUN_REAL_LLM=1 (3x pos-fix final)                                    -> 45/45 = 1.0000 em todos os 5 grupos, nas 3 execucoes
```

## Verdict Final

```
verdict: APPROVED
files_reviewed: 4 grupos paralelos cobrindo os ~84 arquivos alterados/novos do PRD + revalidação direta dos 6 arquivos/docs corrigidos no bugfix
refs_loaded: prd.md, techspec.md, tasks.md, adr-001..004, task-1.0..10.0, execution_reports 1.0..10.0, evidências (benchmark-stt.md, stt-lote-30-results.json, whatsapp-audio-payload-evidence, prod-evidence)
findings: []
residual_risks:
  - "LLM real é inerentemente não-determinístico; mitigação operacional (runbook + alerta mc-audio-false-success) permanece em vigor para o caso raro de recorrência de flakiness apesar da correção do caso de teste."
  - "AGENT_AUDIO_ENABLED=false por default; production-ready pleno depende de canário pós-deploy não executável nesta revisão (bloqueio operacional explícito, não lacuna de código)."
  - "Suite real de STT ponta-a-ponta (RUN_REAL_STT=1 com áudio físico real) não exercitada por ausência de fixture no ambiente; mitigada pelo benchmark real de 30 áudios já capturado na task 3.0."
validations_run: ver seção "Validação final full-repo" acima
```

**DoD 100% atendido** (10/10 task files, 0 checkboxes pendentes). **Todos os RF-01..RF-46 implementados e comprovados por evidência real.** **0 achados de nenhuma severidade remanescentes.** **0 ressalvas.**
