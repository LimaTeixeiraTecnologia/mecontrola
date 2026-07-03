# Review PRD - infra-evolucao-kvm2-10k

- Data: 2026-07-03
- PRD alvo: `.specs/prd-infra-evolucao-kvm2-10k`
- Idioma: pt-BR
- Objetivo: prompt enriquecido para executar um ciclo estrito `review -> bugfix -> review` ate obter `APPROVED`, sem falsos positivos, sem lacunas e com aderencia total a especificacao.

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilizacao, validando estritamente contra .specs/prd-infra-evolucao-kvm2-10k
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
Não implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em docs/reviews/.
```

## Prompt enriquecido

```text
Voce vai conduzir uma revisao de conformidade total da implementacao entregue para o PRD `.specs/prd-infra-evolucao-kvm2-10k`, executando obrigatoriamente um ciclo deterministico `review -> bugfix -> review` ate o veredito final ser exatamente `APPROVED`.

Objetivo inegociavel:
- aprovar somente se houver 100% de aderencia ao PRD, techspec, tasks, task files, execution reports, criterios de aceite, criterios de sucesso, DoD e regras de negocio;
- aceitar somente o estado final com:
  - 0 gaps
  - 0 lacunas
  - 0 falsos positivos
  - 0 ressalvas
  - 100% dos criterios implementados e comprovados com evidencia

Contrato de carga base obrigatorio:
1. Ler `AGENTS.md` e cumprir suas regras sem flexibilizacao.
2. Assumir o working tree atual como fonte da verdade.
3. Em qualquer analise de implementacao que toque codigo Go, respeitar a governanca do repositorio.
4. Em qualquer necessidade de raciocinar sobre bootstrap/wiring, partir obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`.
5. Nao referenciar `internal/platform/runtime` como ponto de partida.

Skills obrigatorias no fluxo:
1. Executar `@.claude/skills/review/` como skill principal de auditoria.
2. Se houver qualquer finding real, material e acionavel, converter os achados para o formato canonico exigido e executar `@.claude/skills/bugfix/`.
3. Apos cada rodada de bugfix, executar nova rodada de `@.claude/skills/review/`.
4. Repetir o ciclo ate obter `APPROVED`.
5. Nao encerrar com `APPROVED_WITH_REMARKS`.
6. Nao encerrar com risco residual, ressalva, "na pratica ok", "provavelmente atende" ou qualquer aprovacao por aproximacao.

Escopo documental obrigatorio:
Leia e confronte, no minimo, os seguintes artefatos:
1. `.specs/prd-infra-evolucao-kvm2-10k/prd.md`
2. `.specs/prd-infra-evolucao-kvm2-10k/techspec.md`
3. `.specs/prd-infra-evolucao-kvm2-10k/tasks.md`
4. todos os `task-*.md` sob `.specs/prd-infra-evolucao-kvm2-10k/`
5. todos os `*_execution_report.md` / `*.0_execution_report.md` sob `.specs/prd-infra-evolucao-kvm2-10k/`

Escopo de validacao obrigatorio:
1. Validar todos os RFs do PRD, sem excecao.
2. Validar todos os criterios de aceite globais do PRD.
3. Validar todos os criterios de sucesso de cada task.
4. Validar todos os blocos `Definition of Done (DoD)` presentes nos execution reports.
5. Validar todas as regras de negocio e restricoes declaradas no PRD e na techspec.
6. Validar que a implementacao entregue corresponde ao escopo declarado e que nao deixou itens parcialmente implementados, mascarados ou apenas documentados sem entrega real quando a especificacao exige implementacao.
7. Validar evidencias de testes, build, deploy, operacao, observabilidade, backup/restore, carga e hardening quando esses itens forem exigidos pela especificacao.

Regras de auditoria:
1. Seja extremamente criterioso e aja como code owner.
2. Nao invente requisito, comportamento, arquivo, regra ou evidencias ausentes.
3. Nao aceite "quase pronto", "coberto parcialmente", "fora do diff mas presumido", "nao consegui provar mas parece correto" ou equivalentes.
4. Todo finding deve ter evidencia objetiva, citando arquivo(s), linha(s) quando aplicavel, impacto, criterio/RF/DoD violado e dica de correcao.
5. Se nao houver evidencia suficiente para afirmar conformidade, trate como `BLOCKED` ou finding real; nunca como aprovacao.
6. Se houver qualquer criterio nao atendido ou nao comprovado, o veredito nao pode ser `APPROVED`.
7. Se houver divergencia entre documentos historicos e codigo atual, o working tree atual prevalece como fonte da verdade para o estado implementado, mas a aprovacao continua condicionada a aderencia integral a especificacao alvo.

Subagentes:
1. Dispare subagentes especializados apenas quando aumentarem a qualidade ou a cobertura da revisao sem redundancia.
2. Priorize subagentes para trilhas independentes como seguranca, diffs extensos, validacao de carga/infra ou correlacao entre varios artefatos.
3. Nao delegue de forma decorativa.
4. Consolide o resultado final em um unico veredito canonico.

Politica para bugfix:
1. So acione `@.claude/skills/bugfix/` se houver finding real e evidenciado.
2. Passe os bugs no formato canonico exigido pela skill.
3. Exija correcao pela causa raiz, com testes/regressoes/evidencias requeridas pela skill.
4. Apos o bugfix, revise apenas o delta de remediacao quando aplicavel, sem perder o confronto contra a especificacao.
5. Continue o ciclo ate zerar todos os findings.

Formato minimo obrigatorio da saida de cada review:
1. `verdict`
2. `files_reviewed`
3. `refs_loaded`
4. `findings`
5. `residual_risks`
6. `validations_run`
7. matriz de rastreabilidade contendo:
   - cada RF do PRD
   - cada criterio de aceite global
   - cada criterio de sucesso por task
   - cada bloco DoD por execution report
   - status: `atendido`, `nao atendido` ou `nao verificavel`
   - evidencia objetiva por item

Condicao de parada:
1. Pare somente em `APPROVED`.
2. `APPROVED` so e permitido quando todos os itens da matriz de rastreabilidade estiverem `atendido`.
3. Se restar qualquer item `nao atendido` ou `nao verificavel`, continue no ciclo ou retorne `BLOCKED`/`REJECTED`, conforme a skill.

Entrega final esperada:
- um veredito final `APPROVED`
- sem ressalvas
- sem riscos residuais relevantes
- sem findings abertos
- com rastreabilidade completa entre especificacao e implementacao
- com evidencias suficientes para sustentar a aprovacao sem falso positivo
```

## Justificativas curtas

1. O prompt enriquecido fixa explicitamente o conjunto de documentos obrigatorios, evitando revisao superficial limitada ao PRD principal.
2. A matriz de rastreabilidade reduz falso positivo porque obriga prova item a item para RF, criterios globais, criterios de sucesso e DoD.
3. A condicao de parada foi tornada deterministica: somente `APPROVED`, nunca `APPROVED_WITH_REMARKS`.
4. O ciclo com `bugfix` foi preservado, mas condicionado a findings canonicos e evidenciados.
5. As regras do repositorio que impactam a auditoria foram incorporadas, incluindo working tree como fonte da verdade e ponto de partida por `cmd/server/server.go` e `cmd/worker/worker.go`.

## Variantes

Sem variante recomendada. O objetivo aqui e deterministico e pede tolerancia zero a lacunas, entao a versao acima e a mais aderente.
