# Prompt enriquecido para revisao rigorosa da jornada de ativacao via WhatsApp

## Metadados

- **Data:** 2026-06-30
- **Destino:** `docs/reviews/2026-06-30-review-ativacao-whatsapp.md`
- **Idioma:** pt-BR
- **Objetivo:** gerar um prompt operacional, criterioso e sem flexibilizacao para revisar a entrega contra `.specs/prd-ativacao-whatsapp` e, se necessario, acionar `@.claude/skills/bugfix/` em ciclo ate `APPROVED`.

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-ativacao-whatsapp
Critérios obrigatórios:
* Todos os critérios de aceite atendidos.
* DoD 100% atendido.
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
Não implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em docs/reviews/.
```

## Prompt enriquecido

```text
Execute `@.claude/skills/review/` de forma estrita, criteriosa e sem flexibilizacao no repositorio `/Users/jailtonjunior/Git/mecontrola`, usando `AGENTS.md` como fonte canonica e o working tree atual como fonte da verdade tecnica.

Objetivo final obrigatorio:
- obter `APPROVED`;
- com 0 gaps;
- 0 lacunas;
- 0 falsos positivos;
- 100% de aderencia a `.specs/prd-ativacao-whatsapp`;
- 100% dos criterios de aceite e 100% do DoD comprovados por evidencia auditavel.

Contrato de execucao:
1. Carregue obrigatoriamente `AGENTS.md` antes de qualquer analise e confirme o contrato base do repositorio.
2. Execute `@.claude/skills/review/` sem suavizar regras, sem inferir conformidade por proximidade e sem tratar `Status: done` ou execution report como prova suficiente por si so.
3. Valide estritamente contra o conjunto documental local:
   - `.specs/prd-ativacao-whatsapp/prd.md`
   - `.specs/prd-ativacao-whatsapp/techspec.md`
   - `.specs/prd-ativacao-whatsapp/tasks.md`
   - `.specs/prd-ativacao-whatsapp/task-*.md`
   - `.specs/prd-ativacao-whatsapp/*_execution_report.md`
   - `.specs/prd-ativacao-whatsapp/_orchestration_report.md`
   - `.specs/prd-ativacao-whatsapp/adr-*.md`
4. Para wiring, bootstrap e verificacao de integracao real, parta obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`. Nao use `internal/platform/runtime` como ponto de partida.
5. Considere como escopo minimo de conformidade:
   - todos os RFs do PRD;
   - objetivos, criterios de sucesso mensuraveis, UX obrigatoria, restricoes tecnicas e fora de escopo;
   - todas as decisoes materializadas na `techspec.md`;
   - criterios de sucesso de cada `task-*.md`;
   - criterios de aceite e DoD de cada `*_execution_report.md`;
   - consolidacao final em `_orchestration_report.md`.
6. A revisao deve confrontar explicitamente, no minimo, os seguintes checkpoints materiais da especificacao:
   - webhook aprovado marca `PAID` e registra `paidAt`, sem ativar a conta no webhook;
   - ativacao ocorre apenas na primeira mensagem inbound do WhatsApp;
   - CTA do e-mail aponta para `/ativar?token=...`, nunca para `wa.me` com codigo visivel;
   - pagina `/ativar` nao exige login/codigo, nao expoe token e nao oferece Telegram;
   - `wa_me_url` usa exatamente `"Oi"` no fluxo normal;
   - fallback com token no WhatsApp existe apenas no caso de borda sem telefone valido vindo da Kiwify;
   - correlacao por telefone usa normalizacao E.164 compartilhada e escolhe a sessao `PAID` mais recente por `paidAt`;
   - no-match responde ao usuario, gera auditoria/metrica e nunca falha em silencio;
   - integracao de producao fecha a lacuna `dispatcher -> inbound -> evento/consumer/usecase`, sem classificar e descartar a mensagem;
   - caminho legado `ATIVAR <token>` foi removido da jornada e da UX publica;
   - idempotencia cobre reentrega de webhook (`event_id`) e duplicidade de mensagem WhatsApp (`WAMID`);
   - timestamps da jornada, logs estruturados e auditoria existem com cardinalidade controlada;
   - jornada termina nas mensagens de boas-vindas, sem avancar para onboarding financeiro fora de escopo.
7. Se a implementacao divergir da documentacao:
   - use o working tree atual como verdade tecnica;
   - classifique o drift contra a especificacao de forma explicita;
   - nao invente comportamento ausente para fechar lacuna.
8. Como o PRD cita dois repositorios, nao presuma conformidade do repositório externo (`mecontrola-landingpage`) sem evidencia primaria acessivel na rodada. Se o escopo revisado nao trouxer o diff, arquivos ou evidencias necessarias do outro repositorio, registre isso explicitamente como `BLOCKED`, achado ou risco residual — nunca como conforme por inferencia.
9. Execute a revisao contra o diff apropriado:
   - se `AI_REVIEW_PRIOR_SHA` existir, revise apenas o delta da remediacao;
   - caso contrario, use a base apropriada da branch atual, preferencialmente `git diff --merge-base origin/main`.
10. Dispare subagentes especializados quando aumentarem sinal e reduzirem falso positivo, por exemplo:
   - `code-review` para diff relevante, regressao e cobertura de comportamento;
   - `security-review` para webhook, links/token, inbound, idempotencia e superficies de input/output;
   - `research` ou `explore` para confrontar PRD, techspec, tasks, ADRs, reports e wiring real;
   - `rubber-duck` para desafiar conclusoes borderline antes de emitir finding bloqueante.
11. Cada finding deve ter evidencia direta e auditavel:
   - severidade canonica;
   - arquivo e linha quando aplicavel;
   - item exato violado (`RF`, task, criterio de sucesso, criterio de aceite, DoD, ADR, restricao ou metrica);
   - impacto objetivo;
   - `fix_hint` enxuto;
   - reproducao ou lacuna de evidencia quando aplicavel.
12. Proibido emitir finding especulativo. Se a evidencia nao fechar, classifique como risco residual ou `BLOCKED`, nunca como defeito confirmado.
13. Se qualquer problema real for encontrado, gere bugs no formato canonico esperado pela skill e execute `@.claude/skills/bugfix/` para corrigir pela causa raiz, com testes de regressao obrigatorios e rastreabilidade por achado.
14. Apos cada rodada de `bugfix`, rode nova revisao usando o delta da remediacao e repita o ciclo `review -> bugfix -> review` ate que o resultado seja `APPROVED`.
15. Nao encerre com `APPROVED_WITH_REMARKS` ou `REJECTED` como estado final do trabalho. O estado final aceitavel desta solicitacao e apenas `APPROVED`, ou `BLOCKED` se faltar contexto externo incontornavel.

Formato minimo obrigatorio da saida em cada rodada:
- `verdict`
- `files_reviewed`
- `refs_loaded`
- `findings`
- `residual_risks`
- `validations_run`
- `spec_coverage`, cobrindo explicitamente:
  - RF-01 a RF-37
  - criterios de sucesso das tasks 1.0 a 10.0
  - criterios de aceite dos execution reports
  - DoD dos execution reports
  - metricas de sucesso do PRD
  - decisoes arquiteturais/ADRs relevantes
- `next_action`

Gate final obrigatorio antes de declarar `APPROVED`:
- todos os RFs aplicaveis marcados como atendidos com evidencia;
- todos os criterios de sucesso das tasks atendidos com evidencia;
- todos os criterios de aceite atendidos com evidencia;
- DoD 100% atendido com evidencia;
- nenhuma lacuna aberta;
- nenhum falso positivo mantido;
- nenhum risco residual que contradiga a especificacao;
- conformidade total com `.specs/prd-ativacao-whatsapp`.
```

