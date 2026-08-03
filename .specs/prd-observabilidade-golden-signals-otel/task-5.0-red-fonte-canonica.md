# Tarefa 5.0: Fonte canônica de RED (série do histograma)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Migrar o alerta `mc-api-5xx` em `rules.yaml` para usar a série canônica do histograma `http_server_request_duration_seconds_count` (taxa de erro filtrada por `http.response.status_code=~"5.."`) em vez do counter auxiliar `http_server_request_count_total`. Documentar que os counters devkit (`http_server_request_count`/`_active`) permanecem como sinais AUXILIARES (liveness/in-flight), não como fonte primária de RED. Cobre RF-08; segue ADR-004. Depende da Tarefa 1.0.

<requirements>
- Migrar o alerta `mc-api-5xx` para a série canônica `http_server_request_duration_seconds_count`, filtrando erros por `http.response.status_code=~"5.."`.
- Confirmar a série Prometheus renderizada (nome e labels) no ambiente antes de fixar a query (RF-14).
- Manter os counters devkit (`http_server_request_count`/`_active`) disponíveis e documentados como AUXILIARES (liveness `absent(...)`, in-flight), sem removê-los.
- Documentar em `observability/STANDARD.md` a fonte canônica de RED e o papel dos auxiliares (nota).
- Não regredir alertas válidos existentes (RF-20); coexistir com os thresholds já presentes.
</requirements>

## Subtarefas

- [ ] 5.1 Confirmar no ambiente as séries Prometheus `http_server_request_duration_seconds_count` e seus labels (`http.request.method`/`http.route`/`http.response.status_code`).
- [ ] 5.2 Reescrever a query do alerta `mc-api-5xx` em `rules.yaml` para derivar a taxa de erro do `_count` do histograma filtrado por `status_code=~"5.."`.
- [ ] 5.3 Adicionar nota em `observability/STANDARD.md` fixando a série do histograma como fonte canônica de RED e os counters devkit como auxiliares (liveness/in-flight).
- [ ] 5.4 Validar as duas séries em paralelo no período de transição (coerência numérica entre o counter auxiliar e o `_count` do histograma).

## Detalhes de Implementação

Ver techspec.md, seção "Sequenciamento de Desenvolvimento → Ordem de Build" (item 4, fonte canônica de RED depende de item 3 para consistência de queries; item 1 = Tarefa 1.0) e "Considerações Técnicas → Decisões Chave" (ADR-004). O contrato completo está em `adr-004-red-canonical-source.md` (histograma canônico `http.server.request.duration` com buckets `[0.005…10.0]` e atributos semconv; counters auxiliares não-canônicos rebaixados; migração do `mc-api-5xx`; validação em paralelo antes do corte; não alterar o provider devkit — fora de escopo).

## Critérios de Sucesso

- O alerta `mc-api-5xx` deriva a taxa de erro de `http_server_request_duration_seconds_count` filtrado por `status_code=~"5.."`.
- A nova query PromQL avalia sem erro (`execErrState` limpo) e é coerente com o counter auxiliar no período de transição.
- Os counters devkit permanecem disponíveis e documentados como auxiliares em `observability/STANDARD.md`.
- Nenhum alerta válido existente removido ou renomeado indevidamente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `golden-signals-otel-standards` — padrão canônico de mapeamento dos Golden Signals para métricas semconv e queries PromQL.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

Validar que a nova query PromQL do `mc-api-5xx` avalia sem erro e é coerente com o counter auxiliar no período de transição (comparação das duas séries em paralelo).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` — alerta `mc-api-5xx` migrado.
- `observability/STANDARD.md` — nota sobre fonte canônica de RED e counters auxiliares.
