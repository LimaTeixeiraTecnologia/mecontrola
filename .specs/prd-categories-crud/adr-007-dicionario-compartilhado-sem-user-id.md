# ADR-007: Dicionário de categorias é compartilhado e NÃO carrega user_id por design

## Metadados

- **Título:** Tabelas de dicionário de categorias sem coluna `user_id` por design
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola (Jailton Junior)
- **Relacionados:** PRD `prd-categories-crud`; análise crítica de isolamento per-user em `~/.claude/plans/analise-de-forma-criteriosa-shiny-book.md` seção 3; revisão adversarial 2026-06-12; bugfix `.specs/prd-gateway-auth-forensics/bugfix_report.md` (corrigiu budgets — categories foi confirmado conforme)

## Contexto

A auditoria de isolamento per-user realizada em 2026-06-12 (precursora do go-live) inspecionou todos os módulos `internal/budgets`, `internal/card`, `internal/categories` e `internal/transactions` procurando queries SQL que pudessem permitir vazamento cross-tenant. O baseline canônico é `internal/transactions`: toda mutação inclui `WHERE id = $X AND user_id = $Y` para defesa em profundidade.

Em `internal/categories`, as tabelas relacionais NÃO carregam `user_id`:

```
migrations/000004_categories_baseline.up.sql
  ├── CREATE TABLE mecontrola.category_editorial_version   -- sem user_id
  ├── CREATE TABLE mecontrola.categories                    -- sem user_id
  └── CREATE TABLE mecontrola.category_dictionary           -- sem user_id
```

Auditorias mecânicas (gate `task lint:user-isolation` — `deployment/scripts/lint-repo-user-id.sh`) hoje EXCLUEM `internal/categories` justamente porque o repositório dele tem queries SELECT sem `WHERE user_id`, o que dispararia falso positivo em qualquer varredura ingênua.

Sem ADR explícito, futuras auditorias (humanas ou automatizadas, internas ou de compliance) podem interpretar a ausência de `user_id` como gap de segurança e abrir bug crítico equivocado, levando a refatorações invasivas que quebram a semântica do dicionário.

## Decisão

Tabelas `mecontrola.categories`, `mecontrola.category_dictionary` e `mecontrola.category_editorial_version` são **deliberadamente compartilhadas** entre todos os usuários do mecontrola e NÃO devem conter coluna `user_id`. As queries do módulo `internal/categories` não devem incluir filtro `WHERE user_id = ...`.

Esta decisão se aplica a:
- Schema das tabelas (sem ALTER TABLE adicionando `user_id`).
- Queries em `internal/categories/infrastructure/repositories/postgres/*.go` (sem filtro por usuário).
- Handlers e use cases em `internal/categories/application/usecases/*.go` (sem consumir `auth.Principal` para filtrar dados; consumir apenas para ADR-001 `RequireUser` autenticando, **não** autorizando por dono).
- Gate `task lint:user-isolation` (continua excluindo `internal/categories` do conjunto de módulos per-user).

A decisão **não** invalida ADR-001 (autenticação via `RequireUser`): categorias continuam exigindo usuário autenticado para acesso, mas o **dado retornado é o mesmo independentemente do usuário** — não há autorização por dono porque não há dono.

## Alternativas Consideradas

| Alternativa | Vantagens | Desvantagens | Motivo de rejeição |
|---|---|---|---|
| Adicionar `user_id` a todas as tabelas (per-user copy) | Padrão único com transactions/budgets/card; gate de lint cobre tudo | Quebra o conceito de dicionário curado; explosão de dados (15k aliases × 1k users = 15M linhas redundantes); cada usuário precisaria de seed das mesmas categorias canônicas; conflito ao atualizar o dicionário | Dicionário é **dado de referência editorial**, não dado de usuário. Per-user copy contradiz a natureza do recurso. |
| Permitir `user_id NULL` com fallback a categorias globais | Permite customização futura per-user mantendo curadoria global | Complexidade no parser/resolver; risco de cross-tenant accidental se query esquecer `OR user_id IS NULL`; sem demanda real no MVP | Over-engineering para hipótese sem evidência; reabrir quando houver feature "categorias customizadas pelo usuário". |
| Mover dicionário para configuração estática (YAML/embed) | Zero risco de cross-tenant; versionamento via git | Perde a capacidade de evoluir dicionário sem deploy; perde índices SQL para busca full-text; `task migrate` deixa de cobrir mudanças de catalogo | Categorias evoluem com base em uso (ADR-002 `category_editorial_version`); deploy a cada update é fricção desnecessária. |
| Adicionar `user_id` somente em uma tabela intermediária "user_category_preferences" | Permite preferências sem mexer no dicionário base | Solução para problema que ainda não existe (preferências/customização) | Reabrir quando houver feature concreta; ADR-007 atual cobre o estado MVP. |