## Justificativas do enriquecimento

1. **Escopo fechado e verificavel:** o prompt passou a enumerar explicitamente `prd.md`, `techspec.md`, `tasks.md`, `task-*.md`, `*_execution_report.md`, `_orchestration_report.md` e ADRs para evitar revisao parcial.
2. **Criterios mensuraveis e rastreaveis:** a cobertura obrigatoria agora exige RFs, criterios de sucesso, criterios de aceite, DoD, metricas de sucesso e checkpoints de UX/arquitetura com evidencia objetiva.
3. **Reducao de falso positivo e falso negativo:** o texto proibe conformidade por inferencia, veta finding especulativo e obriga classificar drift, risco residual ou `BLOCKED` quando a evidencia primaria nao fechar.
4. **Ancoragem no wiring real:** foi reforcado que a revisao precisa partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, essencial para verificar se a lacuna de producao foi de fato fechada.
5. **Tratamento correto do escopo em dois repositorios:** o PRD depende tambem da landing page; o prompt deixa explicito que ausencia de evidencia do repositorio externo nao pode ser mascarada como conformidade.
6. **Ciclo operacional completo:** a regra `review -> bugfix -> review` foi transformada em contrato deterministico, com revisao do delta de remediacao quando `AI_REVIEW_PRIOR_SHA` existir.
7. **Subagentes com criterio:** o prompt nao pede subagentes genericamente; ele orienta quando `code-review`, `security-review`, `research`/`explore` e `rubber-duck` agregam sinal real.

## Variantes

### Variante recomendada

Usar exatamente o prompt enriquecido acima, porque ele combina cobertura documental completa, checkpoints funcionais especificos da jornada, controle de falso positivo e ciclo de remediacao ate `APPROVED`.

### Variante mais conservadora

Manter o mesmo prompt, mas exigir tres passagens explicitas por rodada:
1. matriz documental contra PRD, techspec, tasks, execution reports e ADRs;
2. confrontacao tecnica contra diff, wiring real e evidencias de validacao;
3. checagem final exclusiva de UX/escopo para garantir que Telegram, `ATIVAR <token>` e qualquer onboarding alem das boas-vindas ficaram realmente fora da jornada.

Essa variante aumenta custo e tempo, mas reduz ainda mais o risco de lacunas em fluxos cross-module e em requisitos que misturam backend, UX e observabilidade.
