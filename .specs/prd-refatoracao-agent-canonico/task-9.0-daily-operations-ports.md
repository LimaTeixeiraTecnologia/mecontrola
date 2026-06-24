# Tarefa 9.0: Operação diária via portas (recorrência, % categoria, consultas, casos especiais)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar a cobertura funcional do Documento Oficial na operação diária, acionando os módulos donos via
porta de entrada: recorrência de orçamento, edição de % de categoria pós-onboarding, consultas
(resumo/como estou/categoria/cartões), compra parcelada, casos especiais da matriz de decisão,
redirecionamento de "desfazer" e formatação de respostas (tom/emoji). Tudo via `buildRegistry` (seam),
tools finas, sem regra de domínio no agent.

<requirements>
- RF-23: mapeamento operação → porta cobre registrar despesa/receita, compra cartão (à vista/parcelada), consultar resumo/orçamento, editar/apagar lançamento, cartão CRUD, configurar orçamento, valores de categoria no onboarding, editar % categoria pós-onboarding, recorrência de orçamento.
- RF-26: registrar movimentações mesmo sem orçamento configurado (continuidade sem orçamento).
- RF-27: compras parceladas criadas com parcelas/competências futuras pela porta do módulo dono.
- RF-28: consultas oficiais (resumo do mês, "como estou", orçamento por categoria, cartões) compondo dados via portas.
- RF-29: recorrência de orçamento ("repetir por N meses") via porta `CreateRecurrence` (limite no dono).
- RF-30: edição de valor/percentual de categoria pós-onboarding via porta `EditCategoryPercentage`.
- RF-33: casos especiais (falta valor/pagamento/categoria, cartão não encontrado, múltiplos resultados, categoria ambígua, falha de integração).
- RF-34: "Desfaz isso" redireciona para apagar/editar com confirmação (sem reversão automática).
- RF-35: respostas seguem tom de voz, emojis oficiais e hierarquia visual do Documento Oficial.
</requirements>

## Subtarefas

- [ ] 9.1 Wire tools/workflows novos no seam `buildRegistry`: recorrência de orçamento (porta `CreateRecurrence`) e edição de % categoria pós-onboarding (porta `EditCategoryPercentage`) — adapters finos.
- [ ] 9.2 Redirecionamento de undo: `KindUndo`/intent equivalente → resposta que oferece apagar/editar último com HITL (RF-34), sem reversão automática.
- [ ] 9.3 Verificar consultas (resumo, como estou, por categoria, cartões) compondo `transactions`/`budgets`/`card`/`categories` via portas (RF-28); compra parcelada via `CreateCardPurchase` (RF-27); continuidade sem orçamento (RF-26).
- [ ] 9.4 Casos especiais (RF-33): garantir clarificação por falta de valor/pagamento/categoria, cartão não encontrado (oferecer cadastro), múltiplos resultados (escolha), categoria ambígua (5 categorias), falha de integração (mensagem clara).
- [ ] 9.5 Formatação de respostas (RF-35): tom/emojis oficiais e hierarquia visual conforme runbook conversacional; bater literal nos exemplos do Documento Oficial.
- [ ] 9.6 e2e WhatsApp-only dos fluxos de operação diária.

## Detalhes de Implementação

Ver techspec §"Mapeamento operação → porta de entrada" e PRD §"Cobertura funcional (MVP)". Portas já
existentes (`CreateRecurrence`, `EditCategoryPercentage`, `GetMonthlySummary`, `CreateCardPurchase`,
etc.) — agent apenas aciona (R-AGENT-WF-001.2, R-ADAPTER-001.2).

## Critérios de Sucesso

- Recorrência e editar % categoria operacionais via porta (sem recálculo no agent).
- Consultas/parcelada/continuidade-sem-orçamento verdes; casos especiais cobertos pela matriz.
- Undo redireciona para apagar/editar com HITL.
- Respostas batem o tom/emoji/hierarquia do Documento Oficial nos cenários de aceite.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — adiciona tools/workflows no seam `buildRegistry` e cobre a operação diária do `internal/agent` (R-AGENT-WF-001.1/.2).

## Testes da Tarefa

- [ ] Testes unitários (tools de recorrência/% categoria finas; undo redirect; casos especiais; formatação).
- [ ] Testes de integração / e2e (operação diária WhatsApp-only: registro, parcelada, consultas, recorrência, % categoria, casos especiais).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agent/application/services/agent_workflows.go` (seam `buildRegistry`)
- `internal/agent/application/tools/*` (recorrência, % categoria, consultas, conversational/undo)
- `internal/agent/infrastructure/binding/{recurring.go,budget_percentage.go,budget_config.go}`
- `internal/agent/application/prompting/*` (tom/emoji/formatação) + e2e features
