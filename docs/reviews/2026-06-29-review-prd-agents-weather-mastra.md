# Prompt Enriquecido — Review criterioso contra `.specs/prd-agents-weather-mastra`

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-agents-weather-mastra
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
Objetivo: executar uma revisão de dono do código, sem flexibilização, contra a especificação completa de `.specs/prd-agents-weather-mastra`, e somente encerrar quando o estado final estiver `APPROVED` com conformidade integral comprovada por evidência do código e dos artefatos. Não aceite `APPROVED_WITH_REMARKS`. Qualquer gap, lacuna, critério não atendido, item não verificável, risco residual relevante ou falso positivo invalida a rodada.

Contrato de carga obrigatório:
1. Ler `AGENTS.md` e confirmar o contrato base do repositório.
2. Carregar `.claude/skills/review/SKILL.md` como skill principal.
3. Se houver qualquer achado acionável, carregar `.claude/skills/bugfix/SKILL.md` para a remediação e voltar imediatamente para nova rodada de review.
4. Usar o working tree atual como fonte primária da verdade para o código; usar a spec como contrato obrigatório; usar execution reports apenas como evidência auxiliar, nunca como substituto da validação no código real.

Escopo obrigatório de confronto:
1. `.specs/prd-agents-weather-mastra/prd.md`
2. `.specs/prd-agents-weather-mastra/techspec.md`
3. `.specs/prd-agents-weather-mastra/tasks.md`
4. Todos os arquivos `task-*.md` dentro de `.specs/prd-agents-weather-mastra/`
5. Todos os arquivos `adr-*.md` dentro de `.specs/prd-agents-weather-mastra/`
6. `AGENTS.md`, especialmente regras de governança, layering, DMMF, workflow/tool, zero comentários em Go, adapters finos, validações e restrições de `internal/agent` vs `internal/platform/agent`

Modo de revisão:
1. Revisar o diff correto e também a superfície real impactada no workspace, sem depender apenas do diff quando a spec exigir comprovação cross-cutting.
2. Confrontar explicitamente cada RF do PRD, cada decisão obrigatória da techspec, cada critério de sucesso/aceite das tasks e o DoD implícito da entrega inteira.
3. Tratar como bloqueante qualquer item:
   - não atendido
   - parcialmente atendido
   - atendido sem evidência suficiente
   - não verificável pelo código/testes/artefatos
   - em desacordo com `AGENTS.md`
4. Não suavizar ausência de evidência como “risco residual aceitável”. Para esta execução, risco residual relevante conta como gap.
5. Não levantar finding especulativo. Se não houver evidência suficiente para afirmar defeito, investigue mais antes de registrar. A meta é 0 falsos positivos.

Checklist mínimo obrigatório de conformidade:
1. Todos os RF-01 até RF-30 devem estar atendidos com evidência concreta.
2. Todas as tasks 1.0 até 7.0 devem ser confrontadas contra seus critérios de sucesso, testes da tarefa e arquivos relevantes, independentemente do status escrito em `tasks.md`.
3. Confirmar as decisões centrais:
   - `internal/agents` consome `internal/platform` sem reimplementar mecanismo
   - weather-agent/tool/workflow/scorers/memory/semantic recall/Run auditável/WhatsApp E2E existem conforme a spec
   - indexação assíncrona de embeddings está conectada e idempotente
   - `internal/agent` foi eliminado 100% sem resíduo de produção, testes, wiring ou config órfã
   - onboarding conversacional do WhatsApp foi desligado conforme a decisão do PRD
   - gates de governança, build, vet, testes e formatação exigidos pela spec permanecem coerentes
4. Confirmar aderência às regras hard do repositório, inclusive:
   - sem `init()`
   - sem `panic` em produção
   - zero comentários em Go de produção
   - adapters finos
   - `context.Context` nas fronteiras de IO
   - tipos fechados e DMMF onde exigido
   - kernel/workflow sem violação de fronteiras

