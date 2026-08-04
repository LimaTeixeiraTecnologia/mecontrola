# Prompt Enriquecido — Review Crítico do PRD `prd-observabilidade-golden-signals-otel`

<!-- gerado por: prompt-enricher | data: 2026-08-04 | nao executar automaticamente -->

## Prompt original (entrada do usuario)

> Execute `@.claude/skills/review/` de forma criteriosa e sem flexibilizacao, validando estritamente contra `.specs/prd-observabilidade-golden-signals-otel`
>
> Criterios obrigatorios:
> * Todos os criterios de aceite atendidos (implementados).
> * DoD 100% atendido (implementados).
> * 0 gaps.
> * 0 lacunas.
> * 0 falsos positivos.
> * 0 ressalvas
> * Todas Regras de negocio atendidos (implementados)
>
> Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e repita o ciclo review -> bugfix -> review ate obter `APPROVED`, sem falsos positivos e em conformidade total com a especificacao.
>
> Dispare subagentes especializados quando agregarem qualidade a revisao.
>
> Nao implemente nada. Apenas crie/enriqueca o prompt e salve o arquivo em `docs/reviews/`.

## Lacunas identificadas no prompt original

- Nao define com precisao o alvo da revisao: diff local, branch vs. `origin/main` ou revisao "as-built" pelos artefatos do PRD.
- Nao explicita quais documentos da especificacao devem ser confrontados obrigatoriamente: `prd.md`, `techspec.md`, `tasks.md`, 4 ADRs e 8 task files.
- Nao transforma "0 ressalvas" em regra operacional; a skill `review` aceita `APPROVED_WITH_REMARKS`, entao o prompt precisa forcar o ciclo ate `APPROVED` puro.
- Nao define como tratar ausencia de evidencias, task incompleta, artifact drift ou task file sem relatorio de execucao.
- Nao orienta onde subagentes agregam qualidade real.

---

## Tarefa (contexto completo para execucao futura)

<context>
Repositorio `mecontrola`, governado por `AGENTS.md` e pelas skills canonicas do repositorio. A revisao deve validar estritamente a entrega contra `.specs/prd-observabilidade-golden-signals-otel/`, sem assumir conformidade com base em relatorios autodeclarados.

Artefatos obrigatorios da especificacao:

- `prd.md`
- `techspec.md`
- `tasks.md`
- `adr-001-register-global-runtime-metrics.md`
- `adr-002-burn-rate-slo-alerts.md`
- `adr-003-alert-metric-reconciliation.md`
- `adr-004-red-canonical-source.md`
- `task-1.0-reconciliacao-alerta-metrica.md`
- `task-2.0-saturacao-runtime.md`
- `task-3.0-worker-heartbeat.md`
- `task-4.0-slo-burn-rate.md`
- `task-5.0-red-fonte-canonica.md`
- `task-6.0-runtime-alert-dashboards.md`
- `task-7.0-email-roteamento-severidade.md`
- `task-8.0-standard-md-validacao.md`

Resumo do PRD a ser validado contra o codigo real:

- RF-01 a RF-03: metricas `go.*` de saturacao de runtime para API e worker, com `RegisterGlobal: true`, dashboard e alerta de causa sem paging isolado.
- RF-04 a RF-06: SLO formal 99,9% + P95 < 500ms e alertas de burn-rate/latencia coexistindo com thresholds existentes.
- RF-07 a RF-08: reconciliacao alerta<->metrica sem serie morta e RED canonico baseado em `http_server_request_duration_seconds_count`.
- RF-09 a RF-12: contact-point de e-mail por severidade, runbooks obrigatorios, heartbeat/liveness do worker, logs com `trace_id`/`span_id`.
- RF-13 a RF-20: cardinalidade estrita, nomes canonicos, preservacao da topologia OTel existente, `STANDARD.md` validado, dashboards atualizados e zero regressao de sinais validos.

As 8 tasks da spec tambem sao obrigatorias e devem ser tratadas como fonte primaria de criterios de aceite e DoD:

