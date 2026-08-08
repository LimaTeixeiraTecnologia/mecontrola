# Review PRD - indicador-digitando-whatsapp

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilizacao, validando estritamente contra .specs/prd-indicador-digitando-whatsapp

Criterios obrigatorios:
* Todos os criterios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* 0 ressalvas.
* Todas as regras de negocio atendidas (implementadas).

Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review -> bugfix -> review ate obter APPROVED, sem falsos positivos e em conformidade total com a especificacao.

Dispare subagentes especializados quando agregarem qualidade a revisao.
```

## Prompt enriquecido (recomendado)

```xml
<goal>
Obter veredito final APPROVED, sem falsos positivos, validando a branch/worktree atual contra .specs/prd-indicador-digitando-whatsapp ate comprovar conformidade total com o PRD, a techspec, as tasks, os DoDs e as regras de negocio.
</goal>

<context>
- Fonte canonica de regras do repositorio: AGENTS.md.
- Skill principal obrigatoria: @.claude/skills/review/
- Skill de remediacao obrigatoria quando houver achados: @.claude/skills/bugfix/
- Especificacao alvo: .specs/prd-indicador-digitando-whatsapp/
- Ler obrigatoriamente, antes do primeiro veredito: prd.md, techspec.md, tasks.md, task-*.md, *_execution_report.md e adr-*.md dessa pasta.
- Validar contra o codigo real da branch/worktree atual e contra o diff apropriado da branch; quando o diff sozinho nao bastar para comprovar um criterio, inspecionar o workspace e as evidencias necessarias em vez de assumir.
- Contexto relevante encontrado agora: tasks.md desta especificacao ainda marca 5.0 e 6.0 como pending; trate isso como forte indicio de lacuna e so descarte esse indicio se houver evidencia objetiva no codigo, nos testes e nos artefatos de execucao de que o escopo correspondente foi concluido fora da atualizacao documental.
</context>

<task>
1. Executar @.claude/skills/review/ de forma estrita, sem flexibilizacao, sobre a implementacao relacionada a .specs/prd-indicador-digitando-whatsapp.
2. Conferir exaustivamente a implementacao contra:
   - RF-01 ate RF-10 do PRD;
   - funcionalidades core;
   - objetivos e restricoes tecnicas;
   - regras de negocio explicitas e implicitas documentadas;
   - criterios de sucesso/aceite de cada task file;
   - DoD de cada execution report;
   - ADRs e decisoes arquiteturais registradas;
   - zero regressao com a flag desligada;
   - gate RF-07 e suas evidencias obrigatorias.
3. Se o diff for grande demais para uma unica passada rigorosa, fatiar a revisao em trilhas independentes e depois consolidar o veredito unico. Trilhas sugeridas:
   - configuracao/feature flag;
   - client Meta/payload typing_indicator;
   - gateway WhatsApp;
   - consumer e metrica;
   - wiring module/worker;
   - gate RF-07, validacao ampla e evidencias de zero regressao.
4. Disparar subagentes especializados quando aumentarem a cobertura ou a qualidade da revisao sem duplicar leitura do mesmo escopo.
5. Ao encontrar qualquer problema real, emitir achados com evidencia primaria, gerar a lista canonica de bugs para @.claude/skills/bugfix/, acionar a remediacao e repetir o ciclo review -> bugfix -> review.
6. Repetir o ciclo ate atingir APPROVED real, sem ressalvas, sem achados remanescentes e sem pendencias de evidencia.
</task>

<rules>
- Nao flexibilizar nenhum criterio do PRD, da techspec, das tasks, dos DoDs ou das regras de governanca.
- Nao considerar "parcialmente implementado" como atendido.
- Nao considerar "provavel", "aparenta", "deve estar coberto" ou inferencias sem evidencia como aprovacao.
- Nao emitir falsos positivos: todo achado precisa apontar arquivo e linha quando houver codigo, ou o artefato/documento ausente quando o problema for lacuna de evidencia.
- Nao esconder gaps como risco residual suave; se o item e obrigatorio e nao foi comprovado, trate como finding bloqueante.
- Nao encerrar com APPROVED_WITH_REMARKS. O alvo final deste fluxo e exclusivamente APPROVED.
- Nao concluir APPROVED se existir qualquer gap, lacuna, teste faltante relevante, DoD incompleto, task pendente material, validacao ausente ou regra de negocio nao comprovada.
- Se a skill review exigir recorte por budget, fazer multiplas passadas focadas e consolidar; nao usar o budget como desculpa para reduzir rigor.
- Se uma validacao obrigatoria nao puder ser comprovada, tratar como BLOCKED ou REJECTED na rodada, acionar bugfix quando houver correcao cabivel e seguir o ciclo ate eliminar a causa.
</rules>

