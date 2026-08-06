# Prompt Enriquecido — Review Crítico do PRD `prd-alertas-proativos`

<!-- gerado por: prompt-enricher | data: 2026-08-05 | nao executar automaticamente -->

## Prompt original (entrada do usuario)

> Execute `@.claude/skills/review/` de forma criteriosa e sem flexibilizacao, validando estritamente contra `.specs/prd-alertas-proativos`
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

- Nao define explicitamente o alvo da revisao: diff da feature, branch atual ou estado consolidado "as-built".
- Nao enumera todos os artefatos obrigatorios da spec que precisam ser confrontados antes do veredito.
- Nao transforma "0 ressalvas" em regra operacional; a skill `review` aceita `APPROVED_WITH_REMARKS`, entao o prompt precisa forcar `APPROVED` puro.
- Nao explicita o confronto individual de RFs, REQs, criterios de sucesso, criterios de aceite, DoD e ADRs contra codigo e evidencias reais.
- Nao deixa claro que a revisao nao pode confiar em claims declaratorias, status de task, markdowns de evidencia ou documentacao sem confirmacao material no codebase.

---

## Tarefa (contexto completo para execucao futura)

<context>
Repositorio `mecontrola`, governado por `AGENTS.md`, com PRD alvo em `.specs/prd-alertas-proativos/`. A revisao deve ser estrita e sem flexibilizacao, validando implementacao e evidencias contra a especificacao real, nunca por inferencia, nunca por status declarado em markdown e nunca por presuncao de conformidade.

Artefatos obrigatorios da especificacao:

- `.specs/prd-alertas-proativos/prd.md`
- `.specs/prd-alertas-proativos/techspec.md`
- `.specs/prd-alertas-proativos/tasks.md`
- `.specs/prd-alertas-proativos/adr-001-dry-run-antes-de-envio-real.md`
- `.specs/prd-alertas-proativos/adr-002-nao-aplicar-pattern-formal.md`
- `.specs/prd-alertas-proativos/adr-003-threshold-90-condicionado.md`
- `.specs/prd-alertas-proativos/task-1.0-configuracao-e-dry-run-seguro-de-thresholds.md`
- `.specs/prd-alertas-proativos/task-2.0-politica-de-dominio-release-1.md`
- `.specs/prd-alertas-proativos/task-3.0-template-generico-whatsapp-meta-sem-regressao.md`
- `.specs/prd-alertas-proativos/task-4.0-notifier-por-template-aprovado-e-sucesso-real.md`
- `.specs/prd-alertas-proativos/task-5.0-follow-up-agentivo-com-contexto-de-alerta.md`
- `.specs/prd-alertas-proativos/task-6.0-observabilidade-e-auditoria-operacional.md`
- `.specs/prd-alertas-proativos/task-7.0-gates-de-rollout-e-validacao-meta.md`
- `.specs/prd-alertas-proativos/task-8.0-validacao-integrada-e-documentacao-final.md`
- `docs/refin/2026-08-05-sdd-alertas-proativos.md`
- `docs/refin/2026-08-05-meta-templates-alertas-proativos.md`, quando existir e for citado por tasks ou evidencias

Resumo minimo do escopo funcional que precisa ser provado no codigo real:

