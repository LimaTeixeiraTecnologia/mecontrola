# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Busca de candidatos de edição por valor e/ou descrição
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Solicitante do produto, engenharia de plataforma
- **Relacionados:** PRD `prd.md` (RF-15); techspec `techspec.md`

## Contexto

RF-15 exige localizar o lançamento a corrigir a partir de falas como "era 25" (valor, sem descrição) ou "corrige aquele mercado" (descrição). Hoje só existe `SearchByDescription` (`internal/transactions/.../transaction_repository.go`), que filtra por `description ILIKE`, `ref_month` opcional e recência, com teto de 10 linhas; não há query por valor, por categoria, nem "últimos N" sem termo de descrição. Sem uma busca adequada, o fluxo de edição não consegue apresentar candidatos por valor.

## Decisão

Adicionar uma **query combinada de candidatos de edição**: novo método de repositório `SearchEditCandidates(userID, amountCents, term, refMonth, limit)` que casa por valor exato OU termo de descrição, restrito ao mês vigente, ordenado por recência (`created_at DESC`), limitado a um top-N pequeno (ex.: 5). Exposto por um usecase e por uma porta do agente (`TransactionsLedger.SearchEditCandidates` + `EditCandidateQuery`). Havendo mais de um candidato, o fluxo de edição lista as opções; havendo um, apresenta o registro.

## Alternativas Consideradas

- **Reusar só `SearchByDescription`**: menos código; rejeitada por não localizar candidatos quando o usuário fornece apenas o valor ("era 25").
- **Só último lançamento / recência pura**: simples; rejeitada por imprecisão quando o alvo não é o mais recente.

## Consequências

### Benefícios Esperados

- Cobre os dois modos do documento (valor e descrição) de forma determinística e testável.
- Top-N pequeno mantém a lista de escolha curta e legível.

### Trade-offs e Custos

- Novo método de repositório + usecase + porta; um índice adequado por `(user_id, ref_month, created_at)` é desejável para performance.

### Riscos e Mitigações

- Risco: valor coincidente em vários lançamentos. Mitigação: apresentação de lista de escolha (top-N) com data e descrição para desambiguar.
- Risco: alvo fora do mês vigente ("ontem" no vira-mês). Mitigação: recência dentro do mês vigente cobre o caso comum; fora disso, o usuário refina com descrição.

## Plano de Implementação

1. Implementar `SearchEditCandidates` no repositório (SQL com `amount_cents = $` OU `description ILIKE`).
2. Criar o usecase e expor `TransactionsLedger.SearchEditCandidates` + binding.
3. Integrar ao fluxo `transaction-write` (operação de edição) e à tool de edição.
4. Testes unitários (valor, descrição, ambos vazios) e integração.

## Monitoramento e Validação

- Testes de repositório e usecase; golden do fluxo de edição por valor e por descrição.
- Sucesso: "era 25" e "aquele mercado" localizam e apresentam candidatos corretamente.

## Impacto em Documentação e Operação

- Documentar a nova query e recomendar o índice de suporte.

## Revisão Futura

- Revisitar para incluir valor aproximado (faixa) se a busca exata mostrar-se insuficiente na prática.
