# Tarefa 6.0: Harness de carga k6 e prova dos envelopes A e B

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Nenhum teste de carga foi executado em qualquer envelope de 10k; o veredito de B é **não comprovado**. Construir o harness proporcional aos envelopes A (10k/mês) e B (10k/dia) e produzir prova objetiva, convertendo o veredito ou registrando o gap com honestidade.

<requirements>
- RF-17: harness k6 para envelopes A e B (webhook, outbox drain, leitura) com perfis documentados.
- RF-18: critérios de aprovação (p95, error rate, pool, CPU) + relatório de evidência; veredito honesto por envelope.
</requirements>

## Subtarefas

- [ ] 6.1 Estender `scripts/loadtest/` com perfis proporcionais aos envelopes A e B, parametrizados por VUs/duração.
- [ ] 6.2 Integrar como target de Taskfile e documentar como executar.
- [ ] 6.3 Definir thresholds (p95, `http_req_failed`, `mecontrola_db_pool_in_use`, CPU do host) e critérios de aprovação por envelope.
- [ ] 6.4 Executar contra staging/ambiente equivalente e produzir relatório de evidência em `docs/runs/`, com veredito `comprovado`/gap.

## Detalhes de Implementação

Ver `techspec.md` REQ-06. Executar **após** as Tarefas 4.0 (sampling) e 5.0 (recursos/pool) para medir na configuração-alvo. Não rodar sobre produção com usuários reais.

## Critérios de Sucesso

- Harness roda os dois envelopes com perfis reproduzíveis e parametrizáveis.
- Relatório de evidência publicado com p95/erro/pool/CPU por envelope.
- Veredito de B atualizado para `comprovado` ou gap registrado sem falso positivo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `taskfile-production` — integrar o harness de carga como target de Taskfile reproduzível.
- `otel-grafana-dashboards` — observar p95/pool/CPU durante a carga e ancorar a evidência em painéis.

## Testes da Tarefa

- [ ] Testes unitários (não aplicável; validação por execução do harness)
- [ ] Testes de integração (execução dos envelopes A e B com thresholds avaliados)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `scripts/loadtest/README.md`, `scripts/loadtest/kiwify-webhook.js`, `scripts/loadtest/outbox-throughput.sh`
- `taskfiles/test.yml` (ou novo target de carga)
- `deployment/dashboards/*.json`
- `docs/runs/<data>-evidencia-carga-envelopes.md` (novo)