- Release 1 contem apenas alertas de categoria 80%, categoria 100%, orcamento ausente no inicio do mes e orcamento nao revisado ate o terceiro dia.
- Threshold 90 permanece bloqueado ate alteracao explicita de dominio, constraint e migration PostgreSQL.
- Dry-run e obrigatorio antes de envio real e nao pode publicar outbox, marcar envio, chamar Meta ou alterar estado externo.
- Envio proativo fora da janela WhatsApp so ocorre com template Meta `APPROVED`; `PENDING`, `REJECTED`, ausente ou nao configurado bloqueiam o envio daquele alerta.
- Texto livre nao pode ser fallback do envio proativo fora da janela.
- Quiet hours obrigatorias entre 20:00 e 08:00 no timezone do usuario, com fallback `America/Sao_Paulo`.
- Templates `MARKETING` exigem opt-in explicito.
- No maximo um alerta iniciado pelo sistema por usuario por rodada diaria.
- Deduplicacao obrigatoria por usuario, tipo de alerta, alvo e periodo, sem colapsar indevidamente alertas distintos.
- Follow-up agentivo deve usar contexto recente valido do alerta; se o contexto expirar, o sistema deve pedir esclarecimento.
- Fluxos atuais de onboarding, WhatsApp inbound, budgets threshold, outreach de ativacao e agente financeiro nao podem sofrer regressao.
- Observabilidade deve expor avaliacao, supressao, fila, envio e falha com metricas e logs de baixa cardinalidade.

Tarefas obrigatorias da spec a confrontar individualmente:

1. `1.0 — Configuracao e dry-run seguro de thresholds`
2. `2.0 — Politica de dominio Release 1`
3. `3.0 — Template generico WhatsApp/Meta sem regressao`
4. `4.0 — Notifier por template aprovado e sucesso real`
5. `5.0 — Follow-up agentivo com contexto de alerta`
6. `6.0 — Observabilidade e auditoria operacional`
7. `7.0 — Gates de rollout e validacao Meta`
8. `8.0 — Validacao integrada e documentacao final`

A revisao deve tratar `prd.md`, `techspec.md`, `tasks.md`, ADRs, task files, SDD e evidencias referenciadas como conjunto obrigatorio. Se houver drift entre esses artefatos e o codigo real, o codigo real prevalece como fato observado, e o drift deve virar achado.
</context>

<task>
Objetivo: executar `@.claude/skills/review/` sem flexibilizacao, confrontando implementacao e evidencias contra `.specs/prd-alertas-proativos/`, e acionar `@.claude/skills/bugfix/` sempre que houver qualquer problema ate atingir `APPROVED` puro, com 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas.

1. Nao implementar nada nesta etapa. Este prompt serve para uma execucao futura do ciclo de review.
2. Determinar o escopo real da revisao antes de emitir veredito:
   - preferencialmente revisar o diff da feature contra `origin/main`;
   - se a entrega ja estiver consolidada, revisar o estado atual do codigo e artefatos como "as-built";
   - em rodada pos-remediacao, usar `AI_REVIEW_PRIOR_SHA` para revisar apenas o delta corrigido, sem perder o confronto material contra a spec.
3. Ler obrigatoriamente todos os artefatos listados em `<context>` antes do veredito.
4. Cumprir obrigatoriamente a Etapa 1.5 da skill `review`: confrontar cada item de `## Criterios de Sucesso` e `## Criterios de Aceite` de cada task file contra codigo, diff e evidencias reais, mesmo que o diff nao toque todos os arquivos citados.
5. Confrontar todos os RFs do `prd.md` individualmente contra a implementacao real.
6. Confrontar todos os REQs do `techspec.md` individualmente contra a implementacao real.
7. Confrontar cada DoD, teste da tarefa, subtarefa critica, risco controlado e gate operacional definidos nas task files e ADRs.
8. Nao aceitar como prova suficiente:
   - task marcada como `done`;
   - checklist preenchido em markdown;
   - comentario no codigo sem verificacao material;
   - evidencia declaratoria sem teste, fixture, config, log, metrica, wiring ou comportamento observavel correspondente.
9. Tratar como bloqueante qualquer um dos seguintes casos:
   - RF, REQ, criterio de aceite, criterio de sucesso ou DoD nao implementado ou implementado parcialmente;
   - existencia de gap, lacuna, ressalva ou risco residual nao resolvido;
   - qualquer falso positivo no review;
   - qualquer violacao `[HARD]` de governanca do repositorio;
   - dry-run com side effects externos;
   - envio real sem template `APPROVED`;
   - fallback para texto livre onde a spec o proibe;
   - uso indevido do threshold 90 no Release 1;
   - ausencia de quiet hours, timezone fallback ou opt-in quando exigidos;
   - deduplicacao incorreta, priorizacao errada ou mais de um alerta por usuario/rodada;
   - follow-up inferindo intencao sem contexto recente valido;
   - regressao nos fluxos atuais de onboarding, WhatsApp inbound, budgets thresholds, outreach ou agente financeiro;
   - metricas/logs sem baixa cardinalidade ou com exposicao de dados sensiveis/segredos.
