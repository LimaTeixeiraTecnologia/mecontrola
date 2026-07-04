# Review PRD - transactions-crud-unificado

- Data: 2026-07-04
- PRD alvo: `.specs/prd-transactions-crud-unificado`
- Idioma: pt-BR
- Objetivo: prompt enriquecido para executar uma revisao estrita contra a especificacao, com ciclo deterministico `review -> bugfix -> review` ate `APPROVED`, sem falso positivo, sem lacuna e sem ressalva.

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilizacao, validando estritamente contra .specs/prd-transactions-crud-unificado
Criterios obrigatorios:
* Todos os criterios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* 0 ressalvas
* Todas Regras de negocio atendidos (implementados)
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review -> bugfix -> review ate obter APPROVED, sem falsos positivos e em conformidade total com a especificacao.
Dispare subagentes especializados quando agregarem qualidade a revisao.
Nao implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em docs/reviews/.
```

## Ambiguidades tratadas

1. A linha `Nao implemente nada. Apenas crie/enriqueca o prompt...` pertence a esta tarefa atual e nao ao executor futuro do review; por isso ela nao entra no prompt enriquecido.
2. O alvo correto de validacao nao e apenas `prd.md`: a revisao precisa confrontar tambem `techspec.md`, `tasks.md`, `task-*.md` e `*_execution_report.md` do mesmo PRD para evitar aprovacao superficial.
3. O veredito precisa ser deterministico: `APPROVED` somente com 100% de rastreabilidade comprovada; qualquer item ausente, nao implementado ou nao verificavel bloqueia aprovacao.

## Prompt enriquecido

```text
Voce vai executar uma revisao de conformidade total da implementacao referente a `.specs/prd-transactions-crud-unificado`, usando obrigatoriamente `@.claude/skills/review/` como skill principal e mantendo tolerancia zero a falso positivo, lacuna, gap ou ressalva.

Objetivo inegociavel:
- aprovar somente com veredito final exatamente `APPROVED`;
- exigir 100% de aderencia implementada e comprovada contra PRD, techspec, tasks, task files, execution reports, Definition of Done (DoD), criterios de aceite, criterios de sucesso e regras de negocio;
- aceitar somente estado final com:
  - 0 gaps
  - 0 lacunas
  - 0 falsos positivos
  - 0 ressalvas
  - 100% dos itens obrigatorios implementados e evidenciados

