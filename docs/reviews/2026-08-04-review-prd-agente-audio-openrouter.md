# Prompt Enriquecido — Review Crítico do PRD `prd-agente-audio-openrouter`

<!-- gerado por: prompt-enricher | data: 2026-08-04 | nao executar automaticamente -->

## Prompt original (entrada do usuario)

> Execute `@.claude/skills/review/` de forma criteriosa e sem flexibilizacao, validando estritamente contra `.specs/prd-agente-audio-openrouter`
>
> Criterios obrigatorios:
> * Todos os criterios de aceite atendidos (implementados).
> * DoD 100% atendido (implementados).
> * 0 gaps.
> * 0 lacunas.
> * 0 falsos positivos.
> * 0 ressalvas
> * Todas regras de negocio atendidas (implementadas)
>
> Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e repita o ciclo `review -> bugfix -> review` ate obter `APPROVED`, sem falsos positivos e em conformidade total com a especificacao.
>
> Dispare subagentes especializados quando agregarem qualidade a revisao.
>
> Nao implemente nada. Apenas crie/enriqueca o prompt e salve o arquivo em `docs/reviews/`.

## Lacunas identificadas no prompt original

- Nao define com precisao se o alvo da revisao e diff local, branch vs. `origin/main` ou revisao "as-built".
- Nao explicita todos os artefatos que devem ser confrontados obrigatoriamente dentro de `.specs/prd-agente-audio-openrouter/`.
- Nao transforma "0 ressalvas" em regra operacional; a skill `review` aceita `APPROVED_WITH_REMARKS`, entao o prompt precisa forcar `APPROVED` puro.
- Nao define como tratar evidencias autodeclaradas, status `done` em task files, relatórios de execucao e drift entre spec e codigo.
- Nao explicita que todo RF, criterio de aceite, criterio de sucesso e DoD das tasks deve ser confrontado individualmente contra codigo e evidencias reais.

---

## Tarefa (contexto completo para execucao futura)

<context>
Repositorio `mecontrola`, governado por `AGENTS.md`, com regras canonicas de arquitetura, governanca e revisao. A validacao deve ser estrita contra `.specs/prd-agente-audio-openrouter/`, sem assumir conformidade com base em reports autodeclarados, task status ou claims em markdown.

Artefatos obrigatorios da especificacao:

- `prd.md`
- `techspec.md`
- `tasks.md`
- `adr-001-stt-dedicado-openrouter.md`
- `adr-002-outcome-terminal-wamid-audio.md`
- `adr-003-auditoria-sem-audio-bruto.md`
- `adr-004-nao-aplicar-pattern-formal.md`
- `task-1.0-payload-whatsapp-tipado.md`
- `task-2.0-meta-media-api-duracao.md`
- `task-3.0-stt-openrouter-custo.md`
- `task-4.0-decisor-audio.md`
- `task-5.0-auditoria-postgres-wamid.md`
- `task-6.0-integracao-consumer-wiring.md`
- `task-7.0-config-metricas-logs.md`
- `task-8.0-golden-real-stt.md`
- `task-9.0-runbook-dashboard-readiness.md`
- `task-10.0-gate-final-production-ready.md`
- artefatos de evidencia relevantes da mesma pasta, quando citados por PRD, techspec, ADRs ou tasks

Resumo minimo do PRD a validar contra o codigo real:

- RF-01..RF-05: discovery obrigatorio do payload real de audio WhatsApp, com campos confirmados e rejeicao segura de payload invalido.
- RF-06..RF-12: download de midia com timeout, limite de tamanho, limite de duracao, STT via OpenRouter, timeout configuravel e budget de latencia.
- RF-13..RF-20: gates de `TranscriptionUncertain`, zero tool call financeira em incerteza tecnica, resposta segura, entrada textual canonica e reaproveitamento do fluxo textual existente.
- RF-21..RF-27: idempotencia por WAMID, outcome terminal unico, descarte do audio original, auditoria sem audio bruto e logs sem dados sensiveis indevidos.
- RF-28..RF-29: metricas, thresholds, janelas e readiness operacional de audio.
- RF-30..RF-36: golden set texto/audio, score >= 0,90 por grupo, 0 falso-sucesso, 0 tool call em `TranscriptionUncertain` e execucao real por flag/credencial.
- RF-37..RF-39: uso obrigatorio das skills `domain-modeling-production`, `design-patterns-mandatory`, `mastra` e `go-implementation`, sem agente paralelo, sem workflow especifico de audio e sem provider STT fora do OpenRouter.
- RF-40..RF-46: preservacao dos gates atuais de inbound, evidencia do payload real, proibicao de correcao semantica heuristica, manutencao de confirmacoes sensiveis, modo de teste, fail-closed do STT e runbook operacional.

As 10 tasks sao obrigatorias e devem ser tratadas como fonte primaria de criterios de aceite, criterios de sucesso, DoD e rastreabilidade por RF:

