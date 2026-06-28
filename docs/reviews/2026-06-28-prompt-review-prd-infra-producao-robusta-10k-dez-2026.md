# Prompt pronto para uso

## Conflito material identificado

O objetivo de "repetir o ciclo até obter `APPROVED`" conflita com o estado documental atual de `.specs/prd-infra-producao-robusta-10k-dez-2026`, porque `tasks.md`, `task-8.0-runbooks-staging.md`, `8.0_execution_report.md` e `_orchestration_report.md` registram a tarefa 8.0 como `blocked` por depender de execução manual em produção e evidências que não podem ser inferidas do código. O prompt enriquecido abaixo preserva a meta de buscar `APPROVED`, mas proíbe aprovação benevolente: se o bloqueio persistir sem evidência nova, o resultado correto deve permanecer `BLOCKED` ou `REJECTED`, nunca um `APPROVED` artificial.

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-infra-producao-robusta-10k-dez-2026
Critérios obrigatórios:
* Todos os critérios de aceite atendidos.
* DoD 100% atendido.
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
Não implemente nada.
```

## Prompt enriquecido

```text
Execute `@.claude/skills/review/` com rigor máximo, sem flexibilização e sem aprovação benevolente, para revisar o diff e/ou working tree relacionados a `.specs/prd-infra-producao-robusta-10k-dez-2026`.

Objetivo mandatório:
- buscar `APPROVED` somente se houver conformidade integral, comprovada e rastreável com a especificação;
- exigir `0` gaps, `0` lacunas e `0` falsos positivos;
- exigir todos os critérios de aceite atendidos e `DoD` 100% atendido;
- se qualquer requisito obrigatório estiver sem evidência suficiente, não presumir conformidade.

Contrato de carga obrigatório antes da revisão:
1. Carregue `AGENTS.md` como fonte canônica.
2. Carregue e siga integralmente `.claude/skills/review/SKILL.md`.
3. Se houver qualquer finding acionável, use `.claude/skills/bugfix/SKILL.md` no ciclo de remediação.
4. Considere o estado atual do working tree e o diff real como fonte de verdade principal quando houver divergência com prompts históricos.

Escopo documental obrigatório:
- `.specs/prd-infra-producao-robusta-10k-dez-2026/prd.md`
- `.specs/prd-infra-producao-robusta-10k-dez-2026/techspec.md`
- `.specs/prd-infra-producao-robusta-10k-dez-2026/tasks.md`
- todas as `task-*.md` da pasta
- todos os `*_execution_report.md`
- `_orchestration_report.md`
- `adr-*.md` relevantes
- `AGENTS.md`
- `.claude/skills/review/SKILL.md`
- `.claude/skills/bugfix/SKILL.md`

Escopo técnico mínimo a confrontar quando houver diff/arquivos alterados nessas áreas:
- `deployment/compose/`
- `deployment/caddy/`
- `deployment/postgres/`
- `deployment/pgbouncer/`
- `deployment/pgbackrest/`
- `deployment/runbooks/`
- `deployment/scripts/`
- `deployment/telemetry/grafana/`
- `cmd/server/`
- `cmd/worker/`
- `cmd/migrate/`
- `internal/platform/http/server/health/`
- `internal/platform/outbox/`
- `.github/workflows/`

Regras obrigatórias da revisão:
1. Respeite integralmente a skill `review`, incluindo budget, severidade, critérios de veredito e formato de saída.
2. Confronte a implementação contra PRD, techspec, tasks, task files, ADRs, execution reports e orquestração; não revise apenas por estilo ou intenção.
3. Para cada RF (`RF-01` a `RF-26`), cada critério de sucesso mensurável (`CS-01` a `CS-06`), cada critério de sucesso de cada task file, cada critério de aceite de cada execution report e cada item de `Definition of Done (DoD)`:
   - marque exatamente um estado entre `atendido`, `não atendido` ou `não verificável`;
   - cite evidência objetiva em código, diff, teste, comando, relatório ou artefato local;
   - trate `não atendido` como finding bloqueante no mínimo `high`;
   - trate `não verificável` como risco material ou `BLOCKED`, nunca como aprovação implícita.
4. Não aprove por aproximação, intenção, comentário, TODO, placeholder, evidência indireta, cobertura parcial ou narrativa de relatório sem lastro verificável.
5. Não gere falso positivo: todo finding precisa ter evidência objetiva, impacto concreto, referência precisa e possibilidade real de reprodução ou inspeção.
6. Não gere falso negativo por omissão: verifique explicitamente cobertura funcional, operacional, arquitetural, segurança, observabilidade, deploy, rollback, backup, restore, idempotência, locking, retry/DLQ e governança.
7. Não reduza severidade para forçar aprovação.
8. Não aceite `APPROVED_WITH_REMARKS` como estado final deste trabalho.

Pontos de atenção obrigatórios neste PRD:
- `tasks.md` e `_orchestration_report.md` registram status final `partial/blocked`.
- `task-8.0-runbooks-staging.md` e `8.0_execution_report.md` registram dependência de execução manual em produção para evidências finais de migração, restore e alertas.
- Portanto, só permita `APPROVED` se existir evidência nova suficiente para eliminar esse bloqueio sem inferência benevolente.
- Se o bloqueio persistir, use `BLOCKED` ou `REJECTED` conforme a skill e documente exatamente o que falta.