Uso de subagentes especializados:
1. Dispare subagentes apenas quando aumentarem a qualidade da revisão.
2. Recomendações:
   - `code-review`: para leitura crítica do diff e regressões lógicas
   - `security-review`: para HTTP outbound, WhatsApp inbound/outbound, worker, outbox, credenciais, tracing e superfícies de integração
   - `explore`: para auditoria cross-cutting de referências, wiring, imports e resíduos de `internal/agent`
3. Não delegue o mesmo escopo duas vezes sem necessidade. Consolide os achados em uma decisão única.

Ciclo obrigatório review → bugfix → review:
1. Execute `review`.
2. Se existir qualquer finding, gere bugs no formato canônico exigido pela skill `bugfix`.
3. Execute `bugfix` somente sobre os achados reais confirmados.
4. Reexecute `review` focando pelo menos no delta da remediação e reconfirmando que o conjunto da spec continua íntegro.
5. Repita até que o veredito final seja `APPROVED`.
6. Se surgir impedimento externo real e incontornável, retorne `BLOCKED` com evidência objetiva; nunca invente conformidade.

Critério de aprovação final:
1. `APPROVED` é permitido somente se:
   - não houver findings
   - não houver riscos residuais relevantes
   - não houver itens “não verificáveis”
   - todos os critérios de aceite estiverem atendidos
   - DoD estiver 100% atendido
   - houver 0 gaps, 0 lacunas e 0 falsos positivos
2. Qualquer outra condição exige nova rodada de investigação ou remediação.

Formato mínimo da resposta:
1. `verdict` canônico da rodada: `BLOCKED`, `REJECTED`, `APPROVED_WITH_REMARKS` ou `APPROVED`
2. `files_reviewed`
3. `refs_loaded`
4. `acceptance_matrix` cobrindo RFs, tasks, ADRs e DoD com status `atendido` ou `bloqueado`
5. `findings` na última rodada
6. `residual_risks`
7. `validations_run`
8. `review_cycles` resumindo quantas rodadas review/bugfix/review foram necessárias

Regra de encerramento:
1. O fluxo completo só pode encerrar quando a última rodada retornar `APPROVED`.
2. `REJECTED` ou `APPROVED_WITH_REMARKS` exigem nova iteração; `BLOCKED` só é aceitável diante de impedimento externo real e comprovado.

Restrições adicionais desta execução:
1. Não encerrar com `APPROVED_WITH_REMARKS`.
2. Não tratar documento histórico como prova suficiente sem confirmação no código real.
3. Não declarar conformidade parcial como suficiente.
4. Não omitir divergência entre PRD, techspec, tasks, ADRs e working tree.
5. Não introduzir falso positivo para “forçar rigor”.
```

## Justificativas do enriquecimento

1. **Escopo fechado e auditável:** explicita que a revisão deve confrontar PRD, techspec, tasks, task files e ADRs, evitando review incompleto.
2. **Fonte da verdade bem definida:** combina working tree atual para código real com a spec como contrato obrigatório, reduzindo ambiguidade e falso positivo documental.
3. **Critério de aprovação endurecido:** transforma qualquer item não verificável, parcial ou com risco residual em bloqueio, alinhando com o requisito de 0 gaps e DoD 100%.
4. **Ciclo operacional claro:** deixa determinístico quando acionar `bugfix` e quando reexecutar `review`, evitando encerramento prematuro.
5. **Uso orientado de subagentes:** adiciona delegação especializada apenas onde aumenta a qualidade da revisão, sem duplicação de escopo.
6. **Saída final estruturada:** facilita auditar se a aprovação realmente cobriu RFs, tasks, ADRs, validações e número de ciclos.

## Ambiguidades tratadas

1. **`tasks.md` com status pendentes vs. código atual:** o prompt manda validar o código real e não confiar no status textual das tasks.
2. **Execution reports vs. código:** os relatórios viram apenas evidência auxiliar; o código e a spec continuam mandatórios.
3. **Risco residual:** para esta execução, risco residual relevante não é aceitável e impede `APPROVED`.

## Variante recomendada

Use este prompt sem reduzir escopo. Se precisar de uma versão ainda mais rígida, mantenha o mesmo conteúdo e acrescente a persistência obrigatória do artefato de review em `evidence/<task>/review.md` quando a execução estiver vinculada a task específica.
