# Prompt enriquecido para revisao rigorosa da plataforma Mastra

## Metadados

- **Data:** 2026-06-29
- **Destino:** `docs/reviews/2026-06-29-review-platform-mastra.md`
- **Idioma:** pt-BR
- **Objetivo:** gerar um prompt operacional, criterioso e sem flexibilizacao para revisar a entrega contra `.specs/prd-platform-mastra` e, se necessario, acionar `@.claude/skills/bugfix/` em ciclo ate `APPROVED`.

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilizacao, validando estritamente contra .specs/prd-platform-mastra
Critérios obrigatórios:
* Todos os critérios de aceite atendidos.
* DoD 100% atendido.
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
```

## Prompt enriquecido

```text
Execute `@.claude/skills/review/` de forma estrita, criteriosa e sem flexibilizacao no repositorio `/Users/jailtonjunior/Git/mecontrola`, usando `AGENTS.md` como fonte canonica e o working tree atual como fonte da verdade.

Objetivo final obrigatorio:
- obter `APPROVED`;
- com 0 gaps;
- 0 lacunas;
- 0 falsos positivos;
- 100% de aderencia a `.specs/prd-platform-mastra`;
- 100% dos criterios de aceite e 100% do DoD comprovados por evidencia.

Contrato de execucao:
1. Carregue obrigatoriamente `AGENTS.md` e siga o contrato base do repositorio antes de qualquer analise.
2. Execute `@.claude/skills/review/` sem suavizar regras, sem assumir conformidade por inferencia e sem tratar `Status: done` como evidencia suficiente.
3. Valide estritamente contra o conjunto de especificacao local:
   - `.specs/prd-platform-mastra/prd.md`
   - `.specs/prd-platform-mastra/techspec.md`
   - `.specs/prd-platform-mastra/tasks.md`
   - `.specs/prd-platform-mastra/task-*.md`
   - `.specs/prd-platform-mastra/*_execution_report.md`
   - `.specs/prd-platform-mastra/_orchestration_report.md`
   - `.specs/prd-platform-mastra/adr-*.md` quando o diff tocar o tema correspondente