<acceptance_checks>
- PRD:
  - todos os objetivos preservados;
  - funcionalidades core 1..3 implementadas;
  - RF-01..RF-10 atendidos com evidencia objetiva;
  - regras de negocio e restricoes tecnicas respeitadas;
  - fora de escopo nao violado.
- Tasks:
  - cada task-*.md com criterios de sucesso/aceite 100% implementados ou evidenciados;
  - nenhuma dependencia critica pendente que invalide o escopo do PRD.
- Execution reports:
  - cada *_execution_report.md com DoD 100% atendido;
  - comandos, evidencias e resultados coerentes com o que o codigo entrega.
- Zero regressao:
  - comportamento com flag desligada identico ao baseline esperado;
  - contagens, mocks, e2e, integracao e contratos existentes preservados.
- RF-07:
  - ha evidencia real suficiente para autorizar ativacao, ou
  - a ausencia dessa evidencia impede aprovacao plena do criterio correspondente.
</acceptance_checks>

<output>
Em cada rodada, retornar no minimo:
1. verdict: APPROVED | APPROVED_WITH_REMARKS | REJECTED | BLOCKED
2. files_reviewed
3. refs_loaded
4. traceability_matrix com uma linha por requisito/criterio/DoD contendo: item, status, evidencia, lacuna
5. findings no formato da skill review
6. residual_risks
7. validations_run

Se houver findings:
1. gerar tambem a lista canonica de bugs para consumo da skill bugfix;
2. executar @.claude/skills/bugfix/ com foco em causa raiz e regressao;
3. repetir @.claude/skills/review/ apos a remediacao, revisando o delta e revalidando os criterios impactados;
4. continuar o ciclo ate zerar findings e obter APPROVED.

Resposta final aceitavel:
1. verdict = APPROVED
2. confirmacao explicita de que todos os criterios de aceite foram implementados
3. confirmacao explicita de que o DoD esta 100% atendido
4. confirmacao explicita de 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas
5. confirmacao explicita de que todas as regras de negocio foram implementadas e comprovadas
</output>

<example>
Exemplo de linha da traceability_matrix:
- RF-04 | atendido | internal/.../consumer.go:123-167 + teste X cobre falha best-effort sem bloquear resposta | nenhuma

Exemplo de finding valido:
- severity: high
  file: .specs/prd-indicador-digitando-whatsapp/tasks.md
  line: 15
  impact: task 5.0 permanece pendente e o wiring module/worker obrigatorio para RF-05/RF-06/RF-09 nao esta comprovado como concluido
  fix_hint: concluir o wiring e anexar evidencia de validacao correspondente
</example>
```

## Justificativas das adicoes

- **Escopo fechado e rastreavel:** amarrei explicitamente PRD, techspec, tasks, execution reports e ADRs para evitar review incompleto.
- **Criterios mensuraveis:** transformei "sem lacunas" e "DoD 100%" em checks verificaveis por matriz de rastreabilidade.
- **Controle de falso positivo:** exigi evidencia primaria por arquivo/linha ou artefato ausente, evitando achados especulativos.
- **Tratamento do budget da skill review:** converti o limite operacional em fatiamento rigoroso, nao em relaxamento da analise.
- **Loop deterministico review -> bugfix -> review:** deixei claro que APPROVED_WITH_REMARKS nao encerra o fluxo.
- **Contexto atual da spec:** registrei que tasks.md ainda aponta 5.0 e 6.0 como pending, o que ajuda a evitar aprovacao indevida sem evidencia objetiva.

## Variante curta

Se quiser uma versao mais compacta para colar direto no agente, use apenas a secao `Prompt enriquecido (recomendado)`.
