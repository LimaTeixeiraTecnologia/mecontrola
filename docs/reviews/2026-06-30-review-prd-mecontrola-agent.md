# Prompt enriquecido para review rigorosa de `prd-mecontrola-agent`

## Metadados

- **Data:** 2026-06-30
- **Destino:** `docs/reviews/2026-06-30-review-prd-mecontrola-agent.md`
- **Idioma:** pt-BR
- **Objetivo:** gerar um prompt operacional, estrito e auditavel para revisar a entrega contra `.specs/prd-mecontrola-agent` e, se necessario, acionar `@.claude/skills/bugfix/` em ciclo ate `APPROVED`.

## Conflito identificado antes do enriquecimento

Ha um conflito de contexto na frase **"Nao implemente nada"**:

- para esta solicitacao atual, a frase vale para **quem esta criando o prompt**;
- para a execucao futura do prompt enriquecido, ela **nao pode** valer literalmente, porque o proprio objetivo exige o ciclo `review -> bugfix -> review` ate `APPROVED`.

Por isso, o prompt enriquecido abaixo preserva a intencao principal da revisao estrita e do ciclo de remediacao, mas remove essa restricao operacional do texto final.

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-mecontrola-agent
Critérios obrigatórios:
* Todos os critérios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* Todas Regras de negócio atendidos (implementados)
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
```

## Prompt enriquecido

```text
Execute `@.claude/skills/review/` de forma estrita, criteriosa e sem flexibilizacao no repositorio `/Users/jailtonjunior/Git/mecontrola`, usando `AGENTS.md` como fonte canonica e o working tree atual como fonte da verdade tecnica.

Objetivo final obrigatorio:
- obter `APPROVED`;
- com 0 gaps;
- 0 lacunas;
- 0 falsos positivos;
- 100% de aderencia a `.specs/prd-mecontrola-agent`;
- 100% dos criterios de aceite, 100% do DoD, 100% das regras de negocio e 100% dos requisitos funcionais comprovados por evidencia auditavel.

Contrato de execucao:
1. Carregue obrigatoriamente `AGENTS.md` antes de qualquer analise e confirme o contrato base do repositorio.
2. Execute `@.claude/skills/review/` sem suavizar regras, sem inferir conformidade por proximidade e sem tratar `Status: done`, `tasks.md`, execution report ou artefato de evidence como prova suficiente por si so.
3. Valide estritamente contra o conjunto documental local:
   - `.specs/prd-mecontrola-agent/prd.md`
   - `.specs/prd-mecontrola-agent/techspec.md`
   - `.specs/prd-mecontrola-agent/tasks.md`
   - `.specs/prd-mecontrola-agent/task-*.md`
   - `.specs/prd-mecontrola-agent/*_execution_report.md`
   - `.specs/prd-mecontrola-agent/adr-*.md`
