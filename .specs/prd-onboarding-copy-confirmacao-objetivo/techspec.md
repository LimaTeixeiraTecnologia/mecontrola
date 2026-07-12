<!-- spec-hash-prd: 628a71737328fe4e5f10c7b1f222ffa7721919f536f12f91145bbb90bc7c8958 -->

# Especificação Técnica — Onboarding: Boas-vindas, Confirmação do Objetivo, Emoji de Cartão, Sucesso de Cartão e Objetivo Único no Resumo

## Resumo Executivo

Esta funcionalidade é 100% de copy e montagem de mensagem em dois workflows determinísticos do consumidor `internal/agents`: `onboarding_workflow.go` e `card_create_confirm_workflow.go`. Nenhuma alteração toca o motor genérico de workflow (`internal/platform/workflow`, R-WF-KERNEL-001), a extração via LLM, o system prompt do agente (`mecontrola_agent.go`), as tools, o `pending_entry`/`destructive_confirm` ou a idempotência de escrita. A única unidade de lógica nova é uma função **pura** `goalConfirmationReprompt(goal string) string` (DMMF: sem IO, sem `context.Context`, determinística), que prefixa a confirmação+reforço do objetivo à pergunta opcional de valor já existente. As demais mudanças são reescrita de constantes de string (exemplo de boas-vindas, exemplo de valor, blocos de cartão em bullets, remoção de repetição de 💳, selo de sucesso, remoção do objetivo na frase de conclusão). Não há novo padrão de projeto (ADR-003), nem nova métrica, tabela, estado/enum ou call-site de LLM.

A entrega segue o caminho já existente `suspendStep(prompt) → StepOutput.Suspend.Prompt → usecase → consumer.sendReply → NormalizeOutboundText → gateway.SendTextMessage`. O normalizador (`internal/platform/whatsapp/formatting/normalize.go`) permanece intacto e continua responsável por `** → *`, prefixo `📊` em "Resumo de Onboarding" e `✅` na confirmação de orçamento — nenhuma mudança nele é necessária, pois a regra de emoji 💳 é resolvida na origem (constantes/funções de montagem).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes **modificados** (nenhum novo componente estrutural):

- `internal/agents/application/workflows/onboarding_workflow.go` — constantes e funções de montagem de mensagem: `welcomeCombinedPrompt`, `goalValueReprompt`, nova função pura `goalConfirmationReprompt`, `cardsReprompt`/`cardsRepromptMissingName`/`cardsRepromptMissingDueDay`/`cardsRepromptMissingBoth`, `cardsPrompt`, nova constante `cardCreatedSuccessOnboarding`, `renderCardsSummary`, `conclusionSummaryMessage`, `conclusionFinalMessage`, e o ponto pós-`CreateCard` em `BuildCardsStep`.
- `internal/agents/application/workflows/card_create_confirm_workflow.go` — remoção do 💳 em todas as mensagens exceto a pergunta de confirmação inicial e o selo de sucesso.
- Arquivos de teste que travam a copy (unit + integration) — atualização dos asserts. Ver seção "Abordagem de Testes".

Componentes **inalterados** (fronteiras preservadas): motor `internal/platform/workflow`; `internal/platform/whatsapp/formatting/normalize.go`; `resolve_onboarding_or_agent.go` (usecase que só repassa `Suspend.Prompt`); `whatsapp_inbound_consumer.go` (só entrega); `mecontrola_agent.go`; tools; golden cases (exceto se algum assert de journey precisar de ajuste de 💳, ver testes).

### Fluxo de Dados (inalterado)

```
Workflow step -> suspendStep(prompt) -> StepOutput.Suspend.Prompt
  -> usecase.ResolveOnboardingOrAgent (Start/Resume) -> OnboardingResult.Message
  -> consumer.tryResolveOnboarding -> sendReply(Message)
  -> formatting.NormalizeOutboundText (** -> *, 📊, ✅)
  -> gateway.SendTextMessage
```

## Design de Implementação

### Interfaces Chave

Nova função pura (DMMF Princípio 6 — puro, determinístico, sem IO):

```go
func goalConfirmationReprompt(goal string) string {
    return fmt.Sprintf(
        "Perfeito! Anotei seu objetivo: \"%s\" 🎯 Vamos juntos tornar isso realidade! 💪\n\n%s",
        goal,
        goalValueReprompt,
    )
}
```

Assinatura alterada (remoção de parâmetros não mais usados — o objetivo sai da frase de conclusão):

```go
func conclusionFinalMessage() string
```

### Modelos de Dados

Nenhum. Sem novo tipo, estado, enum, campo de domínio, tabela ou migração. `OnboardingState` e `CardCreateState` permanecem inalterados. A regra de emoji é resolvida por string estática/`fmt.Sprintf`, sem estado adicional.

