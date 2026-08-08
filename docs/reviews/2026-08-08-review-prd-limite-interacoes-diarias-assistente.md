# Review PRD — limite-interacoes-diarias-assistente

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-limite-interacoes-diarias-assistente

Critérios obrigatórios:
* Todos os critérios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* 0 ressalvas
* Todas Regras de negócio atendidos (implementados)

Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.

Dispare subagentes especializados quando agregarem qualidade à revisão.
```

## Ajustes aplicados

| Ajuste | Justificativa |
|---|---|
| Escopo explícito dos artefatos da spec | Evita revisar só `prd.md` e esquecer `techspec.md`, `tasks.md`, ADRs e task files. |
| Critério de done determinístico | Elimina ambiguidade sobre quando o ciclo pode encerrar. |
| Regras anti-falso-positivo | Exige evidência concreta por achado e impede inferência sem base. |
| Cobertura completa de RF, regras de negócio, DoD e critérios de aceite | Reduz risco de gaps ou lacunas silenciosas. |
| Fluxo review → bugfix → review formalizado | Garante repetição canônica até `APPROVED` ou bloqueio real. |
| Formato de saída estruturado | Facilita auditoria, rastreabilidade e consumo da rodada seguinte. |

## Prompt enriquecido

```xml
<goal>
Obter um veredito final `APPROVED` para a implementação relacionada a `.specs/prd-limite-interacoes-diarias-assistente`, sem falsos positivos, sem gaps, sem lacunas, sem ressalvas e com aderência total ao PRD, à techspec, às tasks e às regras canônicas do repositório.
</goal>

<context>
- Repositório: `mecontrola`.
- Fonte canônica de governança: `AGENTS.md`.
- Skill principal obrigatória: `@.claude/skills/review/`.
- Skill de remediação obrigatória quando houver achado acionável: `@.claude/skills/bugfix/`.
- Spec alvo:
  - `.specs/prd-limite-interacoes-diarias-assistente/prd.md`
  - `.specs/prd-limite-interacoes-diarias-assistente/techspec.md`
  - `.specs/prd-limite-interacoes-diarias-assistente/tasks.md`
  - `.specs/prd-limite-interacoes-diarias-assistente/task-*.md`
  - `.specs/prd-limite-interacoes-diarias-assistente/adr-*.md`
- O review deve confrontar a implementação real do working tree e o diff relevante contra esses artefatos.
- Se o contexto necessário para isolar o diff ou a implementação alvo não estiver disponível, retornar `BLOCKED` em vez de assumir.
</context>

<task>
1. Carregue `AGENTS.md` e execute a skill `review` de forma estrita, usando a spec alvo como contrato obrigatório.
2. Valide exaustivamente a implementação contra PRD, techspec, tasks, task files, ADRs e regras do repositório.
3. Se houver qualquer achado real, gere os findings com evidência canônica, acione a skill `bugfix` para corrigir a causa raiz e repita o ciclo `review -> bugfix -> review`.
4. Continue iterando até obter `APPROVED` sem falsos positivos ou até encontrar um bloqueio real e explícito que impeça a conformidade total.
</task>

<rules>
- Não flexibilize critérios.
- Não aprove com `APPROVED_WITH_REMARKS`.
- Não aceite gaps, lacunas, ressalvas ou “depois ajustamos”.
- Não reporte achado sem evidência concreta em código, diff, teste, config, log ou artefato de validação.
- Não invente comportamento, arquivos, requisitos, cobertura ou contexto ausente.
- Não considere “parcialmente implementado” como atendido.
- Não trate item “não verificável” como aprovado; ou aprofunde a investigação, ou registre `BLOCKED`, ou rejeite com evidência.
- Use subagentes especializados quando isso aumentar a qualidade da revisão, mas mantenha `@.claude/skills/review/` como fluxo canônico e consolidado.
- Se a correção tocar Go, respeite integralmente as skills obrigatórias e a governança indicada em `AGENTS.md`.
</rules>

<acceptance_criteria>
- Todos os critérios de aceite das task files estão 100% implementados.
- Todo o DoD aplicável está 100% implementado.
- Todas as regras de negócio do PRD estão implementadas.
- Todos os RFs aplicáveis estão implementados e validados com evidência.
- A implementação está aderente à techspec e às ADRs relevantes.
- Nenhum comportamento fora de escopo foi introduzido para “compensar” requisito faltante.
- Nenhum falso positivo foi emitido.
- Nenhum gap ou lacuna permanece aberto.
- O veredito final só pode ser `APPROVED` se todos os itens acima forem verdadeiros ao mesmo tempo.
</acceptance_criteria>

<review_protocol>
- Monte uma matriz de cobertura item a item para:
  - objetivos
  - histórias de usuário
  - funcionalidades core
  - RF-01 a RF-14
  - experiência do usuário
  - restrições técnicas de alto nível
  - critérios de sucesso/aceite das task files
  - decisões materializadas na techspec e ADRs
- Para cada item, classifique apenas como:
  - `atendido`
  - `nao_atendido`
  - `bloqueado_por_falta_de_evidencia`
- Sempre anexe evidência objetiva: arquivo, linha, teste, comando, diff ou artefato.
- Se houver qualquer `nao_atendido`, o veredito da rodada deve ser `REJECTED`.
- Se houver qualquer `bloqueado_por_falta_de_evidencia`, o veredito da rodada deve ser `BLOCKED`.
- Só use `APPROVED` quando não existir nenhum item fora de `atendido`.
</review_protocol>

<bugfix_protocol>
- Para cada rodada rejeitada, converta os achados em formato canônico consumível pela skill `bugfix`.
- Corrija somente a causa raiz dos achados confirmados.
- Após cada bugfix, reexecute o review:
  - se `AI_REVIEW_PRIOR_SHA` estiver disponível, revise ao menos o delta da remediação;
  - ainda assim, reconcilie o estado final com toda a spec para garantir ausência de gaps regressivos.
- Repita o ciclo até `APPROVED`.
- Se surgir bloqueio externo real, pare e reporte `BLOCKED` com causa precisa.
</bugfix_protocol>

<output>
Retorne em markdown com as seções abaixo:

1. `Verdict`
2. `Cycles Executed`
3. `Files Reviewed`
4. `Spec Artifacts Reviewed`
5. `Coverage Matrix`
6. `Findings`
7. `Bugfix Actions`
8. `Residual Risks`
9. `Validations Run`
10. `Final Decision`

Regras da saída:
- `Verdict` deve ser um entre `APPROVED`, `REJECTED` ou `BLOCKED`.
- `Coverage Matrix` deve listar cada requisito/regra/critério relevante com status e evidência.
- `Findings` deve ficar vazio somente quando o veredito final for `APPROVED`.
- `Residual Risks` deve ficar vazio no estado final `APPROVED`.
- `Final Decision` deve afirmar explicitamente uma das opções:
  - `APPROVED: conformidade total com a especificação`
  - `BLOCKED: contexto insuficiente para provar conformidade`
  - `REJECTED: implementação não atende integralmente à especificação`
</output>

<definition_of_done>
O trabalho só termina quando houver `APPROVED` com conformidade total comprovada contra `.specs/prd-limite-interacoes-diarias-assistente`, zero falsos positivos, zero gaps, zero lacunas, zero ressalvas e 100% dos critérios de aceite, DoD e regras de negócio implementados.
</definition_of_done>
```