1. `1.0 — Payload WhatsApp tipado e regressao textual`
2. `2.0 — Cliente Meta Media API e duracao deterministica`
3. `3.0 — Porta STT OpenRouter com custo pre e pos-STT`
4. `4.0 — Decisor tecnico fechado de audio`
5. `5.0 — Auditoria Postgres e WAMID terminal`
6. `6.0 — Integracao consumer, outbox e wiring Mastra`
7. `7.0 — Configuracao, metricas e logs de audio`
8. `8.0 — Golden set audio/texto e suites reais por flag`
9. `9.0 — Runbook, dashboards e readiness operacional`
10. `10.0 — Gate final production-ready RF-01..RF-46`

O review deve considerar tambem as regras hard de governanca do repositorio quando tocar codigo Go, adapters, observabilidade, agent stack, outbox, workflow, persistencia ou modelagem de dominio.
</context>

<task>
Objetivo: executar `@.claude/skills/review/` sem flexibilizacao, confrontando implementacao e evidencias contra `.specs/prd-agente-audio-openrouter/`, e acionar `@.claude/skills/bugfix/` sempre que houver qualquer problema ate atingir `APPROVED` puro, com 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas.

1. Nao implementar nada nesta etapa. Este prompt serve para uma execucao futura do ciclo de review.
2. Ao executar, determinar o escopo real da revisao:
   - preferencialmente o diff da feature vs. `origin/main`;
   - se a feature ja estiver consolidada, revisar o estado atual do codigo e dos artefatos como "as-built";
   - em rodada pos-remediacao, usar `AI_REVIEW_PRIOR_SHA` para revisar apenas o delta corrigido.
3. Ler obrigatoriamente `prd.md`, `techspec.md`, `tasks.md`, as 4 ADRs e as 10 task files antes do veredito.
4. Cumprir obrigatoriamente a Etapa 1.5 da skill `review`: confrontar cada item de `## Criterios de Sucesso` e `## Criterios de Aceite` de cada task file contra codigo, diff e evidencias reais, mesmo que o diff nao toque todos os arquivos citados.
5. Confrontar todos os RFs do PRD um a um contra a implementacao real. Nenhum RF pode ser dado como atendido apenas porque existe task `done`, report de execucao, benchmark ou evidence markdown.
6. Confrontar tambem, de forma individual, cada DoD declarado nas task files, cada ADR material e cada gate de nao-falso-positivo do PRD.
7. Tratar como bloqueante qualquer um dos seguintes casos:
   - RF nao implementado ou implementado parcialmente;
   - DoD incompleto em qualquer task;
   - criterio de aceite ou de sucesso nao comprovado;
   - ausencia de payload real mascarado quando exigido;
   - audio bruto persistido ou exposto indevidamente;
   - qualquer tool call financeira, `HandleInbound` ou mutacao quando a classificacao deveria ser `TranscriptionUncertain`;
   - qualquer violacao `[HARD]` de governanca do repositorio;
   - ausencia de evidencias minimas para comprovar golden, runbook, metricas, logs, thresholds, ou fail-closed.
8. Verificar explicitamente, sem excecao:
   - fluxo textual preexistente segue sem regressao funcional;
   - discovery do payload real de audio WhatsApp foi feito com campos obrigatorios e exemplos sanitizados;
   - download da midia usa timeout, autenticacao apropriada e limite configuravel;
   - rejeicao por duracao e tamanho ocorre de forma deterministica quando aplicavel;
   - STT usa OpenRouter como provider unico, sem fallback chain;
   - timeout STT inicial de 20 segundos e p95 alvo de ate 8 segundos estao refletidos na implementacao/evidencias requeridas;
   - `TranscriptionUncertain` e modelado como estado fechado e bloqueia dispatch financeiro;
   - a entrada textual canonica preserva o texto transcrito sem enriquecimento semantico indevido;
   - WAMID tem outcome terminal unico e nao reabre processamento automaticamente;
   - audio original e descartado apos transcricao ou rejeicao;
   - logs informativos nao carregam audio bruto nem transcricao completa sensivel;
   - metricas possuem baixa cardinalidade e cobrem aceite, rejeicao, falha STT, incerteza, dispatch, latencia, tamanho, duracao e custo quando aplicavel;
   - golden set cobre positivos e negativos conforme PRD e tasks;
   - score >= 0,90 por grupo de intencao e 0 falso-sucesso em mutacoes;
   - 0 tool call e 0 chamada a `HandleInbound` em `TranscriptionUncertain`;
   - modo real por flag/credencial existe sem substituir testes unitarios;
   - nao foi criado agente paralelo, workflow especifico de audio ou regra financeira duplicada;
   - runbook operacional final cobre falha de download, falha STT, incerteza alta, latencia alta, custo alto e regressao do golden.
9. Disparar subagentes especializados quando agregarem qualidade real, por exemplo:
   - um subagente de exploracao para mapear rapidamente cobertura das 10 tasks e RFs no diff;
   - um subagente de revisao adversarial para segunda opiniao em achados `high`/`critical`;
   - um subagente de execucao para rodar suites pesadas, golden real por flag ou validadores de evidencias.
