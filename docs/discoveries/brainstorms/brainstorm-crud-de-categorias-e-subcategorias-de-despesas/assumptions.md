# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | Categorias servem tanto receitas quanto despesas (entrada e saída de dinheiro). | Esclarecimento explícito do usuário na Rodada 1. | Schema precisa de `kind`/`direction` discriminador; seed e relatórios cobrem os 2 lados. | confirmada |
| H2 | Hierarquia é fixa em exatamente 2 níveis (categoria > subcategoria). | Resposta direta P1.4. | Validação de domínio impede subcategoria de subcategoria. | confirmada |
| H3 | A entrega exige soft-delete, reatribuição de despesas e auditoria de mudanças. | Resposta P1.2 ("Tudo acima + auditoria e versionamento"). | Schema precisa de `deleted_at`, `created_by`, `updated_by`, tabela de auditoria ou eventos de outbox. | confirmada |
| H4 | É feature de fundação que destrava orçamento, dashboards e metas. | Resposta P1.3. | Prioridade alta; precisa entregar contrato estável para módulos consumidores. | confirmada |
| H5 | O módulo deve seguir o "Padrão Obrigatório de Módulo" em `AGENTS.md` (DDD, application/domain/infrastructure, outbox.Publisher quando publicar eventos). | CLAUDE.md instrução 10. | Estrutura interna obrigatória; sem atalhos. | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H6 | O seed deve ser global (read-only para todos) com possibilidade de clonar/customizar. | Se cada usuário precisa do seed copiado no signup, abordagem diferente (write-on-signup). | Rodada 2 - escopo de seed. | usuário |
| H7 | Categorias do seed são imutáveis pelo usuário; ele cria as próprias para customizar. | Se usuário precisa editar nomes do seed, modelo "clonar no signup" vira obrigatório. | Rodada 2. | usuário |
| H8 | Audit log deve ser tabela dedicada (ou outbox events) e não versionamento completo do agregado. | Versionamento completo (event sourcing-lite) muda o desenho radicalmente. | Rodada 3 (alternativas). | agente |
| H9 | Operação de deletar categoria com despesas vinculadas deve forçar reatribuição (não cascata silenciosa). | Cascata = perda de histórico contábil. | Rodada 2/4. | usuário |
| H10 | O CRUD será exposto via HTTP REST (rotas `/v1/categories`) — consistente com `internal/billing` e `internal/identity`. | Se for gRPC ou GraphQL, mudam adapters. | Rodada 2. | usuário |

## Restrições Confirmadas
- Stack Go (versão do `go.mod`), monolito modular, módulos sob `internal/<bounded-context>`.
- Padrão obrigatório de módulo conforme `AGENTS.md` (DDD + outbox + observabilidade).
- Sem `init()`, sem `panic` em produção, `context.Context` em fronteiras de IO (R0–R7 da skill `go-implementation`).
- Migrações em `migrations/` (mesma esteira já usada em `internal/billing`).

## Preferências Não Bloqueantes
- Nomes em PT-BR no seed (alinhado com público brasileiro do mecontrola).
- Pesquisa de mercado em apps locais (Mobills, Organizze, Wallet, GuiaBolso) como referência de taxonomia.
- Métodos populares de orçamento (50/30/20, envelope) como inspiração para agrupamento macro.