4. Para wiring, bootstrap e verificacao de integracao real, parta obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`. Nao use `internal/platform/runtime` como ponto de partida da analise.
5. Considere como escopo minimo obrigatorio de conformidade:
   - todos os objetivos do PRD;
   - todas as metricas de sucesso do produto;
   - todos os RFs do PRD;
   - todas as restricoes tecnicas e fora de escopo;
   - todas as decisoes resolvidas do PRD;
   - todas as decisoes, invariantes, contratos e riscos aceitos da `techspec.md`;
   - todos os criterios de sucesso de cada `task-*.md`;
   - todos os criterios de aceite e DoD de cada `*_execution_report.md`.
6. Confronte explicitamente, no minimo, os seguintes checkpoints materiais da especificacao:
   - substituicao integral do weather por `MeControlaAgent`, sem convivencia, sem `WeatherClient`, sem wiring, scorer, workflow, tool ou config orfa;
   - onboarding financeiro obrigatorio de 8 etapas, duravel, sem pular etapa, sem reiniciar por duvida e com retomada exata apos pausa;
   - ETAPA 6 coleta as 5 categorias em mensagem unica e a distribuicao fecha exatamente 100%;
   - ETAPA 4 coleta apenas apelido + vencimento do cartao e aplica defaults obrigatorios do dominio;
   - recorrencia do planejamento pergunta explicitamente ao usuario e replica 12 meses quando aceita;
   - operacao diaria registra receita, despesa e compra no cartao por linguagem natural, com data default em `America/Sao_Paulo` quando ausente;
   - quando faltar apenas o meio de pagamento, o agente pergunta somente isso;
   - multiplos lancamentos por mensagem geram escritas sequenciais idempotentes e resposta consolidada;
   - compras parceladas respeitam 1..24 parcelas e impacto automatico nas competencias futuras;
   - lancamentos sao escritos exclusivamente via `internal/transactions`; o agente nao chama escrita de despesa em `internal/budgets`;
   - ajuste de alocacao rebalanceia para fechar 100%;
   - edicao/remocao exigem HITL com confirmacao explicita, aviso de impacto, resume antes do parse e limpeza deterministica;
   - objetivo principal persiste em working memory e historico recente suficiente e injetado no prompt;
   - roteamento respeita a ordem `confirmacao destrutiva -> onboarding -> operacao`;
   - `Run` e `Thread` sao auditaveis e as metricas novas nao usam labels de alta cardinalidade;
   - `OpenRouter` e o unico provider LLM, com modelo `openai/gpt-4o-mini`, `RUN_REAL_LLM` como gate relevante e `WithMaxToolRounds(12)` no agente alvo;
   - `TransactionsModule` expoe os use cases adicionais previstos para os bindings consumer-side;
   - existe ledger agent-owned (`agents_write_ledger`) com unique `(wamid, item_seq, operation)` e replay sem dupla mutacao;
   - o caminho ponta a ponta valida `dispatcher -> consumer -> runtime -> tool/workflow -> gateway`, e nao apenas testes isolados;
   - `internal/onboarding` permanece preservado como ativacao de conta separada do onboarding financeiro do agente.
7. A revisao deve confrontar tambem, de forma agrupada por tarefa, os marcos obrigatorios das tasks:
   - 1.0: `WithMaxToolRounds` preserva default global e `TransactionsModule` exposto sem vazamento de dominio no substrato;
   - 2.0: interfaces consumer-side + adapters finos em `internal/agents`, sem SQL direto e sem expor `UpsertExpense`;
   - 3.0: ledger de idempotencia, migration, repositorio, helper `IdempotentWrite` e retencao coerente;
   - 4.0: tools `register_expense`, `register_income`, `register_card_purchase`, `query_month`, `query_plan`, `edit_entry`, `delete_entry`, `adjust_allocation`, `classify_category`;
   - 5.0: `ConfirmState`, `AwaitingKind`, `OperationKind`, snapshot como fonte unica e confirmacao antes de qualquer acao destrutiva;
   - 6.0: workflow duravel de onboarding com 8 fases fechadas, `StructuredContract` estrito e reuso de estado pre-existente;
   - 7.0: `BuildMeControlaAgent`, system prompt, scorers, memoria/historico e registro do agente unico;
   - 8.0: `module.go` + wiring em `cmd/server` e `cmd/worker`, ordem `categories -> card -> budgets -> transactions -> agents`;
   - 9.0: cutover final sem qualquer resquicio de weather e com evidencias E2E/gates verdes.
8. Se a implementacao divergir da documentacao:
   - use o working tree atual como verdade tecnica;
   - classifique o drift contra a especificacao de forma explicita;
   - nao invente comportamento ausente para fechar lacuna.
9. Execute a revisao contra o diff apropriado:
   - se `AI_REVIEW_PRIOR_SHA` existir, revise apenas o delta da remediacao;
   - caso contrario, use a base apropriada da branch atual, preferencialmente `git diff --merge-base origin/main`.
10. Dispare subagentes especializados quando aumentarem sinal e reduzirem falso positivo, por exemplo:
   - `code-review` para diff relevante, regressao e cobertura de comportamento;
   - `security-review` para superfices de inbound, idempotencia, links, tokens, persistencia e IO externo;
   - `research` ou `explore` para confrontar PRD, techspec, ADRs, tasks, execution reports, wiring real e gates;
   - `rubber-duck` para desafiar conclusoes borderline antes de emitir finding bloqueante.
11. Cada finding deve ter evidencia direta e auditavel:
   - severidade canonica;
   - arquivo e linha quando aplicavel;
   - item exato violado (`RF`, task, criterio de sucesso, criterio de aceite, DoD, ADR, restricao, invariante ou metrica);
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
  - RF-01 a RF-39;
  - objetivos e metricas de sucesso do PRD;
  - criterios de sucesso das tasks 1.0 a 9.0;
  - criterios de aceite dos execution reports existentes;
  - DoD dos execution reports existentes;
  - ADR-001 a ADR-008;
  - restricoes tecnicas, fora de escopo e invariantes de arquitetura relevantes.
- `next_action`

Gate final obrigatorio antes de declarar `APPROVED`:
- todos os RFs aplicaveis marcados como atendidos com evidencia;
- todos os criterios de sucesso das tasks atendidos com evidencia;
- todos os criterios de aceite atendidos com evidencia;
- DoD 100% atendido com evidencia;
- nenhuma lacuna aberta;
- nenhum falso positivo mantido;
- nenhum risco residual que contradiga a especificacao;
- nenhuma regra de negocio obrigatoria sem implementacao comprovada;
- nenhuma referencia relevante ao weather restante em producao;
- conformidade total com `.specs/prd-mecontrola-agent`.
```