10. Verificar explicitamente, sem excecao:
   - apenas os 4 cenarios do Release 1 estao ativos;
   - threshold 90 permanece desabilitado por design e nao entra como emissao real;
   - dry-run nao publica outbox, nao grava dedup de envio, nao chama Meta e nao marca alerta como notificado;
   - envio real por template aprovado marca sucesso apenas apos aceite real do gateway;
   - falha Meta nao marca sucesso;
   - `ChannelGateway` e o contrato de template generico permanecem compativeis com os fluxos existentes;
   - `SendText` e `SendActivationTemplate` seguem sem regressao;
   - quiet hours 20:00-08:00 no timezone do usuario, com fallback `America/Sao_Paulo`, estao implementadas;
   - templates `MARKETING` exigem opt-in explicito;
   - existe no maximo um alerta iniciado pelo sistema por usuario por rodada diaria;
   - supressoes por prioridade, frequencia, canal ausente, opt-in ausente, quiet hours e template ausente/nao aprovado sao registradas;
   - dedup impede repeticao por usuario, kind, alvo e periodo, sem colapsar 80 e 100 indevidamente;
   - follow-up agentivo usa contexto recente valido e pede esclarecimento quando expirado;
   - nenhuma alteracao recriou primitivos agentivos, workflow paralelo ou regra financeira duplicada fora do stack existente;
   - metricas e logs exigidos pela spec existem com labels de baixa cardinalidade e sem segredos Meta ou payload sensivel;
   - runbook e documentacao final refletem o estado real de dry-run e aprovacao Meta por kind;
   - `ai-spec check-spec-drift .specs/prd-alertas-proativos` e os gates proporcionais da tarefa 8.0 possuem evidencias coerentes com o estado atual.
11. Disparar subagentes especializados quando agregarem qualidade real, por exemplo:
   - subagente de exploracao para mapear cobertura de RFs, REQs, ADRs e task files no diff e no codebase;
   - subagente de revisao adversarial para segunda opiniao em achados `high`/`critical`;
   - subagente de execucao para rodar suites pesadas, gates integrados ou validadores de evidencia.
12. Se houver qualquer achado em qualquer severidade, gerar bugs no formato canonico exigido pela skill `bugfix` e executar o ciclo `review -> bugfix -> review` ate obter `APPROVED`.
13. Para esta revisao, tratar `APPROVED_WITH_REMARKS` como insuficiente. O unico estado aceitavel para encerramento e `APPROVED`, sem achados residuais de nenhuma severidade.
</task>

<rules>
- Seguir `AGENTS.md` como fonte canonica.
- Aplicar a skill `review` exatamente como definida em `.claude/skills/review/SKILL.md`.
- Aplicar a skill `bugfix` exatamente como definida em `.claude/skills/bugfix/SKILL.md` sempre que houver achado acionavel.
- Usar a skill `go-implementation` e as demais skills obrigatorias quando a analise tocar codigo Go, dominio, patterns, agent stack ou persistencia, conforme os gatilhos do repositorio.
- Nao aceitar evidencia indireta quando houver como confirmar no codigo, testes, configuracoes, wiring, fixtures, logs, metricas, dashboards, scripts ou validadores reais.
- Se houver incerteza, explicitar a incerteza; nao inventar conformidade.
- Nao produzir falsos positivos: cada finding deve apontar arquivo, linha quando aplicavel, impacto e hint de correcao.
- Nao encerrar com ressalvas. Se houver qualquer ressalva, o ciclo continua.
- Nao rebaixar severidade para permitir aprovacao artificial.
- Priorizar corretude, seguranca, regressao, observabilidade, privacidade, deduplicacao, gates operacionais e aderencia integral a PRD, techspec, ADRs e tasks.
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
   - todos os RFs, REQs, regras de negocio, criterios de aceite e criterios de sucesso implementados
