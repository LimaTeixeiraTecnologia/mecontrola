# Registro de Decisao Arquitetural (ADR)

## Metadados

- **Titulo:** Gate unico de categoria antes da persistencia
- **Data:** 2026-07-06
- **Status:** Aceita
- **Decisores:** Engenharia
- **Relacionados:** `prd.md`, `techspec.md`

## Contexto

O PRD exige 0 falso positivo em escrita de transacoes categorizadas. Hoje `categories` calcula evidencias ricas, mas `agents` e `transactions` consomem subconjuntos diferentes e podem aceitar o primeiro candidato em alguns fluxos.

## Decisao

Criar um gate unico de escrita consumido por `transactions` e acionado por fluxos agentivos e nao agentivos. O gate usa `categories` como autoridade canonica e retorna somente `accepted` com evidencia ou bloqueio tipado. `agents` pode classificar e explicar, mas nao autoriza persistencia sozinho.

## Alternativas Consideradas

- Manter validacoes separadas em `agents` e `transactions`: menor alteracao inicial, mas duplica regra e preserva risco de drift.
- Validar apenas no handler HTTP/tool: simples de localizar, mas viola adapter fino e deixa outros writes sem protecao.
- Deixar `categories` persistir transacoes: centraliza demais e quebra bounded contexts.

## Consequencias

### Beneficios Esperados

- Criterio unico de aceite.
- Menor risco de falso positivo por divergencia de consumidores.
- Diagnostico funcional consistente.

### Trade-offs e Custos

- Mais contrato interno entre bounded contexts.
- Alteracao transversal em agents, transactions e categories.

### Riscos e Mitigacoes

- Risco: acoplamento excessivo a tipos de `categories`.
  Mitigacao: interfaces declaradas pelos consumidores e DTOs internos estaveis.

## Plano de Implementacao

1. Modelar DTOs e status fechados do gate.
2. Implementar adapter que chama `categories`.
3. Injetar gate nos use cases de transacao e recorrencia.
4. Atualizar fluxos agentivos para chamar o mesmo contrato.

## Monitoramento e Validacao

Validar por metricas `category_write_gate_total` e testes de integracao cobrindo aceite, ambiguidade, no match, deprecated e kind incompativel.

## Impacto em Documentacao e Operacao

Atualizar docs de contrato interno entre `categories`, `agents` e `transactions`.

## Revisao Futura

Revisar apos primeira rodada de cenarios reais de clarificacao e taxa de bloqueio.
