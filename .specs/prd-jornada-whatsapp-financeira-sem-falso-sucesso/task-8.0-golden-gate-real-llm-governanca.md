# Tarefa 8.0: Golden set, gate real-LLM e gates de governança ponta a ponta

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fecha a jornada "sem falso sucesso" com a rede de regressão ponta a ponta: golden set anonimizado/sintético reproduzindo a conversa produtiva real, gate real-LLM `>= 0,90` por categoria da jornada, asserts de integração sobre `platform_messages` e reconciliação de estado, e a execução completa dos gates Go e de governança sobre todo o escopo alterado nas Tarefas 1.0–7.0. Esta tarefa não introduz correção de domínio nova — ela **prova** que as correções das tarefas anteriores eliminaram falso múltiplo lançamento, orçamento padrão indevido, confirmação duplicada e ausência de transação final, e que nada regride.

Cobre RF-08, RF-16, RF-25, RF-26, RF-27, RF-28. **DEPENDE de TODAS as tarefas 1.0–7.0** (domínio puro, workflows, plataforma/observabilidade, consumidores, migration/auditoria, identidade e status).

<requirements>
- RF-25 — Golden set anonimizado/sintético da conversa produtiva (2026-07-09 00:02 a 00:18 UTC): ativação; personalização de orçamento (caso válido `2500/0/500/0/2000` sobre renda `500000` e caso inválido que não fecha 100%); "Gastei 10 na padaria no dinheiro"; "Gastei 19 na padaria no Pix"; "Hoje"; "Sim"; repetição de "Sim". Deve provar: ausência de falso múltiplo lançamento, ausência de orçamento padrão indevido, ausência de confirmação duplicada e presença de transação final. Seguir o harness de golden/real-LLM existente do projeto (`internal/agents`, padrão `RUN_REAL_LLM=1`); reprodução anonimizada/sintética sem telefone, email ou dado pessoal real.
- RF-27 — Gate real-LLM com resultado `>= 0,90` **por categoria** da jornada; OpenRouter como provider único, sem fallback chain nem novo provider.
- RF-08 — Integração: `platform_messages` contém a mensagem inbound e a resposta final da pendência após a confirmação (assert de 2 linhas: inbound + resposta final).
- RF-16 — E2E: teste de invariante de reconciliação retorna 0 violações após a jornada financeira completa (concordância entre `platform_runs.status`, `workflow_runs.status`, `workflow_runs.state.status`, `agents_write_ledger`, `transactions`, `platform_scorer_results`).
- RF-26 — Gates Go no escopo alterado: `go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `golangci-lint run` quando disponível; mais greps de governança R0/R1/R5/R7 e R-ADAPTER-001.1 (zero comentários em Go de produção).
- RF-28 — Adapters finos (R-ADAPTER-001.2: sem SQL direto, regra de negócio ou branching de domínio em handlers/consumers/jobs/producers/tools) + gate de cardinalidade de métricas (sem `user_id`, telefone, email ou texto de mensagem como label).
- Privacidade e restrições: golden anonimizado/sintético; sem novo pattern GoF; substrato Mastra preservado; estados de fronteira permanecem tipos fechados.
</requirements>

## Subtarefas

- [ ] 8.1 Construir o golden set anonimizado/sintético da jornada (RF-25) no harness existente de `internal/agents`, cobrindo ativação → personalização válida → personalização inválida → "Gastei 10 na padaria no dinheiro" → "Gastei 19 na padaria no Pix" → "Hoje" → "Sim" → 2º "Sim"; asserts determinísticos provando ausência de falso múltiplo lançamento, ausência de orçamento padrão indevido, ausência de confirmação duplicada e presença de transação final.
- [ ] 8.2 Cablear o gate real-LLM `>= 0,90` por categoria (RF-27) sobre as categorias da jornada, usando OpenRouter (provider único, sem fallback), no padrão `RUN_REAL_LLM=1` com `.env` (`OPENROUTER_*`); falha o gate se qualquer categoria ficar abaixdo do limiar.
- [ ] 8.3 Teste de integração RF-08: após a confirmação da pendência, asserir que `platform_messages` contém exatamente a linha inbound e a linha de resposta final da pendência (2 linhas).
- [ ] 8.4 Teste e2e RF-16: executar a jornada financeira completa e invocar a query de auditoria de reconciliação (`internal/agents/infrastructure/postgres/audit_reconciliation.go`), assertando 0 violações de invariante; incluir o caso negativo controlado que **detecta** violação.
- [ ] 8.5 Executar os gates Go de RF-26 no escopo alterado (`go build ./...`, `go vet ./...`, `go test ./... -count=1 -race`, `golangci-lint run` quando disponível) e capturar evidência.
- [ ] 8.6 Executar os greps de governança R0/R1/R5/R7 e R-ADAPTER-001.1 (zero comentários) e confirmar retorno limpo.
- [ ] 8.7 Executar os greps de adapters finos RF-28/R-ADAPTER-001.2 (sem SQL direto/regra/branching em handlers/consumers/jobs/producers/tools) e o gate de cardinalidade de métricas (sem `user_id`/telefone/email/texto como label); confirmar retorno limpo.

## Detalhes de Implementação

Ver `techspec.md`, seção **Abordagem de Testes → Testes E2E (golden + real-LLM, RF-25, RF-27)** para o escopo do golden e a obrigatoriedade do gate real-LLM `>= 0,90` por categoria (precedente do projeto: unit determinístico já mascarou defeitos via brittleness). Ver **Abordagem de Testes → Testes de Integração** para os critérios de RF-08 (2 linhas em `platform_messages`) e RF-16 (query de reconciliação com 0 violações e caso negativo detectado).

Ver `techspec.md`, seção **Considerações Técnicas → Conformidade com Padrões** e **Requisitos transversais** (RF-08/RF-26/RF-28) para os gates Go, greps de governança e adapters finos. A query de reconciliação vive no adapter postgres apenas como leitura, invocada por use case (RF-28). Não duplicar a mecânica aqui — referenciar techspec.

Restrições herdadas do PRD/techspec (não reintroduzir): sem novo pattern GoF; substrato Mastra preservado; OpenRouter provider único sem fallback; estados de fronteira como tipos fechados; golden anonimizado/sintético sem dado pessoal real; cardinalidade de métricas restrita a enums fechados.

## Critérios de Sucesso

- Golden set da jornada verde em CI (execução determinística), provando os quatro invariantes de RF-25.
- Gate real-LLM `>= 0,90` por categoria da jornada (OpenRouter, provider único), verde.
- RF-08: `platform_messages` contém exatamente inbound + resposta final da pendência (2 linhas assertadas).
- RF-16: query de reconciliação retorna 0 violações após a jornada completa; caso negativo controlado é detectado.
- Todos os gates Go de RF-26 verdes no escopo alterado (`go build`, `go vet`, `go test -race -count=1`, `golangci-lint run` quando disponível).
- Greps de governança R0/R1/R5/R7 e R-ADAPTER-001.1 (zero comentários) retornam limpo.
- Greps de adapters finos R-ADAPTER-001.2 e gate de cardinalidade de métricas retornam limpo (nenhum label sensível).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — harness de scorers/evals, golden set e gate real-LLM do substrato agentivo.
- `postgresql-production-standards` — asserts de integração sobre `platform_messages`/`transactions`/reconciliação conforme documentação oficial PostgreSQL.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração
- [ ] E2E golden (determinístico) provando os quatro invariantes de RF-25.
- [ ] E2E real-LLM (`RUN_REAL_LLM=1` com `.env` `OPENROUTER_*`) atingindo `>= 0,90` por categoria (RF-27).
- [ ] Integração da jornada completa: RF-08 (2 linhas em `platform_messages`) e RF-16 (reconciliação com 0 violações + caso negativo detectado).
- [ ] Execução dos gates de governança (gates Go RF-26, greps R0/R1/R5/R7 + R-ADAPTER-001.1, greps R-ADAPTER-001.2 e cardinalidade de métricas) como parte da tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents` — harness de golden/real-LLM (padrão `RUN_REAL_LLM=1`) e testes e2e da jornada.
- `internal/agents/application/scorers/` — scorers usados no gate real-LLM por categoria (`write_persistence_accuracy`, `no_hallucination`).
- `internal/agents/infrastructure/postgres/audit_reconciliation.go` — query de reconciliação read-only invocada no teste RF-16.
- `internal/agents/**/*integration_test.go` — testes de integração da jornada (RF-08, RF-16) com `//go:build integration` e testcontainers-go.
- `internal/platform/agent/`, `internal/platform/scorer/` — substrato consumido pelo harness (não reescrever).
- Scripts/gates de governança (R0/R1/R5/R7, R-ADAPTER-001.1/001.2, cardinalidade de métricas) executados sobre o escopo alterado das Tarefas 1.0–7.0.