## Justificativas do enriquecimento

1. **Escopo fechado e verificavel:** o prompt passou a enumerar explicitamente `prd.md`, `techspec.md`, `tasks.md`, `task-*.md`, `*_execution_report.md` e ADRs para evitar review parcial.
2. **Conflito removido sem mudar a intencao:** a instrucao "Nao implemente nada" foi tratada como restricao desta solicitacao de enriquecimento, e nao como restricao da execucao futura do ciclo `review -> bugfix -> review`.
3. **Cobertura material da especificacao:** os checkpoints agora refletem o conteudo real de `prd-mecontrola-agent`, incluindo onboarding duravel, tools de operacao diaria, ledger, HITL, working memory, scorers, wiring e cutover do weather.
4. **Reducao de falso positivo e falso negativo:** o texto proibe conformidade por inferencia, veta finding especulativo e obriga classificar drift, risco residual ou `BLOCKED` quando a evidencia primaria nao fechar.
5. **Ancoragem no wiring real:** foi reforcado que a revisao precisa partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, essencial para provar bootstrap, DI, consumers, runtime e cutover.
6. **Ciclo operacional deterministico:** o fluxo `review -> bugfix -> review` virou contrato explicito, com revisao do delta de remediacao quando `AI_REVIEW_PRIOR_SHA` estiver disponivel.
7. **Subagentes com criterio:** o prompt nao pede subagentes genericamente; ele orienta quando `code-review`, `security-review`, `research`/`explore` e `rubber-duck` agregam sinal real.

## Variante recomendada

Usar exatamente o prompt enriquecido acima, porque ele combina cobertura documental completa, checkpoints especificos do `MeControlaAgent`, controle de falso positivo e ciclo de remediacao ate `APPROVED`.

## Variante mais conservadora

Manter o mesmo prompt, mas exigir tres passagens explicitas por rodada:

1. matriz documental contra PRD, techspec, ADRs, tasks e execution reports;
2. confrontacao tecnica contra diff, wiring real, runtime, workflow, bindings e evidencias de validacao;
3. checagem final exclusiva de escopo do produto para garantir substituicao total do weather, onboarding obrigatorio de 8 etapas e aderencia integral aos RFs 01..39.

Essa variante aumenta custo e tempo, mas reduz ainda mais o risco de lacunas em fluxos cross-module e em requisitos que misturam agente, dominio, workflow, LLM, observabilidade e cutover.
