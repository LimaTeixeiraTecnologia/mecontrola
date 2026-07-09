# Tarefa 8.0: Suíte E2E real-LLM + integração dos cenários Gherkin

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Validar ponta a ponta, com LLM real (política do repositório) e integração com Postgres, todos os
cenários de aceite do PRD, fechando o gate de prontidão de produção. Depende de 1.0–7.0.

<requirements>
- Cobrir todos os cenários Gherkin do PRD (RF-01..RF-32), incluindo: salário sem clarify; décimo
  terceiro preservado; income nunca em categoria expense; incompatibilidade de kind sem erro
  silencioso; falha de escrita grava erro no Run/span/log; métrica com motivo fechado; trace de erro
  pesquisável; confirmação única (livro no pix); LLM não emite confirmação própria; cancelamento
  descarta; valor BRL único não dispara múltiplos; dois lançamentos reais barrados; falha transitória
  persiste uma vez; replay não duplica; BRL canônico; despesa pergunta forma de pagamento; receita não.
</requirements>

## Subtarefas

- [ ] 8.1 Testes real-LLM (RUN_REAL_LLM=1 com `.env` OPENROUTER_*): salário sem clarify; décimo
  terceiro; confirmação única (livro no pix); LLM sem confirmação própria; número BRL único vs dois
  lançamentos; despesa pede pagamento com exemplos; receita não pede.
- [ ] 8.2 Testes de integração (`//go:build integration`, testcontainers): propagação de erro para
  `platform_runs.error`/`workflow_runs.last_error` + span + log; retry idempotente e ausência de
  duplicação; reclassificação por kind; seed de salário.
- [ ] 8.3 Testes unitários de formatação BRL (milhar, milhão, valores pequenos) e `IsTransient`.

## Detalhes de Implementação

Ver techspec.md "Abordagem de Testes" e as Notas de Validação do PRD. Seguir o padrão canônico
testify/suite whitebox + mockery (R-TESTING-001); real-LLM conforme política do repositório.

## Critérios de Sucesso

- Todos os cenários Gherkin do PRD passam (real-LLM + integração).
- Zero falha silenciosa observada; confirmação única em todos os fluxos; BRL canônico em todos os sites.
- Gates executáveis do `mastra/rules-checklist` verdes (build/vet/test race, kernel puro,
  zero-comentários, cardinalidade, mockery).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — cenários E2E do agente financeiro (inbound WhatsApp, pending, confirmação) no consumidor `internal/agents`.
- `domain-modeling-production` — asserção dos estados fechados e invariantes (kind, pending, idempotência, pagamento).
- `design-patterns-mandatory` — gate `não aplicar padrão` (suíte de testes, sem estrutura nova).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/**/*_test.go`, `internal/agents/e2e/**`
- `internal/categories/**/*_integration_test.go`, `migrations/migrations_integration_test.go`
- `internal/platform/money/money_test.go`