4. Para wiring, bootstrap e integracao real do sistema, parta obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`. Nao use `internal/platform/runtime` como ponto de partida.
5. Considere como escopo minimo de conformidade:
   - todos os RFs do PRD aplicaveis;
   - objetivos, restricoes, metricas de sucesso e fora de escopo;
   - exigencias da techspec;
   - criterios de sucesso de cada `task-*.md`;
   - criterios de aceite e DoD dos `*_execution_report.md`;
   - verificacao final em `_orchestration_report.md`.
6. Nao aceite conformidade parcial. Qualquer RF, criterio, DoD, migracao, teste, gate, evidencia ou comportamento esperado nao comprovado deve virar achado ou risco residual explicitamente classificado.
7. Se houver divergencia entre implementacao e documentacao:
   - use o working tree atual como fonte da verdade tecnica;
   - classifique o drift contra a especificacao de forma explicita;
   - nao invente comportamento ausente para fechar lacuna.
8. A revisao deve checar, no minimo:
   - paridade funcional prometida para agent, tool, workflow, memory, scorer e storage em `internal/platform`;
   - ausencia de semantica de dominio em `internal/platform`;
   - preservacao do kernel `internal/platform/workflow` sem LLM e sem dominio;
   - OpenRouter como canal oficial de LLM/embeddings;
   - Postgres como persistencia unica, incluindo pgvector;
   - structured output validado na fronteira e, em streaming, na conclusao do stream;
   - suspend/resume com merge-patch e idempotencia;
   - observabilidade na stack existente, sem trilha paralela e sem labels de alta cardinalidade;
   - suite weather e conformidade E2E/integracao conforme especificacao;
   - remocao das tabelas `agent_*` e criacao do storage `platform_*` conforme a spec;
   - tipos fechados nas fronteiras publicas.
9. Execute a revisao contra o diff apropriado:
   - se `AI_REVIEW_PRIOR_SHA` existir, revise apenas o delta da remediacao;
   - caso contrario, use a base apropriada da branch atual, preferencialmente `git diff --merge-base origin/main`.
10. Dispare subagentes especializados quando aumentarem a qualidade e reduzirem falso positivo, por exemplo:
   - `code-review` para diff relevante e regressao;
   - `security-review` para OpenRouter, streaming, persistencia e superficies de input/output;
   - `research` ou `explore` para confrontos amplos entre spec, ADRs, reports e implementacao;
   - `rubber-duck` para desafiar conclusoes borderline antes de emitir finding bloqueante.
11. Cada finding deve ter evidencia direta e auditavel:
   - severidade canonica;
   - arquivo e linha quando aplicavel;
   - item de especificacao violado (`RF`, task, criterio de aceite, DoD, ADR ou metrica);
   - impacto objetivo;
   - `fix_hint` enxuto;
   - reproducao ou lacuna de evidencia quando aplicavel.
12. Proibido emitir finding especulativo. Se a evidencia nao fechar, classifique como risco residual ou `BLOCKED`, nunca como defeito certo.
13. Se qualquer problema real for encontrado, gere bugs no formato canonico esperado pela skill e execute `@.claude/skills/bugfix/` para corrigir pela causa raiz, com testes de regressao obrigatorios e evidencia rastreavel.
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
  - RFs relevantes
  - criterios de sucesso das tasks
  - criterios de aceite
  - DoD
  - metricas de sucesso
- `next_action`

Gate final obrigatorio antes de declarar `APPROVED`:
- todos os RFs aplicaveis marcados como atendidos com evidencia;
- todos os criterios de sucesso das tasks atendidos com evidencia;
- todos os criterios de aceite atendidos com evidencia;
- DoD 100% atendido com evidencia;
- nenhuma lacuna aberta;
- nenhum falso positivo mantido;
- nenhum risco residual que contradiga a especificacao;
- conformidade total com `.specs/prd-platform-mastra`.
```

## Justificativas do enriquecimento

1. **Escopo fechado e verificavel:** o prompt passou a enumerar explicitamente `prd.md`, `techspec.md`, `tasks.md`, `task-*.md`, `*_execution_report.md`, `_orchestration_report.md` e ADRs relevantes para evitar revisao parcial.
2. **Fonte da verdade definida:** foi adicionado que o working tree atual prevalece tecnicamente, mas qualquer drift contra a especificacao deve ser apontado, reduzindo falsos positivos e mascaramento de lacunas.
3. **Criterios mensuraveis:** o prompt agora exige cobertura explicita de RFs, criterios de sucesso, criterios de aceite, DoD e metricas de sucesso, todos com evidencia.
4. **Ciclo operacional completo:** a regra `review -> bugfix -> review` foi transformada em contrato claro, com uso do delta de remediacao quando `AI_REVIEW_PRIOR_SHA` existir.
5. **Reducao de ruído:** o texto proibe findings especulativos e exige evidencias auditaveis, alinhando o pedido de `0 falsos positivos`.
6. **Ancoragem arquitetural:** foram inseridos checkpoints especificos da plataforma Mastra em `internal/platform`, incluindo kernel puro, OpenRouter, pgvector, weather, storage e tipos fechados.
7. **Subagentes com criterio:** o prompt nao pede subagentes genericamente; ele orienta quando cada tipo agrega sinal real a revisao.

## Variantes

### Variante recomendada

Usar exatamente o prompt enriquecido acima, porque ele combina cobertura documental completa, controle de falso positivo e ciclo de remediacao ate `APPROVED`.

### Variante mais conservadora

Manter o mesmo prompt, mas exigir duas passagens explicitas por rodada:
1. conformidade documental contra a spec;
2. confrontacao tecnica contra diff/working tree e testes.

Essa variante pode aumentar custo e tempo, mas e valida quando a branch tiver diff grande ou alta dispersao de arquivos.
