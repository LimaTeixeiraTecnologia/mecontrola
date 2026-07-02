# ADR-001 — Banco emissor como texto normalizado no cartão, sem FK; `banks` como lookup de derivação

## Metadados

- **Título:** Banco como texto livre normalizado (sem FK) + tabela `banks` de derivação com fallback 7
- **Data:** 2026-07-01
- **Status:** Aceita
- **Decisores:** JailtonJunior (owner), time de plataforma
- **Relacionados:** PRD `.specs/prd-simplificacao-card-melhor-dia-compra/prd.md` (RF-03, RF-09, RF-10, RF-20), techspec.md, ADR-003

## Contexto

O PRD (RF-03) fixa que o banco emissor é **texto livre** e que um banco fora da tabela **não é erro**:
recai no fallback de 7 dias (RF-09). A tabela `mecontrola.banks` deve ser administrável sem deploy
(RF-10). Uma modelagem com foreign key `cards.bank_id → banks.id` rejeitaria qualquer banco não
catalogado, violando diretamente RF-03/RF-09. Precisamos armazenar o banco no cartão e, ao mesmo tempo,
permitir bancos desconhecidos com fallback.

## Decisão

O cartão armazena o banco como **texto original** informado (`cards.bank TEXT`, VO `BankCode` com
`String()` = original, `LookupKey()` = normalizado), sem foreign key para `banks` (RF-20: preservar o
texto para exibição). A tabela `mecontrola.banks` (`code` PK textual normalizado, `name`,
`days_before_due`) é um **lookup de derivação**, consultado no adapter `BankDaysReader` por
`bank.LookupKey()` no momento do cadastro/consulta. Quando a chave normalizada não existe em `banks`, o
reader retorna **7 dias** (fallback) — nunca erro. A normalização (`normalizeBank`: trim + lowercase +
remoção de acentos + espaços→`-`) é determinística e única, compartilhada por cadastro, endpoint de
consulta e onboarding (RF-20).

Escopo: `internal/card/domain/valueobjects/bank_code.go`, `application/interfaces/bank_days_reader.go`,
`infrastructure/repositories/postgres/bank_repository.go`, migration `000002` (tabela `banks` + coluna
`cards.bank`).

## Alternativas Consideradas

- **FK `cards.bank_id → banks.id`.** Vantagens: integridade referencial, normalização forte.
  Desvantagens: **rejeita banco desconhecido**, violando RF-03/RF-09; exige CRUD administrativo antes de
  cadastrar cartão de banco novo. Rejeitada por conflito direto com o requisito de fallback.
- **Enum fechado de bancos em código.** Vantagens: simples. Desvantagens: exige deploy para novo banco
  (viola RF-10) e rejeita texto livre (viola RF-03). Rejeitada no PRD.
- **Armazenar `days_before_due` diretamente no cartão (desnormalizado).** Vantagens: dispensa lookup na
  leitura. Desvantagens: perde a origem (banco) e a administrabilidade central da tabela; recomputo de
  `closing_day` fica sem fonte. Rejeitada — a tabela `banks` é a fonte administrável (RF-10).

## Consequências

### Benefícios Esperados

- Cumpre RF-03/RF-09 (texto livre + fallback) e RF-10 (tabela administrável sem deploy).
- Normalização única elimina divergência entre canais (API, onboarding).
- Sem acoplamento rígido: bancos novos funcionam imediatamente com a regra padrão de 7 dias.

### Trade-offs e Custos

- Sem integridade referencial: `cards.bank` pode conter um código não presente em `banks` (por design).
- A "correção ortográfica" de banco é responsabilidade da normalização, não do banco de dados.

### Riscos e Mitigações

- **Risco:** proliferação de variantes textuais do mesmo banco. **Mitigação:** normalização
  determinística agressiva (acentos/caixa/espaços); testes unitários de `normalizeBank`.
- **Rollback:** reverter migration `000002` (`.down.sql` recria `limit_cents`, remove `bank` e `banks`).

## Plano de Implementação

1. VO `BankCode` + `normalizeBank` com testes.
2. Migration `000002`: tabela `banks` + seed (8 bancos) + `cards.bank`.
3. Porta `BankDaysReader` + adapter Postgres com fallback 7.
4. Integração nos use cases `CreateCard`/`UpdateCard`/`BestPurchaseDay`.

## Monitoramento e Validação

- Métrica opcional com label `bank_known={true,false}` (cardinalidade 2) — nunca `bank` como label.
- Sucesso: cadastro com banco fora do seed aplica 7 dias sem erro; teste de integração cobre o caso.

## Impacto em Documentação e Operação

- OpenAPI (`bank` como campo de entrada). Runbook de card se existir. Seed de `banks` documentado.

## Revisão Futura

- Revisitar se surgir necessidade de CRUD administrativo de `banks` com validação de duplicatas fortes,
  ou se a cardinalidade de variantes textuais se tornar um problema observado.