10. Se houver qualquer achado em qualquer severidade, gerar bugs no formato canonico exigido pela skill `bugfix` e executar o ciclo `review -> bugfix -> review` ate obter `APPROVED`.
11. Para esta revisao, tratar `APPROVED_WITH_REMARKS` como insuficiente. O unico estado aceitavel para encerramento e `APPROVED`, sem achados residuais de nenhuma severidade.
</task>

<rules>
- Seguir `AGENTS.md` como fonte canonica.
- Aplicar a skill `review` exatamente como definida em `.claude/skills/review/SKILL.md`.
- Aplicar a skill `bugfix` exatamente como definida em `.claude/skills/bugfix/SKILL.md` sempre que houver achado acionavel.
- Nao aceitar evidencia indireta, declaratoria ou apenas documental quando a verificacao puder ser feita no codigo, testes, fixtures, dashboards, logs, configuracoes, scripts ou artefatos reais.
- Se houver incerteza, explicitar a incerteza; nao inventar conformidade.
- Nao produzir falsos positivos: cada finding deve apontar arquivo, linha quando aplicavel, impacto e hint de correcao.
- Nao encerrar com ressalvas. Se houver qualquer ressalva, o ciclo continua.
- Nao rebaixar severidade para permitir aprovacao artificial.
- Nao revisar apenas estilo; priorizar corretude, seguranca, regressao, observabilidade, privacidade, idempotencia e aderencia integral a PRD, ADRs, techspec e tasks.
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
   - todos os RFs, regras de negocio, criterios de aceite e criterios de sucesso implementados
</format>

<check>
Antes de encerrar o ciclo, confirmar:

- [ ] Todos os RFs do `prd.md` foram confrontados individualmente contra a implementacao real.
- [ ] Todos os criterios de aceite e de sucesso das 10 task files foram confrontados individualmente.
- [ ] Todos os DoDs das 10 task files foram confrontados individualmente.
- [ ] Todas as 4 ADRs foram respeitadas na implementacao final.
- [ ] O discovery com payload real de audio WhatsApp foi validado com evidencia mascarada.
- [ ] Nenhum fluxo de texto preexistente sofreu regressao funcional.
- [ ] Nenhum audio incerto dispara tool financeira nem `HandleInbound`.
- [ ] O provider STT permanece exclusivamente OpenRouter.
- [ ] Nao existe agente paralelo, workflow especifico de audio ou duplicacao de regra financeira.
- [ ] Audio bruto nao permanece persistido apos processamento.
- [ ] Logs, metricas e auditoria respeitam privacidade e baixa cardinalidade.
- [ ] Golden set, casos negativos, score >= 0,90 e 0 falso-sucesso foram comprovados por evidencia valida.
- [ ] Runbook operacional final existe e cobre os cenarios exigidos pelo RF-46.
- [ ] O veredito final da skill `review` e `APPROVED` puro.
</check>

---

## Justificativa das adicoes

| Adicao | Motivo |
|---|---|
| Escopo explicito da revisao | Evita review ambigua ou baseada em alvo incorreto. |
| Enumeracao completa dos artefatos obrigatorios | Garante confronto estrito contra toda a spec, nao apenas `prd.md`. |
| Regras operacionais para "0 ressalvas" | Impede encerrar com `APPROVED_WITH_REMARKS`. |
| Verificacoes materiais derivadas de RF-01..RF-46 | Reduz risco de lacunas nos pontos sensiveis da entrega. |
| Confronto individual de RF, DoD, criterios e ADRs | Traduz a exigencia do usuario para um protocolo verificavel. |
| Uso orientado de subagentes | Melhora profundidade da revisao sem perder foco nem gerar ruido. |

---

## Variante enxuta

Use esta variante apenas se o executor ja conhecer bem a spec e precisar de um disparo mais curto:

```text
Execute @.claude/skills/review/ validando estritamente contra .specs/prd-agente-audio-openrouter/ (prd.md, techspec.md, tasks.md, 4 ADRs e 10 task files), confrontando cada RF, cada regra de negocio, cada criterio de aceite/sucesso e cada DoD contra o codigo real, diff e evidencias reais.

Regras obrigatorias:
- 0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas.
- DoD 100% atendido.
- Todos os RFs, ADRs e regras de negocio implementados.
- APPROVED_WITH_REMARKS e insuficiente; o unico estado aceitavel e APPROVED puro.
- Se houver qualquer achado em qualquer severidade, gerar bugs no formato canonico, executar @.claude/skills/bugfix/ e repetir o ciclo review -> bugfix -> review ate APPROVED.
- Dispare subagentes especializados quando agregarem qualidade real.
- Nao assuma conformidade com base em reports; valide no codigo, testes, fixtures, evidencias mascaradas, logs, metricas, runbooks e configs reais.
```
