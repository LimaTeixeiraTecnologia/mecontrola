# Prompt

```md
Crie um PRD, em portugues do Brasil, para evoluir o agente financeiro do MeControla com foco em garantir que `internal/agents/application/agents/mecontrola_agent.go` disponha de uma superficie de tools completa, precisa, production-ready e realmente utilizada em execucao, sem gaps, sem lacunas, sem falso positivo de cobertura, sem desvios e sem flexibilidade interpretativa fora do dominio real do sistema.

Este output sera usado como entrada da skill `create-prd`.

Restricoes inegociaveis:
- Nao implemente nada.
- Nao produza plano tecnico, diff, pseudocodigo, desenho de classes ou proposta de refactor.
- Produza apenas um artefato de produto no formato de PRD.
- O PRD deve considerar como obrigatorias, para a futura implementacao, as skills `go-implementation` e `mastra`.
- O PRD deve refletir estritamente o codigo real do workspace atual.
- E proibido inventar tools, APIs, handlers, workflows, use cases, contratos, nomes, capacidades ou comportamentos nao verificados no repositorio.
- Se houver lacuna de contexto, registre como suposicao ou questao em aberto; nao preencha com invencao.

Contexto obrigatorio do repositorio:
- O projeto e um monolito modular em Go.
- A capacidade agentiva usa o substrato `internal/platform/{agent,llm,memory,workflow,tool,scorer}` e o consumidor `internal/agents`.
- O objetivo e garantir que o agente MeControla tenha cobertura correta e uso efetivo das capacidades relevantes de `internal/budgets`, `internal/card`, `internal/categories` e `internal/transactions`.

Estado atual confirmado no codigo:
- `internal/agents/module.go` atualmente registra 9 tools no agente:
  - `BuildRegisterExpenseTool`
  - `BuildRegisterIncomeTool`
  - `BuildRegisterCardPurchaseTool`
  - `BuildQueryMonthTool`
  - `BuildQueryPlanTool`
  - `BuildEditEntryTool`
  - `BuildDeleteEntryTool`
  - `BuildAdjustAllocationTool`
  - `BuildClassifyCategoryTool`
- `internal/agents/application/agents/mecontrola_agent.go` define o agente `mecontrola-agent`.
- `internal/budgets` expoe, no minimo, criacao e ativacao de budget, recorrencia, exclusao de draft, alertas, resumo mensal, upsert e delete de expense, edicao de percentual por categoria e sugestao de alocacao.
- `internal/card` expoe, no minimo, criacao, listagem, consulta, atualizacao, atualizacao de limite, exclusao logica e fatura por cartao.
- `internal/categories` expoe, no minimo, listagem, detalhe, dicionario, busca em dicionario, resolucao por slug e validacao de subcategoria.
- `internal/transactions` expoe, no minimo, criacao, edicao, exclusao e consulta de transacoes, criacao, edicao e exclusao de compras no cartao, resumo mensal, listagem mensal, busca, recorrencias, fatura por cartao e verificacao de parcelas em aberto.
- A instrucao `bash scripts/verify-go-mod.sh` referenciada por `go-implementation` nao foi encontrada no workspace atual; trate isso apenas como restricao de contexto.

Objetivo do PRD:
- Definir a necessidade de produto para que o agente tenha todas as tools necessarias para operar corretamente sobre as capacidades relevantes dos modulos citados.
- Garantir que o agente saiba escolher a tool correta, pedir apenas o dado faltante, executar a acao correta, confirmar operacoes destrutivas quando necessario e nunca simular sucesso sem execucao real.
- Garantir que a disponibilidade das tools nao seja apenas nominal: o agente precisa usa-las de forma efetiva, eficiente, deterministica e auditavel.

O PRD deve distinguir explicitamente:
1. capacidades ja expostas hoje em tools no `internal/agents`;
2. capacidades existentes nos modulos de negocio que ainda nao estao expostas ao agente;
3. capacidades existentes nos modulos que nao devem virar tool conversacional.

Requisitos obrigatorios do PRD:
- Definir problema, objetivo, ator principal, escopo incluido, escopo excluido, restricoes, criterios de sucesso e requisitos funcionais numerados.
- Incluir requisitos funcionais claros para cobertura funcional exaustiva baseada no codigo real.
- Incluir requisitos para mapeamento formal `capacidade do modulo -> tool do agente`.
- Incluir requisitos para identificacao formal de gaps entre os modulos e a tool surface atual do agente.
- Incluir requisitos para uso deterministico das tools pelo agente.
- Incluir requisitos para proibicao de inventar dados, operacoes, resultados ou sucesso de execucao.
- Incluir requisitos para confirmacao explicita em operacoes destrutivas.
- Incluir requisitos objetivos para classificar a solucao como production-ready do ponto de vista de produto.
- Incluir requisitos para observabilidade e auditabilidade do uso das tools.
- Incluir requisitos para validar uso efetivo das tools, e nao apenas seu registro no runtime.
- Incluir requisitos para impedir desvios do dominio financeiro pessoal do MeControla.
- Incluir requisitos para impedir falso positivo de cobertura funcional.
- Incluir requisitos para impedir classificacao de sucesso quando ainda houver capacidade relevante do modulo nao refletida no agente.

O PRD deve responder com clareza:
- qual problema de produto esta sendo resolvido;
- para qual usuario ou ator principal;
- quais capacidades de `budgets`, `card`, `categories` e `transactions` precisam estar disponiveis ao agente;
- quais capacidades nao devem ser expostas como tool conversacional;
- como medir cobertura real;
- como medir ausencia de gaps;
- como medir ausencia de falso positivo;
- como medir uso correto das tools pelo agente;
- quais riscos, restricoes e questoes em aberto precisam constar antes de qualquer implementacao.

Criterios de aceitacao do output:
- O resultado deve sair pronto para ser persistido como PRD pela skill `create-prd`.
- O documento deve ser concreto, testavel, numerado e orientado a decisao.
- O documento nao pode conter recomendacoes vagas como "adicionar tools necessarias".
- O documento deve mencionar caminhos reais do repositorio quando isso delimitar escopo.
- O documento deve deixar explicito que sucesso nao significa apenas "ter mais tools", e sim ter cobertura correta, uso correto, aderencia ao codigo real e comportamento confiavel em producao.
- O documento deve incluir `Suposicoes e Questoes em Aberto` sempre que algo nao puder ser afirmado com certeza a partir do workspace atual.
```
