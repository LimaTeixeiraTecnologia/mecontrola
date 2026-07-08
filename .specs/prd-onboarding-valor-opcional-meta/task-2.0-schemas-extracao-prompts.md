# Tarefa 2.0: Schemas de extração + structs + system prompts (ADR-001)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar os dois schemas strict de extração (combinado meta+valor e value-only), suas structs de unmarshal, e os system prompts com exemplos de conversão de formato. Atualizar a mensagem inicial e a repergunta do `step-goal` para o padrão combinado com convite opcional ao valor.

<requirements>
- RF-01: extração combinada meta+valor numa única chamada ao parser LLM strict.
- RF-09: suportar máscara monetária ("R$ 400.000,00", "400000") e formas coloquiais ("10 mil reais", "400 mil", "1,5 milhão"), convertendo para centavos (conversão no LLM → `amountBRL number`).
- RF-13: a mensagem inicial do `step-goal` menciona o valor opcional como exemplo/convite, sem torná-lo obrigatório.
- ADR-001: dois schemas + par sentinela `hasAmount`+`amountBRL` (ambos `required`, `additionalProperties:false`).
</requirements>

## Subtarefas

- [ ] 2.1 Adicionar `goalWithValueSchema` (`goal`, `hasAmount`, `amountBRL`; todos required) e `goalValueSchema` (`hasAmount`, `amountBRL`) junto a `goalSchema`/`incomeSchema` (~L359-375).
- [ ] 2.2 Adicionar structs `goalWithValueExtract` e `goalValueExtract` junto a `goalExtract` (~L331).
- [ ] 2.3 Adicionar `_goalWithValueSystemPrompt` e `_goalValueSystemPrompt` com exemplos de conversão RF-09 (instruction-by-example) junto aos prompts (~L412+).
- [ ] 2.4 Atualizar `_welcomeGoalPrompt` (~L412) para convidar o valor como exemplo opcional; substituir `_goalReprompt` (~L417) pela versão combinada (objetivo + valor opcional) e adicionar `_goalValueReprompt`.

## Detalhes de Implementação

Ver `techspec.md` seção "Modelos de Dados" (schemas/structs verbatim) e `adr-001-extracao-combinada-sentinela.md`. Conversão de formato coloquial é responsabilidade do LLM (retorna `amountBRL float64`), nunca parser Go — preserva `Decide*` puro (R-AGENT-WF-001.4). Não alterar `goalSchema`/`incomeSchema` existentes.

## Critérios de Sucesso

- Schemas seguem o padrão `map[string]any` de `incomeSchema` com `Strict:true`/`additionalProperties:false`.
- Prompts contêm os 5 formatos de RF-09 como exemplos.
- `go build ./...`, `go vet ./...` verdes; zero comentários.
- Textos de UX em PT-BR, tom consistente com os prompts atuais do onboarding.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — schemas strict e system prompts do parser LLM sancionado (contrato de extração estruturada `agent.Agent.Execute`).
- `domain-modeling-production` — contrato de dados da extração (par sentinela) alinhado ao modelo de estado.
- `design-patterns-mandatory` — gate confirmou "sem Strategy" para os dois extratores; registrar a decisão de dois schemas.

## Testes da Tarefa

- [ ] Testes unitários (validação de que os schemas desserializam nas structs; exercitados de fato em 3.0/5.0)
- [ ] Testes de integração (a extração real é validada pelo harness da 5.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — schemas (~L359), structs (~L331), prompts (~L412).
- `internal/platform/llm/types.go` — `llm.Schema` (referência).