Checklist mandatório:
- todos os RFs aplicáveis atendidos com evidência
- todos os critérios de aceite atendidos com evidência
- todos os critérios de sucesso das tasks atendidos com evidência
- todos os itens de DoD atendidos com evidência
- conformidade integral com `AGENTS.md`
- conformidade integral com PRD, techspec, ADRs, tasks e reports
- ausência de regressões funcionais, operacionais, arquiteturais e de governança
- evidências e validações proporcionais ao risco presentes e suficientes
- `0` gaps
- `0` lacunas
- `0` falsos positivos

Fluxo obrigatório de remediação:
1. Execute `@.claude/skills/review/`.
2. Se houver achados reais e acionáveis, produza findings estruturados no formato exigido pela skill.
3. Converta bugs acionáveis para o formato canônico consumido por `@.claude/skills/bugfix/`.
4. Execute `@.claude/skills/bugfix/` somente sobre bugs confirmados, exigindo correção de causa raiz e teste de regressão por bug.
5. Rode nova revisão focada no delta da remediação usando `AI_REVIEW_PRIOR_SHA` ou mecanismo equivalente previsto na skill.
6. Repita `review -> bugfix -> review` até que o estado correto seja `APPROVED`, `REJECTED` ou `BLOCKED`.
7. Nunca declare `APPROVED` enquanto houver qualquer finding remanescente, qualquer item `não verificável` material, qualquer critério obrigatório não atendido ou qualquer evidência pendente de produção.

Uso de subagentes especializados:
- dispare subagentes quando isso aumentar a qualidade da revisão;
- priorize subagentes para:
  - conformidade com PRD/techspec/tasks/reports;
  - governança Go, arquitetura e regras hard de `AGENTS.md`;
  - infraestrutura/deploy Swarm/Caddy/PostgreSQL/pgBouncer/pgBackRest;
  - observabilidade/Grafana/alertas/housekeeping;
  - cobertura de testes, validações e não-regressão;
  - triagem e correção de bugs reais.
- consolide o resultado final sem duplicar findings e sem introduzir falsos positivos.

Formato obrigatório da resposta final:
- `verdict`
- `files_reviewed`
- `spec_artifacts_reviewed`
- `refs_loaded`
- `findings`
- `acceptance_matrix`
- `dod_checklist`
- `task_status_matrix`
- `validations_run`
- `residual_risks`
- `bugfix_cycles_executed`
- `final_decision_rationale`

Regras finais de saída:
- `APPROVED` somente com conformidade total comprovada, sem incerteza material remanescente;
- `REJECTED` se existir qualquer finding bloqueante remanescente;
- `BLOCKED` se faltar diff, artefato, contexto, validação, acesso ou evidência suficiente para provar requisito obrigatório;
- não implemente nada fora do fluxo explicitamente exigido pela skill `review` e pelo eventual ciclo `bugfix`;
- não faça mudanças cosméticas;
- não declare sucesso parcial como sucesso final.
```

## Justificativas do enriquecimento

1. **Conflito explicitado:** o prompt original exige `APPROVED`, mas a documentação atual registra a tarefa 8.0 como `blocked`; tornar isso explícito evita falso positivo por aprovação benevolente.
2. **Escopo fechado e verificável:** a lista de artefatos obrigatórios reduz omissões e força confronto entre PRD, techspec, tasks, execution reports e código real.
3. **Matriz de aceite mensurável:** exigir status por RF, critérios de sucesso, critérios de aceite e DoD reduz subjetividade e melhora rastreabilidade.
4. **Critério anti-falso-positivo:** o prompt agora proíbe aprovação por intenção, narrativa ou evidência indireta.
5. **Ciclo de remediação operacionalizado:** o fluxo `review -> bugfix -> review` foi detalhado para convergir com as skills reais e com revisão focada no delta da correção.
6. **Subagentes especializados direcionados:** foram definidos focos claros para aumentar qualidade sem dispersar escopo.

## Variante curta

```text
Execute `@.claude/skills/review/` com rigor máximo contra `.specs/prd-infra-producao-robusta-10k-dez-2026`, confrontando obrigatoriamente `AGENTS.md`, `prd.md`, `techspec.md`, `tasks.md`, todas as `task-*.md`, `*_execution_report.md`, `_orchestration_report.md` e ADRs relevantes com o diff e o working tree real. Valide item a item `RF-01..RF-26`, `CS-01..CS-06`, critérios de aceite, critérios de sucesso e `DoD`, marcando cada ponto como `atendido`, `não atendido` ou `não verificável`, sempre com evidência objetiva. Não aceite `APPROVED_WITH_REMARKS` como estado final. Se houver qualquer bug, gap, lacuna, regressão, evidência insuficiente ou bloqueio de produção ainda aberto — especialmente o registrado na tarefa 8.0 — produza findings canônicos, execute `@.claude/skills/bugfix/`, exija teste de regressão e repita `review -> bugfix -> review` até o estado correto ser `APPROVED`, `REJECTED` ou `BLOCKED`. Nunca force `APPROVED` sem prova completa e sem falsos positivos.
```