1. `1.0 — Reconciliacao alerta<->metrica + gate de CI de auditoria`
2. `2.0 — Saturacao de runtime do processo Go`
3. `3.0 — Heartbeat de liveness do worker + alerta de staleness`
4. `4.0 — SLO + alertas de burn-rate multi-janela + alerta de latencia SLO`
5. `5.0 — Fonte canonica de RED`
6. `6.0 — Alerta de causa de runtime + paineis de runtime e SLO`
7. `7.0 — Contact-point de e-mail + roteamento por severidade + runbooks`
8. `8.0 — STANDARD.md + assets + validacao + cardinalidade/sampling`

O review deve considerar tambem as regras hard de governanca e arquitetura do repositorio quando o diff tocar Go, observabilidade, adapters, workflow ou agent stack.
</context>

<task>
Objetivo: executar `@.claude/skills/review/` sem flexibilizacao, confrontando implementacao e evidencias contra `.specs/prd-observabilidade-golden-signals-otel/`, e disparar `@.claude/skills/bugfix/` sempre que houver qualquer achado ate atingir `APPROVED` puro, com 0 falsos positivos, 0 gaps, 0 lacunas e 0 ressalvas.

1. Nao implementar nada nesta etapa. Este prompt serve para uma execucao futura do ciclo de revisao.
2. Ao executar, determinar o escopo real do review:
   - preferencialmente o diff da feature vs. `origin/main`;
   - se a feature ja estiver consolidada, revisar o estado atual do codigo e dos artefatos como "as-built";
   - se houver rodada de remediacao, usar `AI_REVIEW_PRIOR_SHA` para revisar apenas o delta corrigido.
3. Ler obrigatoriamente `prd.md`, `techspec.md`, `tasks.md`, as 4 ADRs e as 8 task files antes do veredito.
4. Cumprir obrigatoriamente a Etapa 1.5 da skill `review`: confrontar cada item de `## Criterios de Sucesso` e `## Criterios de Aceite` de cada task file contra o codigo, o diff e as evidencias reais, mesmo que o diff nao toque todos os arquivos citados.
5. Confrontar todos os RFs do PRD um a um contra a implementacao real. Nenhum RF pode ser inferido como atendido apenas porque existe task marcada como concluida ou report de execucao.
6. Tratar como bloqueante qualquer um dos seguintes casos:
   - RF nao implementado ou implementado parcialmente;
   - DoD incompleto;
   - alerta ou dashboard referenciando metrica morta;
   - ausencia de `STANDARD.md` valido quando exigido;
   - ausencia de runbook em alerta de pagina;
   - ausencia de cobertura ou evidencia minima para zero regressao;
   - qualquer violacao `[HARD]` de governanca do repositorio.
7. Verificar explicitamente, sem excecao:
   - `RegisterGlobal: true` em server e worker quando a instrumentacao de runtime for adicionada.
   - Uso dos nomes canonicos `go.*` e ausencia de nomes inventados.
   - Preservacao da topologia do Collector, pipelines OTLP e tail sampling.
   - Alertas de burn-rate multi-janela conforme ADR-002.
   - Migracao ou justificativa formal do `mc-api-5xx` para a serie canonica do histograma conforme ADR-004.
   - Eliminacao de referencias a metricas mortas listadas no PRD/ADR-003.
   - Roteamento Telegram para pagina e e-mail para ticket por severidade.
   - Heartbeat/staleness do worker ou evidencia robusta de nao duplicacao.
   - `observability/STANDARD.md` e assets passando no validador especificado.
   - Zero regressao de dashboards, alertas e metricas validas ja existentes.
8. Disparar subagentes especializados quando agregarem qualidade real, por exemplo:
   - um subagente de exploracao para mapear rapidamente os arquivos alterados e a cobertura das 8 tasks;
   - um subagente de revisao adversarial para segunda opiniao sobre achados de alta severidade;
   - um subagente de execucao para rodar validacoes pesadas ou gates de auditoria/documentacao.
9. Se houver qualquer achado em qualquer severidade, gerar bugs no formato canonico aceito pela skill `bugfix` e executar o ciclo `review -> bugfix -> review` ate obter `APPROVED`.
10. Para esta revisao, tratar `APPROVED_WITH_REMARKS` como insuficiente. O unico estado aceitavel para encerramento e `APPROVED`, sem achados residuais de nenhuma severidade.
</task>

