# Tarefa 7.0: Refactor do agente para CRUD unificado

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Refatorar o consumidor `internal/agents` para operar sobre o CRUD unificado de
transações. A tool `register_expense` passa a aceitar `payment_method=credit_card`
com `card_id` e `installments`, delegando ao `CreateTransaction` unificado via
binding. A superfície de card-purchase do agente (tools + métodos de binding +
interface) é removida no mesmo PR, sem intervalo com agente quebrado — coerente
com o corte HTTP da superfície `card-purchase` (ADR-002, RF-24).

Cobre **RF-11a** (`credit_card` só em `direction=outcome`), **RF-11b** (`card_id`
obrigatório para `credit_card`) e **RF-24** (remoção da superfície card-purchase).
As tools do agente são adapters finos (R-AGENT-WF-001.2): o `exec` valida o input
contra o schema, mapeia para o command do usecase e delega ao binding → usecase;
sem regra de negócio, SQL ou branching de domínio.

<requirements>
- `register_expense` aceita `paymentMethod=credit_card` + `cardId` + `installments`
  (default 1, intervalo 1..24), roteando pelo `CreateTransaction` unificado via binding.
- Remover as tools `register_card_purchase.go`, `get_card_purchase.go`,
  `list_card_purchases.go` (fundir consulta/listagem no `get_transaction`/`search_transactions`
  existentes quando aplicável; não recriar superfície paralela).
- Em `infrastructure/binding/transactions_ledger_adapter.go`: remover os 5 métodos de
  card-purchase (create/update/delete/get/list) e suas dependências; compra no crédito
  passa a rotear pelo `CreateTransaction` unificado.
- Em `application/interfaces/{transactions_ledger.go, types.go}`: remover
  `RawCardPurchase`, `RawUpdateCardPurchase` e os métodos de card-purchase da interface
  `TransactionsLedger`.
- Tool fina (R-AGENT-WF-001.2 / R-ADAPTER-001.2): sem regra/SQL/branching; exec delega
  ao binding → usecase. Estados fechados (`agent.ToolOutcome`/`agent.RunStatus`), nunca
  string livre. Zero comentários em Go de produção (R-ADAPTER-001.1).
- Sem intervalo com agente quebrado: refactor no mesmo PR do corte HTTP (ADR-002).
- Validação REAL-LLM obrigatória (RUN_REAL_LLM=1 com credenciais do `.env` /
  `OPENROUTER_*`); mocks não bastam como evidência.
</requirements>

## Subtarefas

- [ ] 7.1 Estender `register_expense`: schema de input ganha `cardId` (string, opcional)
  e `installments` (integer 1..24, default 1); enum de `paymentMethod` inclui `credit_card`,
  `vale_refeicao`, `vale_alimentacao`; mapear os campos para o `RegisterExpenseCommand`.
- [ ] 7.2 Rotear a compra no crédito pelo `CreateTransaction` unificado no binding
  `transactions_ledger_adapter.go`; remover os 5 métodos de card-purchase (createCP/updateCP/
  deleteCP/getCP/listCP) e dependências que ficarem órfãs.
- [ ] 7.3 Enxugar a interface `TransactionsLedger` e `types.go`: remover
  `RawCardPurchase`/`RawUpdateCardPurchase` e os métodos `CreateCardPurchase`,
  `UpdateCardPurchase`, `DeleteCardPurchase`, `GetCardPurchase`, `ListCardPurchases`.
- [ ] 7.4 Remover as tools `register_card_purchase.go`, `get_card_purchase.go`,
  `list_card_purchases.go` e desregistrá-las do wiring do agente; regenerar mocks afetados.
- [ ] 7.5 Testes unitários (schema da tool + binding sobre mock) e validação real-LLM.

## Detalhes de Implementação

Ver `techspec.md` — seção "Visão Geral dos Componentes" (bloco "Consumidor Agente")
e "Pontos de Integração" (`internal/agents`), além do "Sequenciamento de Desenvolvimento"
item 7. O corte, o descarte de dados e o refactor do agente no mesmo PR estão em
`adr-002-card-purchase-cutover-drop.md` (Decisão itens 1–3; Plano de Implementação item 3).

Contrato do agente durante a transição (techspec, "Riscos Conhecidos"): mitigado por
refatorar as tools no mesmo PR — sem intervalo quebrado.

Regras aplicáveis: `.claude/rules/agent-workflows-tools.md` (R-AGENT-WF-001, em especial
.1 roteamento `Workflow → Tool → binding → usecase`, .2 tool fina sem regra/SQL/branching,
.3 estados fechados) e `.claude/rules/go-adapters.md` (R-ADAPTER-001.1/.2).
**Não duplicar** validação semântica de enum/parcelas na tool: o schema valida a
superfície; a regra `credit_card ⇒ outcome` e `credit_card ⇒ card_id` vive no smart
constructor do command unificado (RF-11a/11b), não na tool.

## Critérios de Sucesso

- Agente registra compra parcelada no crédito (`credit_card`, `installments > 1`) e
  compra à vista (`installments=1`) via `register_expense` unificado.
- Nenhuma referência a use case de card-purchase no binding
  `transactions_ledger_adapter.go` nem na interface `TransactionsLedger`.
- Tools `register_card_purchase`/`get_card_purchase`/`list_card_purchases` removidas;
  agente compila e responde sem elas.
- Sem intervalo com agente quebrado (mesmo PR do corte HTTP da superfície card-purchase).
- Gates verdes: sem comentários Go em `internal/agents/application/tools/` e no binding
  (R-ADAPTER-001.1); sem SQL direto na tool/binding (R-ADAPTER-001.2); sem estado de
  fronteira como string livre (R-AGENT-WF-001.3).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — construir/alterar tool do agente sobre internal/platform e o consumidor internal/agents (ciclo Thread→Run, tool fina).

## Testes da Tarefa

- [ ] Testes unitários: schema da tool `register_expense` (campos `cardId`/`installments`,
  enum de `paymentMethod` com `credit_card`/`vale_refeicao`/`vale_alimentacao`) e binding
  `transactions_ledger_adapter` sobre mock do usecase (rota credit_card → `CreateTransaction`).
- [ ] Validação real-LLM (RUN_REAL_LLM=1, credenciais `.env`/`OPENROUTER_*`): registrar
  despesa `credit_card` parcelada e à vista pelo agente end-to-end da tool.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Integração e e2e (jornada HTTP completa + 404 nas rotas removidas) pertencem à Tarefa 8.0.

## Arquivos Relevantes

- `internal/agents/application/tools/register_expense.go` — estender schema/exec para `credit_card`.
- `internal/agents/application/tools/register_card_purchase.go` — remover.
- `internal/agents/application/tools/get_card_purchase.go` — remover.
- `internal/agents/application/tools/list_card_purchases.go` — remover.
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go` — remover 5 métodos de card-purchase; rotear crédito pelo `CreateTransaction` unificado.
- `internal/agents/application/interfaces/transactions_ledger.go` — enxugar interface `TransactionsLedger`.
- `internal/agents/application/interfaces/types.go` — remover `RawCardPurchase`/`RawUpdateCardPurchase`.
- `.specs/prd-transactions-crud-unificado/techspec.md` — Consumidor Agente, Pontos de Integração, Sequenciamento item 7.
- `.claude/rules/agent-workflows-tools.md` — R-AGENT-WF-001 (.1/.2/.3).