## Consequências

### Benefícios Esperados

- **Auditorias futuras não geram falso positivo** sobre `internal/categories` — este ADR é a referência canônica explicando a ausência de `user_id`.
- **Tamanho de dados otimizado**: ~15k aliases × 1 (global) em vez de × N usuários.
- **Curadoria centralizada**: ajuste de canonical/alias propaga para todos os usuários instantaneamente sem migration por usuário.
- **Performance de busca**: índices full-text (GIN) operam sobre o conjunto único, evitando overhead de filtro `user_id` no hot path.
- **Cache trivial**: dicionário pode ser cacheado in-memory por toda a instância (já feito em parte via `slug.go` e `confidence.go`).

### Trade-offs e Custos

- **Sem customização per-user no MVP**: usuário não pode renomear categoria nem criar alias próprio. Aceito; sem demanda no escopo atual.
- **Sem isolamento de privacidade no dicionário**: se um alias revelar tendência editorial (ex: novo nicho de mercado), todos os usuários veem. Aceito; dicionário é considerado conhecimento público de domínio.
- **Risco de confusão em auditoria**: revisor de boa fé pode flagar a ausência de `user_id` como gap. **Mitigado por este ADR** + comentário implícito no gate de lint.

### Riscos e Mitigações

| Risco | Impacto | Mitigação |
|---|---|---|
| Futura feature "categorias customizadas pelo usuário" exige reabrir decisão | Refatoração de schema | ADR explícito facilita a discussão; alternativa "user_category_preferences" já está documentada acima |
| Auditoria de compliance (LGPD) considera dicionário "dado pessoal" | Necessidade de purga por usuário | Dicionário é dado de domínio (categorias de despesa), não dado pessoal; vínculo com usuário ocorre apenas em `budgets_expenses.root_slug` que é purgado pelo módulo budgets |
| Atualização do dicionário (via `category_editorial_version`) gera locks longos | Indisponibilidade momentânea de busca | ADR-002 já mitiga via versionamento e refresh assíncrono |
| Auditoria mecânica futura inclui `internal/categories` por erro | Falso positivo de bypass | Gate `lint-repo-user-id.sh` linhas 11–15 enumera explicitamente os 3 módulos per-user; categories ausente é a regra |

## Plano de Implementação

Sem mudança de código necessária — esta ADR documenta o estado atual.

1. Confirmar que `deployment/scripts/lint-repo-user-id.sh` enumera apenas `budgets`, `card`, `transactions` na variável `targets` (já implementado em 2026-06-12).
2. Adicionar referência a este ADR no README do módulo `internal/categories/` se existir (opcional).
3. Citar este ADR em revisões de PR que questionem ausência de `user_id` em categorias.

## Monitoramento e Validação

- Critério de sucesso: nenhum bug crítico aberto em 2026 (ou subsequente) reportando "categorias sem user_id" como falha de segurança.
- Sinal de revisão necessária: surgir requisito de produto para "categorias customizadas pelo usuário" — neste caso, reabrir ADR e avaliar `user_category_preferences` como tabela complementar (não como ALTER às tabelas atuais).

## Impacto em Documentação e Operação

- ADR-007 (este documento): novo.
- `.claude/rules/governance.md`: nenhuma alteração — governança transversal não muda.
- `deployment/scripts/lint-repo-user-id.sh`: nenhuma alteração — já exclui categories por design.
- Runbooks operacionais: nenhuma alteração.
- README do módulo: opcional, adicionar link para este ADR.

## Revisão Futura

Revisar quando:
- Surgir requisito de produto para customização per-user de dicionário (alias, renomear, criar categoria privada).
- Auditoria de compliance reclassificar dicionário como dado pessoal.
- Cardinalidade do dicionário exceder 100k linhas (re-avaliar índices e cache).
- Data sugerida: 2027-06-12 ou no surgimento de qualquer das condições acima.