### Strings Concretas (fonte de verdade da implementação)

Item 1 — Boas-vindas (`welcomeCombinedPrompt`, linha 527): trocar apenas o fragmento
`comprar uma casa, meta de R$ 400.000,00` por `comprar um celular novo, meta de R$ 5.000,00`. Nenhuma outra linha muda.

Item 2 — Pergunta de valor (`goalValueReprompt`, linha 531): trocar o exemplo de formato
`"R$ 400.000,00" ou "400 mil"` por `"R$ 5.000,00" ou "5 mil"`. A confirmação+reforço é prefixada por `goalConfirmationReprompt(state.Goal)` nos dois pontos de emissão (linhas 768 e 775), substituindo `suspendStep(state, goalValueReprompt)` por `suspendStep(state, goalConfirmationReprompt(state.Goal))`.

Item 3 — Cartão em bullets, 💳 só na 1ª mensagem de cartão (convite inicial):

`cardsPrompt(existing == 0)` — convite inicial, 1× 💳:
```
O cartão 💳 é opcional. Você deseja cadastrar um cartão agora?

Por exemplo:
• "Roxinho, Nubank e vencimento dia 1"
• "Nubank e vencimento dia primeiro" (sem apelido, o apelido fica igual ao banco)

Se não quiser agora, é só responder "não" e seguir sem cartão.
```

`cardsPrompt(existing > 0)` — convite inicial em sessão retomada, 1× 💳, mantém contagem e `**outro**`:
```
Você já tem %d cartão 💳 cadastrado(s). Deseja cadastrar **outro** cartão agora?

Por exemplo:
• "Roxinho, Nubank e vencimento dia 1"
• "Nubank e vencimento dia primeiro" (sem apelido, o apelido fica igual ao banco)

Se não, responda "não".
```

`cardsReprompt` / `cardsRepromptMissingName` / `cardsRepromptMissingDueDay` / `cardsRepromptMissingBoth` — 0× 💳, em bullets, preservando os fragmentos obrigatórios (exemplos com e sem apelido, "dia 1"/"dia primeiro", nota de apelido = banco). A palavra passa a ser "cartão" (sem 💳).

Item 4 — Selo de sucesso (nova constante, substitui `cardsPrompt(len(existingCards))` na linha 888):
```
const cardCreatedSuccessOnboarding = "💳 Cartão registrado com sucesso ✅\nQuer registrar algum outro?"
```

Item 3 (resumo) — remoção do 💳 na seção de cartões:
- `conclusionSummaryMessage` linha 701: `"\nCartões 💳:\n"` -> `"\nCartões:\n"`.
- `renderCardsSummary` linha 671: `"Nenhum cartão 💳 cadastrado."` -> `"Nenhum cartão cadastrado."`.

Item 5 — Objetivo uma vez (`conclusionFinalMessage`, linhas 650-660): remover o objetivo e o parâmetro; preservar a CTA:
```
Tudo pronto! 🚀

Agora é só começar: me envie seus gastos e receitas no dia a dia (ex.: "gastei R$ 50 no mercado" ou "recebi R$ 200 de freela") que eu registro tudo pra você. Vamos juntos! 💪
```
O cabeçalho `🎯 Objetivo:` em `conclusionSummaryMessage` (linhas 694/696) permanece — é a única aparição do objetivo. O caller na linha 706 passa a chamar `conclusionFinalMessage()` sem argumentos.

Avulso (`card_create_confirm_workflow.go`) — aplicar RF-15 (mantém 💳 só na pergunta de confirmação inicial, linha 94, e no selo de sucesso, linha 155; remove 💳 de todas as outras):
- linha 60 (cancelamento): `🚫 Cadastro de cartão cancelado conforme solicitado.`
- linha 65 (reprompt): `Não entendi. Por favor, responda apenas *sim* ou *não* para confirmar o cadastro do cartão.`
- linha 87 (cancelamento ambíguo): `🚫 Cadastro de cartão cancelado: resposta não reconhecida.`
- linhas 132/145 (erro infra): `Não consegui cadastrar o cartão. Tente novamente em breve.`
- linha 153 (idempotência): `✅ *%s* já estava cadastrado.`
- linhas 179/181/187/189 (erros de domínio): trocar `💳` por `cartão`. Linhas 183/185 já não têm 💳.
- linha 94 (confirmação) e linha 155 (sucesso): **inalteradas** (mantêm 💳).

## Pontos de Integração

Nenhuma integração externa nova. WhatsApp Meta permanece o canal único, pelo caminho de entrega existente. OpenRouter (LLM) não é chamado por nenhuma das mudanças (a confirmação+reforço é determinística — ADR-002).

