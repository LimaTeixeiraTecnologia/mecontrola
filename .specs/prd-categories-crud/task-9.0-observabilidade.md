# Tarefa 9.0: Observabilidade: métricas custom, logs e traces

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Instrumentar handlers e use cases com métricas custom (counters e histogramas via `devkit-go`), logs estruturados e spans OpenTelemetry. Garantir que nenhum termo bruto de busca seja logado ou usado como label de métrica.

<requirements>
- RF-41: métricas com labels `endpoint`, `kind`, `outcome`, `q_len_bucket`, `signal_type_top`
- RF-42: nunca logar consulta bruta, normalizada, hash ou substring do termo
- RF-43: counters distintos por outcome agregados em `q_len_bucket`
- ADR-002: version em respostas
</requirements>

## Subtarefas

- [ ] 9.1 Adicionar métricas de latência e outcome em cada handler
- [ ] 9.2 Adicionar métricas de resultado de busca (`matched`, `ambiguous`, `no_match`, `invalid_query`, `invalid_kind`)
- [ ] 9.3 Implementar `q_len_bucket`: `3-4`, `5-8`, `9-16`, `17-32`, `33+`
- [ ] 9.4 Adicionar traces em handlers, use cases e repositórios
- [ ] 9.5 Validar que termo bruto nunca aparece em logs, traces ou labels

## Detalhes de Implementação

Ver techspec.md seção **Monitoramento e Observabilidade**.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar `references/observability.md`
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- Métricas via `o11y.Metrics().Counter(...)` e `o11y.Metrics().Histogram(...)` (devkit-go).
- Labels permitidas: `endpoint`, `kind`, `outcome`, `q_len_bucket`, `signal_type_top`.
- `outcome`: `matched`, `ambiguous`, `no_match`, `invalid_query`, `invalid_kind`.
- `q_len_bucket`: calcular a partir do `len(q)` após trim (não da normalização).
- `signal_type_top`: apenas quando houver candidato vencedor; vazio caso contrário.
- Nunca usar termo bruto, normalizado, hash ou substring como label.
- Logs: `INFO` com `endpoint`, `method`, `outcome`, `duration_ms`; `ERROR` apenas em falhas de infraestrutura.
- Traces: span por handler (`categories.handler.list`), use case (`categories.usecase.search`), repositório (`categories.repo.query`).

## Critérios de Sucesso

- [ ] Métricas expostas em `/metrics` (via HTTP server existente)
- [ ] Counter `category_dictionary_search_total` incrementa corretamente por outcome
- [ ] Nenhum log contém termo bruto de busca
- [ ] Spans criados em todas as fronteiras de IO
- [ ] Gate R0-R7 passa

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Teste que métricas são incrementadas em cenários de sucesso e erro
- [ ] Teste que `q_len_bucket` está correto para tamanhos de query variados
- [ ] Teste que termo bruto não aparece em logs (verificar com `slog.Handler` custom em teste)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- Handlers e use cases modificados em `internal/categories/`
- Testes de métricas em `_test.go` correspondentes