</format>

<check>
Antes de encerrar o ciclo, confirmar:

- [ ] Todos os RFs do `prd.md` foram confrontados individualmente contra a implementacao real.
- [ ] Todos os REQs do `techspec.md` foram confrontados individualmente contra a implementacao real.
- [ ] Todos os criterios de aceite e de sucesso das 8 task files foram confrontados individualmente.
- [ ] Todos os DoDs, testes da tarefa e gates operacionais das 8 task files foram confrontados individualmente.
- [ ] As 3 ADRs da spec foram respeitadas na implementacao final.
- [ ] Apenas os 4 alertas do Release 1 estao ativos.
- [ ] O threshold 90 continua bloqueado ate migration/modelagem futura.
- [ ] Dry-run nao executa side effects externos.
- [ ] Envio real so ocorre com template Meta `APPROVED`.
- [ ] Nao existe fallback por texto livre onde o PRD o proibe.
- [ ] Quiet hours, timezone fallback e opt-in `MARKETING` estao corretos.
- [ ] Nenhum fluxo existente de onboarding, WhatsApp inbound, budgets, outreach ou agente financeiro sofreu regressao funcional.
- [ ] Supressao, prioridade, dedup e follow-up foram comprovados com evidencia valida.
- [ ] Logs, metricas e auditoria respeitam privacidade e baixa cardinalidade.
- [ ] Runbook e documentacao final refletem o estado real de Meta e dry-run.
- [ ] O veredito final da skill `review` e `APPROVED` puro.
</check>

---

## Justificativa das adicoes

| Adicao | Motivo |
|---|---|
| Escopo explicito da revisao | Evita review baseada no alvo errado. |
| Enumeracao completa dos artefatos obrigatorios | Garante confronto estrito contra toda a spec, nao apenas `prd.md`. |
| Regra operacional para `APPROVED` puro | Impede encerrar com `APPROVED_WITH_REMARKS`. |
| Confronto individual de RF, REQ, ADR, criterios e DoD | Traduz o pedido do usuario para um protocolo verificavel. |
| Regras de evidencia material | Reduz risco de aprovacao por declaracao ou markdown. |
| Verificacoes explicitas dos pontos sensiveis do Release 1 | Cobre os riscos centrais de dry-run, Meta, dedup, follow-up e regressao. |
| Uso orientado de subagentes | Aumenta profundidade sem perder foco. |

---

## Variante enxuta

Use esta variante apenas se o executor ja conhecer bem a spec e precisar de um disparo mais curto:

```text
Execute @.claude/skills/review/ validando estritamente contra .specs/prd-alertas-proativos/ (prd.md, techspec.md, tasks.md, 3 ADRs, 8 task files e docs/refin relevantes), confrontando cada RF, cada REQ, cada regra de negocio, cada criterio de aceite/sucesso e cada DoD contra codigo real, diff e evidencias reais.

Regras obrigatorias:
- 0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas.
- DoD 100% atendido.
- Todos os RFs, REQs, ADRs e regras de negocio implementados.
- APPROVED_WITH_REMARKS e insuficiente; o unico estado aceitavel e APPROVED puro.
- Se houver qualquer achado em qualquer severidade, gerar bugs no formato canonico, executar @.claude/skills/bugfix/ e repetir o ciclo review -> bugfix -> review ate APPROVED.
- Dispare subagentes especializados quando agregarem qualidade real.
- Nao assuma conformidade com base em reports; valide no codigo, testes, configs, wiring, logs, metricas, runbooks e evidencias reais.
```