## Abordagem de Testes

### Testes Unitários

Atualizar os asserts que travam a copy alterada (padrão canônico testify/suite já vigente; whitebox `package workflows`). Lista derivada do inventário (file:line) — `onboarding_workflow_test.go`:
- Boas-vindas: o `expected` literal em ~2470-2474 (`s.Equal(expected, welcomeCombinedPrompt)`) atualiza o fragmento do exemplo. Os asserts `s.Equal(welcomeCombinedPrompt, out.Suspend.Prompt)` (772, 2330, 2344, 2361) comparam com a constante e permanecem válidos.
- Confirmação+reforço: 817, 858, 910 (`s.Equal(goalValueReprompt, out.Suspend.Prompt)`) passam a `s.Equal(goalConfirmationReprompt(<goalDoCenario>), out.Suspend.Prompt)`. Adicionar caso unitário puro para `goalConfirmationReprompt` verificando: contém o objetivo ecoado entre aspas, contém `goalValueReprompt`, e não faz IO. Assert 216-217 (`goalValueReprompt` não vazio e contém "não") permanece.
- Exemplo de valor: adicionar assert de que `goalValueReprompt` contém `R$ 5.000,00` e não contém `400.000`.
- Cartão em bullets/emoji: 1958 (`Deseja cadastrar **outro** cartão 💳 agora?`) atualiza para `Deseja cadastrar **outro** cartão agora?` (sem 💳). 1952-1988 (asserts de `cartão 💳`, exemplos, "outro" bold) atualizam para: 💳 presente só em `cardsPrompt(0)`/`cardsPrompt(1)` na 1ª menção; ausente nos reprompts; exemplos e "outro" bold preservados. 1815/1832 (`Contains(prompt, "💳")`) verificar contra o novo posicionamento. 2002 (`NotContains "💰💳"`) permanece.
- Selo de sucesso: novo assert de que a mensagem pós-`CreateCard` é exatamente `cardCreatedSuccessOnboarding` e contém `💳 Cartão registrado com sucesso ✅` e `Quer registrar algum outro?`.
- Resumo: 2175-2176 (`Cartões 💳:`, `Nenhum cartão 💳 cadastrado.`) atualizam para `Cartões:` e `Nenhum cartão cadastrado.`.
- Conclusão: 2178, 2294-2306 (`conclusionFinalMessage(...)` com "está registrado"/"meta de") atualizam para a assinatura sem args e para a nova string sem objetivo; 2298-2306 passa a assegurar `NotContains(msg, "objetivo")` e `NotContains(msg, "está registrado")` mantendo a CTA.

`card_create_confirm_workflow` (unit): adicionar asserts de RF-15 — reprompt, cancelamento, erros de domínio, erro de infra e idempotência **não** contêm `💳`; a pergunta de confirmação inicial e o selo de sucesso **contêm** `💳`. Os asserts existentes de `✅` (142) e do gate de "cadastrado com sucesso" (harness 320/334) permanecem válidos (o selo de sucesso mantém a frase e o 💳).

### Testes de Integração

Sim — o projeto já tem fronteiras reais cobertas por integração de onboarding ponta a ponta (`whatsapp_inbound_consumer_integration_test.go`, `onboarding_workflow_integration_test.go`, `onboarding_workflow_postgres_resume_integration_test.go`) sob `//go:build integration`. Ações:
- `whatsapp_inbound_consumer_integration_test.go`: reverificar `replies[6]`/`replies[7]` `Contains("💳")` (390/392) contra o novo posicionamento — o convite inicial de cartão mantém 💳; se `replies[7]` corresponder a um reprompt ou ao selo, ajustar (selo mantém 💳; reprompt perde). Manter `Contains(replies[8], "Resumo de Onboarding")` (397). O input `resumeText: "comprar uma casa..."` (123) é entrada do usuário e não precisa mudar.
- Não são necessários novos containers nem novas dependências reais; reusar a suíte existente.

### Testes E2E

Gate golden real-LLM agregado por categoria (`internal/agents/application/golden/harness_realllm_test.go`, threshold `goldenGateThreshold = 0.90`) roda `RUN_REAL_LLM=1`. Os 2 casos de onboarding (`cases_onboarding.go`) validam presença de "Bem-vindo"/"objetivo financeiro" (não copy exata) — permanecem verdes. Rodar o gate após as mudanças para confirmar `CategoryOnboarding` ≥ 0,90. Não introduzir novo caso golden de copy exata (a copy determinística é coberta por unit/integration; o golden cobre comportamento do LLM, que não muda).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `onboarding_workflow.go` — item 1 (welcome) e item 2 (`goalConfirmationReprompt` + `goalValueReprompt`), pois são isolados e de baixo risco.
2. `onboarding_workflow.go` — item 3 (cartão em bullets + regra de emoji), item 4 (`cardCreatedSuccessOnboarding` no pós-`CreateCard`), item 5 (`conclusionFinalMessage` + seção de cartões do resumo).
3. `card_create_confirm_workflow.go` — RF-15 (regra de emoji no avulso).
4. Atualização dos testes unitários e de integração afetados; rodar `go build`/`go vet`/`go test -race`/lint no módulo `internal/agents`.
5. Gate golden real-LLM (`RUN_REAL_LLM=1`) para `CategoryOnboarding`.

