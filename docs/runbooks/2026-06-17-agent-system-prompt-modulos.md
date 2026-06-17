# System prompt do `internal/agent` expert em todos os módulos

## Contexto

O módulo `internal/agent` interpreta mensagens de WhatsApp/Telegram via LLM (OpenRouter) e roteia
para os use cases dos módulos. Existiam dois prompts voltados ao usuário, ambos "magros": só
citavam genericamente "categorias, cartões, orçamento e lançamentos", sem ensinar ao modelo a
responsabilidade de cada módulo nem o que ele pode ou não fazer — resultando em conversa travada e
recusas demais ("parecendo chatbot ruim").

Objetivo: tornar o system prompt expert nos módulos — fronteira de cada capability, redirecionamento
fluido para o que não executa (assinatura/cancelamento, ativação) e conversa natural, sem inventar
funcionalidade ou número.

Decisões confirmadas com o usuário:
- Enriquecer ambos os prompts (persona conversacional + intent JSON), com capacidades/fronteiras consistentes.
- Consciência + redirecionar para módulos passivos (billing, onboarding, identity).
- Sem JourneyHint: escopo só o conteúdo dos prompts; nenhum wiring/loader novo.

## Skill obrigatória

`prompt_builder.go` é Go de produção → skill `go-implementation` obrigatória. Zero comentários em
`.go` (R-ADAPTER-001.1) preservado; alteração apenas no conteúdo do `const systemPromptTemplate`.

## Fonte da verdade (capacidades reais, fiéis ao dispatcher)

`internal/agent/infrastructure/dispatcher/intent_dispatcher.go` + adapters:

| Módulo | Pode fazer (via conversa) | NÃO faz (fronteira) |
|--------|---------------------------|---------------------|
| categories | listar, obter, buscar dicionário (catálogo oficial PT-BR) | criar/editar/apagar categoria — só catálogo |
| cards | listar, obter, criar, atualizar, apagar cartão | consultar fatura (não exposto no agente hoje) |
| transactions | listar, obter, criar (lançamento), apagar | editar/atualizar transação (não exposto) |
| budgets | resumo mensal, alertas, criar orçamento/recorrência/despesa, ativar orçamento, apagar rascunho/despesa | — |
| billing | — | assinatura/cobrança/cancelamento → orienta (Kiwify: Minhas Compras > MeControla) |
| onboarding | — | checkout/ativação/magic-link → orienta |
| identity | — | conta/perfil/permissões → nunca altera; opera só com o user_id injetado |

Regra de negócio: gasto do dia a dia entra sempre como lançamento (`transactions.create`); o
orçamento se atualiza sozinho. Nunca instruir o usuário a editar o orçamento para refletir um gasto.

## Arquivos alterados

1. `internal/agent/application/prompting/persona.system.tmpl` — voz conversacional (V5): seção
   "o que faço / o que não faço" em linguagem natural, redirecionamento fluido p/ módulos passivos,
   recusa-redirect em vez de recusa seca, bloco `{{- if .JourneyHint }}` preservado. Mantidos os
   tokens exigidos pelo teste: MeControla, cartões, orçamento, lançamentos, Conexão, Atualização
   Automática, Segurança, SQL, Kiwify.
2. `internal/agent/application/services/prompt_builder.go` — `const systemPromptTemplate`: seção
   "RESPONSABILIDADE E FRONTEIRA DE CADA MÓDULO", desambiguação gasto→lançamento, redirect
   estruturado p/ passivos. Caveats: 6 `%s` na ordem (user_id, channel, permissions, categories,
   cards, current_date); nenhum `%` literal; padding/glossário intactos; zero comentários.
3. Testes: `internal/agent/application/prompting/persona_test.go` (asserts mantidos) e novos asserts
   de conteúdo conforme necessário.

## Verificação

1. `go build ./...`
2. `go test ./internal/agent/... -count=1`
3. Gate de comentários (vazio):
   `grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" "^[[:space:]]*//" internal/agent | grep -Ev "(//go:|//nolint:|// Code generated)"`
4. Sanidade do `Sprintf`: `BuildSystemPrompt` sem `%!s(MISSING)`/`%!(EXTRA...)`.
5. Smoke conversacional com `AGENT_MODE=openrouter` quando houver `OPENROUTER_API_KEY`.

## Addendum — correção de falso positivo (persistência)

Auditoria ponta a ponta (`adapter → usecase → repository → DB`) encontrou um falso positivo real:

- **Sintoma:** com `AGENT_MODE=openrouter` e `TRANSACTIONS_ENABLED=false` (default, ausente no `.env`),
  `ports.Transactions` ficava `nil` e o dispatcher respondia *"Ainda nao consigo executar X, mas anotei
  seu pedido."* com `WasApplied:false` — dizia que registrou sem registrar nada.

Correções aplicadas:
1. `internal/agent/infrastructure/dispatcher/intent_dispatcher.go` — mensagem honesta: "nao registrei
   nada", sem "anotei", sem expor nome técnico do módulo.
2. `internal/agent/module.go` — fail-fast: em `openrouter`, transactions e budgets passam a ser
   obrigatórios (antes só categories/cards). App não sobe mentindo.
3. `.env` — `TRANSACTIONS_ENABLED=true` para a persistência real do lançamento funcionar.

Persistência auditada como real (UoW + SQL) em cards, categories, transactions e budgets. Claim
"orçamento atualiza automaticamente ao registrar gasto" é WIRED via outbox → `transaction_created_consumer`
→ `UpsertExpense`. Gap residual conhecido: deletar transação NÃO remove automaticamente a expense
espelhada em budgets (assimetria a tratar em trabalho futuro; a persona não promete isso).

## Endurecimento do prompt (após matriz de reconhecimento)
A matriz completa contra o LLM real expôs 2 problemas, ambos corrigidos no `systemPromptTemplate`:
1. Falso positivo: o modelo emitia `categories.create` (capability inexistente) para "cria nova
   categoria". Fronteira reforçada: categories suporta APENAS list/get; criar/editar/apagar categoria
   retorna `out_of_scope`.
2. Sub-reconhecimento: "repetir/replicar orçamento" não mapeava para `operation:recurrence`.
   Adicionado vocabulário natural (repetir/replicar/copiar) na linha de recorrência.

## Evidências de validação real
- OpenRouter (chave do `.env`, modelo `google/gemini-2.5-flash-lite`): matriz de 30 frases cobrindo
  TODAS as ações dos 4 módulos + segurança + out-of-scope. Resultado final: **30/30 (100%)**,
  **0 falsos positivos perigosos**, **0 misroutes**, 0 sub-refúsas.
  - categories: list, get, busca; cards: list/get/create/update/delete; transactions:
    create(expense/income)/list/delete; budgets: summary/alerts/create/recurrence/activate/delete.
  - Segurança: prompt injection → `unauthorized`. Fora de escopo: cancelamento→Kiwify, fatura,
    ativação, off-topic, criar categoria → todos `out_of_scope`.
- Postgres real (testcontainers): `transaction_repository` integration test verde (INSERT/SELECT/SoftDelete).
- `go build ./...`, `go test ./internal/agent/...`, `go vet ./internal/agent/...` verdes; gate zero-comentários vazio.