Contrato base obrigatorio:
1. Ler `AGENTS.md` e seguir integralmente as regras canonicas do repositorio, sem flexibilizacao.
2. Assumir o working tree atual como fonte da verdade para o estado implementado.
3. Em analise de wiring, bootstrap, registro de modulos, rotas, jobs ou consumers, partir obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`.
4. Nao usar `internal/platform/runtime` como ponto de partida.
5. Nao inventar comportamento, requisito, arquivo, evidencia, cobertura ou implementacao ausente.

Escopo documental obrigatorio:
Leia e confronte, no minimo:
1. `.specs/prd-transactions-crud-unificado/prd.md`
2. `.specs/prd-transactions-crud-unificado/techspec.md`
3. `.specs/prd-transactions-crud-unificado/tasks.md`
4. todos os `task-*.md` sob `.specs/prd-transactions-crud-unificado/`
5. todos os `*_execution_report.md` e `*.0_execution_report.md` sob `.specs/prd-transactions-crud-unificado/`
6. o diff/working tree efetivamente entregue

Escopo de validacao obrigatorio:
1. Validar todos os RFs do PRD, sem excecao.
2. Validar todos os criterios de aceite globais do PRD.
3. Validar todos os criterios de sucesso/aceite de cada task file.
4. Validar todos os blocos de DoD e evidencias declaradas nos execution reports.
5. Validar todas as regras de negocio e restricoes tecnicas declaradas no PRD e na techspec.
6. Validar que o escopo entregue removeu por completo tudo que o PRD manda remover, sem manter superficie morta, opcional, depreciada ou paralela.
7. Validar que itens marcados como `pending`, `blocked`, `failed` ou sem evidencia objetiva nao sejam considerados implementados.
8. Validar que nenhum requisito foi apenas documentado, parcialmente entregue ou presumido sem implementacao real quando a especificacao exige comportamento implementado.

Regras de auditoria:
1. Aja como code owner do repositorio.
2. Todo finding precisa de evidencia objetiva: arquivo, linha quando aplicavel, impacto, criterio/RF/DoD violado e dica de correcao.
3. Se nao houver evidencia suficiente para provar conformidade, trate como `BLOCKED` ou finding real; nunca como aprovacao.
4. Se qualquer RF, criterio de aceite, criterio de sucesso, DoD ou regra de negocio estiver ausente, parcial, nao implementado ou nao verificavel, o veredito nao pode ser `APPROVED`.
5. Nao aceite "quase pronto", "coberto parcialmente", "fora do diff mas presumido", "parece correto", "risco residual aceitavel" ou equivalentes.
6. Nao produzir falso positivo: nao aponte defeito sem evidencia no codigo, no diff, nos testes, nas validacoes ou na especificacao.
7. Nao produzir falso negativo: nao deixe passar requisito sem prova concreta de implementacao.

Subagentes:
1. Dispare subagentes especializados apenas quando aumentarem qualidade real da revisao.
2. Use subagentes para trilhas independentes, por exemplo: seguranca, correlacao entre PRD e diff, auditoria de migracoes, contratos HTTP/OpenAPI, regressao em workflows/agents ou validacao de remediacoes amplas.
3. Nao delegue por decoracao e nao duplique trabalho.
4. Consolide tudo em um unico veredito canonico.

Fluxo obrigatorio:
1. Executar `@.claude/skills/review/`.
2. Se houver qualquer finding real, material e acionavel, converter os achados para o formato canonico da skill `bugfix` e executar `@.claude/skills/bugfix/`.
3. Apos cada rodada de bugfix, executar nova rodada de `@.claude/skills/review/`.
4. Repetir o ciclo `review -> bugfix -> review` ate zerar todos os findings bloqueantes.
5. Nao encerrar com `APPROVED_WITH_REMARKS`.
6. Nao encerrar com ressalva, risco residual aberto, "aceitavel para agora" ou aprovacao por aproximacao.

Politica de bugfix:
1. Acione `@.claude/skills/bugfix/` somente para findings reais e evidenciados.
2. Exija correcao pela causa raiz, com testes de regressao e evidencias de validacao conforme a skill.
3. Se a remediacao introduzir novo desvio contra a especificacao, rejeite e reabra o ciclo.
4. Em rodadas pos-bugfix, revise no minimo o delta da remediacao e reconfronte os criterios impactados.

Formato minimo obrigatorio da saida de cada review:
1. `verdict`
2. `files_reviewed`
3. `refs_loaded`
4. `findings`
5. `residual_risks`
6. `validations_run`
7. matriz de rastreabilidade completa contendo, para cada item:
   - identificador do requisito/criterio/DoD
   - fonte (`prd`, `techspec`, `tasks`, `task file`, `execution report`)
   - status: `atendido`, `nao atendido` ou `nao verificavel`
   - evidencia objetiva

Condicao de parada:
1. Pare somente em `APPROVED`.
2. `APPROVED` so e permitido quando todos os itens da matriz de rastreabilidade estiverem `atendido`.
3. Se existir qualquer item `nao atendido` ou `nao verificavel`, continue o ciclo ou retorne `BLOCKED`/`REJECTED`, conforme a skill.

Entrega final esperada:
- veredito final `APPROVED`
- sem ressalvas
- sem findings abertos
- sem lacunas
- sem gaps
- sem falsos positivos
- com rastreabilidade completa entre especificacao e implementacao
- com evidencia suficiente para sustentar a aprovacao de forma objetiva
```

## Justificativas curtas

1. O prompt enriquecido amplia o alvo de validacao para todo o bundle do PRD, reduzindo o risco de review superficial.
2. A matriz de rastreabilidade torna a aprovacao verificavel item a item, o que reduz falso positivo e falso negativo.
3. A condicao de parada foi explicitada para impedir `APPROVED_WITH_REMARKS`, risco residual aberto ou aprovacao por aproximacao.
4. O ciclo com `bugfix` foi mantido, mas agora condicionado a findings canonicos, evidenciados e auditaveis.
5. As regras especificas do repositorio que impactam a revisao foram incorporadas: `AGENTS.md` como fonte canonica, working tree como verdade atual e ponto de partida por `cmd/server/server.go` e `cmd/worker/worker.go`.

## Variante

Sem variante recomendada. O objetivo pedido e deterministico e exige tolerancia zero a lacunas e ressalvas.
