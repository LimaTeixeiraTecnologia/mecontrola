# Tarefa 1.0: Copy da etapa de cartões — palavra "cartão"+💳, "outro" em negrito, exemplo de cadastro

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever a copy da etapa de cartões do onboarding para que toda menção use a palavra "cartão" (com acento) junto do emoji 💳, o convite ao próximo cartão destaque apenas a palavra "outro" em negrito e minúscula, e os prompts tragam um exemplo exato de cadastro (com e sem apelido, em ambos os formatos de dia). Atualizar os testes exact-copy afetados e cobrir a herança apelido←banco.

<requirements>
- RF-01: toda mensagem de cartão contém "cartão" (com acento) + 💳 na mesma frase.
- RF-02: nenhuma mensagem usa 💳 isolado no lugar da palavra "cartão".
- RF-03: não introduzir outro emoji para cartão além de 💳.
- RF-04: convite ao próximo cartão é exatamente "Deseja cadastrar **outro** cartão 💳 agora?", só "outro" em negrito e minúscula.
- RF-05: convite inicial inclui exemplo com apelido e sem apelido.
- RF-06: reprompt após "sim" incompleto inclui o mesmo exemplo (com e sem apelido).
- RF-07 (lado copy): exemplo apresenta o dia em ambos os formatos ("dia 1" e "dia primeiro").
- RF-08: exemplo sem apelido comunica que o apelido fica igual ao banco.
- RF-09 (lado determinístico): `normalizeCardExtract` herda apelido←banco quando o apelido vem vazio; garantir por teste de regressão.
</requirements>

## Subtarefas

- [x] 1.1 Reescrever `cardsPrompt(existing)` (`onboarding_workflow.go:605-610`): ramo `existing>0` com a frase exata de RF-04 + exemplo; ramo `existing==0` com "cartão 💳" + exemplo com/sem apelido (RF-08).
- [x] 1.2 Reescrever `cardsReprompt` e variantes `cardsRepromptMissing{Name,DueDay,Both}` (`:547-566`) usando "cartão 💳" e o exemplo pertinente ao dado faltante (RF-06, RF-07, RF-08).
- [x] 1.3 Atualizar os testes load-bearing que asseguram a copy antiga: `onboarding_workflow_test.go:1959` (`OUTRO 💳`) E `onboarding_workflow_integration_test.go:657` (`require.Contains(..., "OUTRO 💳")` dentro de `TestCardFlow_Integration`) para a nova copy (`Deseja cadastrar **outro** cartão 💳 agora?`); adicionar asserts exact-copy de RF-01..RF-08. Nota: o arquivo de integração é `//go:build integration` e usa mock de agente — o prompt é o texto pré-normalização (contém `**outro**`).
- [x] 1.4 Adicionar/garantir teste de regressão de `normalizeCardExtract` (apelido vazio + banco preenchido → `nickname == bank`) para RF-09.

## Detalhes de Implementação

Ver `techspec.md` seções "Copy — especificação exata" e "Componentes" para as strings e evidências `file:line`. Convenções: negrito via `**...**` (normalizador converte para `*...*`); manter asserts de 💳 já existentes (`:1952,1966,1967,1815,1832`). Não alterar `cardsSystemPrompt`, schemas de extração nem `normalizeCardExtract` (apenas testá-lo).

## Critérios de Sucesso

- Todas as mensagens de cartão contêm "cartão" + 💳 (RF-01/RF-02/RF-03) e o convite ao próximo é exatamente "Deseja cadastrar **outro** cartão 💳 agora?" (RF-04).
- Convite inicial e reprompts contêm o exemplo com e sem apelido em ambos os formatos de dia (RF-05/RF-06/RF-07/RF-08).
- Testes exact-copy verdes, incluindo o `:1959` atualizado; teste de herança apelido←banco verde (RF-09).
- Zero comentários no código Go alterado (R-ADAPTER-001.1); `gofmt`/`go vet` limpos no pacote.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     go-implementation é auto-carregada por detecção de diff (category: language). -->

- `mastra` — a copy vive no workflow de onboarding, consumidor Mastra sobre `internal/platform/{agent,workflow}`; preservar o padrão de prompts do consumidor.
- `domain-modeling-production` — garantir que a mudança de copy não introduza estado/semântica de domínio e preserve a herança apelido←banco como regra pura.
- `design-patterns-mandatory` — confirmar o verdict "não aplicar padrão" (copy localizada, sem abstração nova).

## Testes da Tarefa

- [x] Testes unitários (exact-copy de `cardsPrompt`/reprompts; regressão `normalizeCardExtract`)
- [x] Testes de integração (não aplicável nesta tarefa; extração real-LLM coberta na 3.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `cardsPrompt`, `cardsReprompt*`, `cardsRepromptFor`.
- `internal/agents/application/workflows/onboarding_workflow_test.go` — atualizar `:1959`, adicionar asserts exact-copy e regressão de herança.
- `internal/agents/application/workflows/onboarding_workflow_integration_test.go` — atualizar assert stale `:657` (`OUTRO 💳`) em `TestCardFlow_Integration`.
</content>