### Dependências Técnicas

Nenhuma infraestrutura nova. Testes de integração usam a suíte existente (`//go:build integration`). O gate golden exige `OPENROUTER_*` no `.env` (real-LLM), conforme prática já vigente.

## Monitoramento e Observabilidade

Nenhuma métrica, label, log ou dashboard novo. `workflow_steps_total{workflow="onboarding-workflow"}`, `onboarding_workflow_total` e a cardinalidade controlada permanecem inalterados (mudança só de string). Cardinalidade preservada (R-TXN-004/R-AGENT-WF-001.5): nenhum novo label.

## Considerações Técnicas

### Decisões Chave

- ADR-001 — Regra de emoji 💳 por-fluxo (1ª mensagem de cartão + selo de sucesso), escopo restrito a onboarding + avulso; supersede a copy de emoji de PRDs anteriores. Ver `adr-001-regra-emoji-cartao-por-fluxo.md`.
- ADR-002 — Confirmação+reforço do objetivo determinístico (função pura, sem LLM). Ver `adr-002-reforco-objetivo-deterministico-sem-llm.md`.
- ADR-003 — Não aplicar design pattern (gate `design-patterns-mandatory`): mudança de copy + uma função pura de formatação não introduz abstração, polimorfismo ou colaboração de objetos que justifique padrão GoF. Ver `adr-003-nao-aplicar-design-pattern.md`.

### Riscos Conhecidos

- **Quebra de asserts de copy**: muitos testes travam strings exatas (inventário mapeado). Mitigação: lista file:line completa na seção de testes; atualizar no mesmo PR; `go test` verde é gate.
- **Journey de integração (`replies[6]/[7]` com 💳)**: o reposicionamento do 💳 pode alterar quais replies contêm 💳. Mitigação: reverificar índice a índice contra o novo posicionamento antes de fechar.
- **Multi-cartão e "no máximo 2× 💳"**: cada cadastro bem-sucedido emite um selo com 💳; se o usuário cadastrar N cartões, há 1 (convite) + N (selos) aparições. Isto é autorizado por RF-07(b) (o selo de sucesso é a exceção por cadastro); o objetivo "no máximo 2×" refere-se ao caminho comum de 0–1 cartão. Sem ambiguidade: a regra é "1ª mensagem + selo de sucesso".
- **Avulso e gate de falso-sucesso**: o selo mantém a frase "cadastrado com sucesso" e o 💳; os gates `card_create_harness_test.go:320/334` (que barram falso-sucesso) permanecem válidos.

### Conformidade com Padrões

- R-ADAPTER-001.1 — zero comentários em `.go` de produção; todas as mudanças respeitam.
- R-AGENT-WF-001 — comportamento no consumidor sem `switch case intent.Kind`; LLM apenas nas call-sites já sancionadas; nenhuma nova call-site introduzida.
- R-WF-KERNEL-001 — motor `internal/platform/workflow` intocado.
- R-TESTING-001 — testes de use case/workflow no padrão testify/suite whitebox já vigente; asserts atualizados dentro do padrão.
- DMMF (`domain-modeling-production`) — `goalConfirmationReprompt` e `conclusionFinalMessage` puras; sem novo estado; state-as-type inalterado.

### Arquivos Relevantes e Dependentes

- `internal/agents/application/workflows/onboarding_workflow.go` (produção — itens 1–5).
- `internal/agents/application/workflows/card_create_confirm_workflow.go` (produção — RF-15).
- `internal/agents/application/workflows/onboarding_workflow_test.go` (asserts de copy).
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` (`//go:build integration`).
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go` (journey, `replies[...]` 💳/Resumo).
- `internal/agents/application/workflows/card_create_confirm_workflow_test.go` (asserts RF-15).
- `internal/agents/application/golden/harness_realllm_test.go` + `cases_onboarding.go` (gate agregado, sem novo caso).
- Inalterados (não editar): `internal/platform/whatsapp/formatting/normalize.go`, `resolve_onboarding_or_agent.go`, `whatsapp_inbound_consumer.go`, `mecontrola_agent.go`, tools, `internal/platform/workflow`.