<rules>
- Seguir `AGENTS.md` como fonte canonica.
- Aplicar a skill `review` exatamente como definida em `.claude/skills/review/SKILL.md`.
- Aplicar a skill `bugfix` exatamente como definida em `.claude/skills/bugfix/SKILL.md` sempre que houver achado acionavel.
- Nao aceitar evidencia indireta ou declaratoria quando a verificacao puder ser feita no codigo, testes, dashboards provisionados, regras de alerta, scripts e artefatos reais.
- Se houver incerteza, explicitar a incerteza; nao inventar conformidade.
- Nao produzir falsos positivos: cada finding deve apontar arquivo, linha quando aplicavel, impacto e hint de correcao.
- Nao encerrar com ressalvas. Se houver qualquer ressalva, o ciclo continua.
- Nao reclassificar achados reais para severidade menor apenas para permitir aprovacao.
- Nao revisar apenas estilo; priorizar corretude, seguranca, regressao, observabilidade viva e aderencia integral a especificacao.
</rules>

<format>
Saida esperada na execucao futura:

1. Relatorio estruturado da skill `review` com:
   - `verdict`
   - `files_reviewed`
   - `refs_loaded`
   - `findings`
   - `residual_risks`
   - `validations_run`
2. Quando houver achados, lista de bugs no formato canonico para consumo da skill `bugfix`.
3. `bugfix_report.md` por rodada de correcao, salvo no contexto correto da spec.
4. Relatorio final em pt-BR confirmando:
   - `APPROVED`
   - 0 achados residuais
   - 0 falsos positivos
   - 0 gaps
   - 0 lacunas
   - 0 ressalvas
   - DoD 100% atendido
   - todos os RFs e criterios de aceite implementados
</format>

<check>
Antes de encerrar o ciclo, confirmar:

- [ ] Todos os RFs do `prd.md` foram confrontados individualmente contra a implementacao real.
- [ ] Todos os criterios de aceite e de sucesso das 8 task files foram confrontados individualmente.
- [ ] Todas as 4 ADRs foram respeitadas na implementacao final.
- [ ] Nenhum alerta provisionado referencia metrica inexistente.
- [ ] RED usa a fonte canonica exigida ou ha justificativa formal aceita pela especificacao.
- [ ] API e worker expõem os sinais `go.*` corretos quando aplicavel.
- [ ] SLO, burn-rate, latencia, heartbeat, roteamento e runbooks atendem a especificacao integral.
- [ ] `observability/STANDARD.md` e assets existem e passam no validador requerido.
- [ ] Nao houve regressao em metricas, spans, dashboards, alertas ou configuracao valida preexistente.
- [ ] O veredito final da skill `review` e `APPROVED` puro.
</check>

---

## Justificativa das adicoes

| Adicao | Motivo |
|---|---|
| Escopo explicito da revisao | Evita review ambigua ou incompleta. |
| Enumeracao dos artefatos obrigatorios | Garante confronto estrito contra toda a spec, nao apenas o `prd.md`. |
| Regras operacionais para "0 ressalvas" | Impede encerrar com `APPROVED_WITH_REMARKS`. |
| Lista de verificacoes materiais do PRD | Reduz risco de lacunas nos pontos mais sensiveis da entrega. |
| Uso orientado de subagentes | Melhora profundidade da revisao sem perder foco. |
| Checklist final mensuravel | Converte a exigencia do usuario em criterios objetivos de encerramento. |

---

## Variante enxuta

Use esta variante apenas se o executor ja conhecer bem a spec e precisar de um disparo mais curto:

```text
Execute @.claude/skills/review/ validando estritamente contra .specs/prd-observabilidade-golden-signals-otel/ (prd.md, techspec.md, tasks.md, 4 ADRs e 8 task files), confrontando cada RF e cada criterio de aceite/sucesso contra o codigo real, diff e evidencias reais.

Regras obrigatorias:
- 0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas.
- DoD 100% atendido.
- Todos os RFs, ADRs e regras de negocio implementados.
- APPROVED_WITH_REMARKS e insuficiente; o unico estado aceitavel e APPROVED puro.
- Se houver qualquer achado em qualquer severidade, gerar bugs no formato canonico, executar @.claude/skills/bugfix/ e repetir o ciclo review -> bugfix -> review ate APPROVED.
- Dispare subagentes especializados quando agregarem qualidade real.
- Nao assuma conformidade com base em reports; valide no codigo, testes, dashboards, rules.yaml, contact-points e STANDARD.md.
```
