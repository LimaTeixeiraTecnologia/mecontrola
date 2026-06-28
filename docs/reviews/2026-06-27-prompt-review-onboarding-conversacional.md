# Prompt pronto para uso

## Instrução

```text
Execute `@.claude/skills/review/` com rigor máximo, sem flexibilização, para revisar a implementação e/ou diff relacionados a `.specs/prd-onboarding-conversacional`.

Objetivo mandatório:
- obter `APPROVED` somente com conformidade total à especificação;
- garantir todos os critérios de aceite atendidos;
- garantir `DoD` 100% atendido;
- garantir `0` gaps;
- garantir `0` lacunas;
- garantir `0` falsos positivos.

Escopo obrigatório de validação:
- `.specs/prd-onboarding-conversacional/prd.md`
- `.specs/prd-onboarding-conversacional/techspec.md`
- `.specs/prd-onboarding-conversacional/tasks.md`
- todas as task files de `.specs/prd-onboarding-conversacional/`
- ADRs e demais documentos decisórios relevantes dentro de `.specs/prd-onboarding-conversacional/`
- relatórios de execução, `bugfix_input.json` e `bugfix_report.md`, quando relevantes para evidência, regressão e completude
- `AGENTS.md`
- `.claude/rules/agent-workflows-tools.md` e `.claude/rules/workflow-kernel.md` quando o escopo tocar `internal/agent` ou `internal/platform/workflow`

Regras obrigatórias:
1. Respeite integralmente a skill `@.claude/skills/review/`, incluindo gates, budgets, critérios de severidade e veredito.
2. Confronte o diff e os arquivos alterados contra especificação funcional, técnica, arquitetural e de governança.
3. Para cada critério de aceite, critério de sucesso, requisito funcional, restrição técnica, regra hard, item de DoD e evidência esperada:
   - marque exatamente um estado entre `atendido`, `não atendido` ou `não verificável`;
   - cite evidência objetiva no código, diff, teste, validação ou artefato;
   - trate `não atendido` como finding bloqueante no mínimo `high`;
   - trate `não verificável` como risco material ou `BLOCKED`, nunca como aprovação implícita.
4. Não aprove por aproximação, interpretação benevolente, intenção, comentário, TODO, stub, evidência indireta ou cobertura parcial.
5. Não reduza severidade para destravar aprovação.
6. Não omita inconsistências entre código, diff, PRD, techspec, tasks, ADRs, relatórios de execução e regras de governança.
7. Não gere findings especulativos. Cada finding deve ter base objetiva e reproduzível para preservar `0` falsos positivos.
8. Se o diff ou os artefatos não permitirem verificar um requisito obrigatório, retorne `BLOCKED`.
9. `APPROVED_WITH_REMARKS` não é aceitável como resultado final deste trabalho.

Checklist mandatório:
- todos os critérios de aceite do PRD atendidos
- todos os critérios de sucesso aplicáveis atendidos
- DoD integralmente atendido
- aderência total a `AGENTS.md`
- aderência total às regras hard do agent, kernel e DMMF, quando aplicáveis
- testes e validações proporcionais ao risco presentes e suficientes
- ausência de regressões funcionais, arquiteturais e de governança
- coerência integral entre implementação final e especificação

Se houver qualquer bug, regressão, lacuna, gap, desvio de especificação, ausência de evidência ou critério não atendido:
- produza findings no formato estruturado esperado pela skill de review;
- converta bugs acionáveis para o formato canônico consumido por `@.claude/skills/bugfix/`;
- execute `@.claude/skills/bugfix/` pela causa raiz;
- exija teste de regressão para cada bug corrigido;
- após a remediação, execute nova revisão focada no delta da correção;
- repita o ciclo `review -> bugfix -> review` quantas vezes forem necessárias;
- só encerre quando o resultado final for `APPROVED`, `REJECTED` ou `BLOCKED`.

Uso de subagentes:
- dispare subagentes especializados sempre que isso aumentar a qualidade da revisão;
- priorize, quando aplicável, subagentes para:
  - conformidade com PRD e critérios de aceite;
  - conformidade arquitetural e governança;
  - cobertura de testes, evidências e regressões;
  - triagem e remediação de bugs.
- consolide o resultado final sem duplicar findings e sem introduzir falsos positivos.

Critério final de saída:
- `APPROVED` somente se todos os critérios aplicáveis estiverem atendidos, com evidência objetiva, sem gaps, sem lacunas e sem falsos positivos;
- `REJECTED` se existir qualquer finding bloqueante remanescente;
- `BLOCKED` se faltar diff, contexto, artefato, evidência ou verificabilidade suficiente.

Formato obrigatório da resposta final:
- `verdict`
- `files_reviewed`
- `spec_artifacts_reviewed`
- `refs_loaded`
- `findings`
- `acceptance_criteria_checklist`
- `dod_checklist`
- `architecture_governance_checklist`
- `validations_run`
- `residual_risks`
- `bugfix_cycles_executed`
- `final_justification`

Restrição desta execução:
- não implemente nada fora do fluxo explicitamente exigido pela skill de review e pelo eventual ciclo de bugfix;
- não faça mudanças cosméticas;
- não declare sucesso parcial como sucesso final.
```

## Variante curta

```text
Execute `@.claude/skills/review/` com rigor máximo contra `.specs/prd-onboarding-conversacional`, confrontando obrigatoriamente `prd.md`, `techspec.md`, `tasks.md`, task files, ADRs relevantes, relatórios de execução relevantes e `AGENTS.md`. Valide todos os critérios de aceite, DoD, regras hard e evidências. Marque cada item como `atendido`, `não atendido` ou `não verificável`, sempre com prova objetiva. Não aceite `APPROVED_WITH_REMARKS` como estado final. Se houver qualquer bug, gap, lacuna, regressão, evidência insuficiente ou desvio da spec, produza findings canônicos, execute `@.claude/skills/bugfix/`, exija teste de regressão e repita `review -> bugfix -> review` até `APPROVED`, `REJECTED` ou `BLOCKED`. Dispare subagentes especializados quando agregarem qualidade à revisão.
```
