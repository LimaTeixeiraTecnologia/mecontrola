# US-01: Tornar determinística a prova dos jobs e do write ledger do módulo `internal  /  agents`

## Declaração
Como mantenedor do módulo `internal  /  agents`, quero que os jobs de retenção / confirmação e o repositório `write_ledger` tenham testes determinísticos executáveis no fluxo padrão do módulo, para que falhas de agendamento, propagação de erro e persistência idempotente sejam detectadas sem depender apenas de suites `integration`.

## Contexto
- Problema: os componentes de infraestrutura que sustentam retenção e idempotência do módulo existem no código de produção, mas hoje a prova deles está incompleta no caminho padrão de testes. `ConfirmReaperJob` e `LedgerRetentionJob` têm comportamento próprio em produção (`internal  /  agents / infrastructure / jobs / handlers / confirm_reaper_job.go:12-40`, `internal  /  agents / infrastructure / jobs / handlers / ledger_retention_job.go:12-33`) e não possuem arquivo `_test.go` correspondente no módulo. O `write_ledger_repository` concentra lógica de fallback de conexão, mapeamento de `sql.ErrNoRows`, absorção de `UniqueViolation` e tratamento de `RowsAffected` (`internal  /  agents / infrastructure / persistence / write_ledger_repository.go:23-113`), mas sua prova atual está toda sob ` /  / go:build integration` (`internal  /  agents / infrastructure / persistence / write_ledger_repository_integration_test.go:1-180`).
- Resultado esperado: o módulo deve ter prova rápida e repetível para contratos críticos desses artefatos, cobrindo happy path, erro e edge case sem exigir container, rede ou variáveis externas para validar o comportamento básico.
- Fonte: solicitação direta do usuário para auditar 100% dos `_test.go` de `internal  /  agents`, confrontada com leitura do código de produção e dos testes existentes nesta sessão, além da execução local de `go test -cover . / internal  /  agents / ...`, que passou, porém deixou `internal  /  agents / infrastructure / jobs / handlers` e `internal  /  agents / infrastructure / persistence` fora da prova padrão.

## Regras de Negócio
- Jobs do módulo são adapters finos e devem provar explicitamente nome padrão, `schedule` padrão, `timeout` e propagação do erro do use case / reaper.
- O `write_ledger_repository` deve provar contratos de idempotência e tradução de erro no menor nível possível: `sql.ErrNoRows -> ErrLedgerEntryNotFound`, `UniqueViolation -> nil`, erro genérico de banco envelopado com contexto e falha em `RowsAffected` tratada como erro.
- Testes `integration` existentes continuam válidos como prova complementar de comportamento com Postgres real; eles não substituem a prova determinística mínima do fluxo padrão.
- O baseline de robustez deve valer tanto para `go test . / internal  /  agents / ...` quanto para a execução ampliada com `integration`.

## Critérios de Aceite
```gherkin
Cenário: Job propaga erro do reaper ou do use case
  Dado um `ConfirmReaperJob` ou `LedgerRetentionJob` com dependência fake que retorna erro
  Quando `Run(ctx)` for executado
  Então o teste deve comprovar que o erro é retornado sem mascaramento
  E o contrato de `Name`, `Timeout` e `Schedule` padrão também deve estar coberto

Cenário: Repositório traduz erros estruturais de persistência
  Dado um `writeLedgerRepository` com double de `database.DBTX`
  Quando `FindByKey` receber `sql.ErrNoRows` ou `Insert` receber `UniqueViolation`
  Então o teste deve comprovar respectivamente `ErrLedgerEntryNotFound` e retorno `nil`
  E, quando o banco falhar com erro genérico, o erro deve sair envelopado com contexto do adapter

Cenário: Suite padrão detecta regressão sem depender de integration
  Dado o módulo `internal  /  agents` em execução local ou CI
  Quando `go test . / internal  /  agents / ...` for executado sem build tags extras
  Então os jobs e o contrato básico do `write_ledger_repository` devem permanecer exercitados
  E uma regressão nesses artefatos deve falhar a suite padrão
```

## Dados e Permissões
- Dados obrigatórios: doubles para `staleSuspendedReaper`, `purgeLedgerUseCase`, `database.DBTX`, `sql.Result` e erros representativos (`sql.ErrNoRows`, `pgconn.PgError`, erro de `RowsAffected`).
- Perfis / permissões: sem permissão de negócio; escopo técnico interno ao módulo `internal  /  agents`.

## Dependências
- Depende do contrato atual de `writeLedgerRepository` em `internal  /  agents / infrastructure / persistence / write_ledger_repository.go`.
- Depende da manutenção das suites `integration` já existentes em `internal  /  agents / infrastructure / persistence / write_ledger_repository_integration_test.go`.
- Depende de doubles simples de banco e result, sem necessidade de nova integração externa.

## Fora de Escopo
- Não inclui redesign do `write_ledger_repository` nem mudança de schema do banco.
- Não inclui alterar a semântica dos jobs, apenas provar o contrato já implementado.
- Não inclui substituir os testes `integration` por unitários.

## Evidências
- Entrada:
  - Pedido do usuário para auditar 100% do módulo `internal  /  agents` com foco em robustez e confiabilidade dos `_test.go`.
  - Execução local nesta sessão: `go test . / internal  /  agents / ...` passou, mas o recorte padrão não exerceu `jobs / handlers` nem `persistence` de forma executável sem `integration`.
- Base de código:
  - `internal  /  agents / infrastructure / jobs / handlers / confirm_reaper_job.go:12-40` — job com defaults de nome / schedule e propagação direta do `Reap`.
  - `internal  /  agents / infrastructure / jobs / handlers / ledger_retention_job.go:12-33` — job com defaults de nome / schedule e propagação direta de `Execute`.
  - `internal  /  agents / infrastructure / persistence / write_ledger_repository.go:23-113` — lógica de `conn(ctx)`, `FindByKey`, `Insert` com `UniqueViolation` e `DeleteBefore`.
  - `internal  /  agents / infrastructure / persistence / write_ledger_repository_integration_test.go:1-180` — toda a prova atual do repositório está sob ` /  / go:build integration`.
  - `internal  /  agents / module.go:237-242` — os jobs são efetivamente montados no composition root do módulo.
- Inferências:
  - A ausência de `_test.go` para `jobs / handlers` e a dependência exclusiva de `integration` no repositório elevam risco de regressão silenciosa no fluxo padrão de CI.
- Não evidenciado:
  - Não foi encontrado `_test.go` dedicado em `internal  /  agents / infrastructure / jobs / handlers / `.
  - Não foi encontrada prova unitária do ramo `DeleteBefore(...).RowsAffected()` com erro.

## Notas de Validação
- Esta história é uma história de habilitação com resultado observável de engenharia: reduzir o espaço de regressões silenciosas em contratos críticos de retenção e idempotência do módulo.
- Os cenários cobrem fluxo feliz implícito de construção / defaults, fluxo alternativo de persistência idempotente e fluxo de erro de banco / reaper, atendendo o requisito de production-ready / proof solicitado pelo usuário.

# US-02: Cobrir os blind spots comportamentais do `transactions_ledger_adapter`

## Declaração
Como mantenedor do módulo `internal  /  agents`, quero cobertura comportamental explícita para as operações ainda não provadas do `transactions_ledger_adapter`, para que os adapters de leitura / escrita financeira não passem em falso positivo quando houver regressão em mapeamento, erro de principal ou transformação de payload.

## Contexto
- Problema: o adapter `transactions_ledger_adapter` implementa múltiplas portas críticas (`GetCardInvoice`, `SearchTransactions`, `CreateRecurringTemplate`, além de `GetTransaction`) em `internal  /  agents / infrastructure / binding / transactions_ledger_adapter.go:207-380`. O teste unitário atual do adapter cobre apenas `GetMonthlySummary` e `ListMonthlyEntries` (`internal  /  agents / infrastructure / binding / transactions_ledger_adapter_test.go:43-162`). A busca textual executada nesta sessão não encontrou cenários diretos para `GetCardInvoice`, `SearchTransactions` nem `CreateRecurringTemplate` nos testes do adapter nem na suite `transactions_integration_test.go`.
- Resultado esperado: cada operação pública do adapter deve ter pelo menos uma prova de sucesso, uma de erro e uma de edge case relevante de transformação / identidade.
- Fonte: solicitação do usuário confrontada com leitura completa do adapter e do arquivo de testes unitários do próprio adapter.

## Regras de Negócio
- Adapter deve continuar fino: testes devem provar tradução de contexto / identidade e mapeamento de DTO, não regra de negócio do módulo `transactions`.
- Cada operação pública do adapter precisa de prova explícita do erro de identidade ausente ou inválida quando o contrato passa por `principalCtx`.
- Funções que transformam slices e campos opcionais devem ter edge case cobrindo lista vazia, `SubcategoryID` nulo ou itens múltiplos conforme o contrato específico.
- A prova deve distinguir comportamento unitário do adapter e comportamento de integração do módulo `transactions`; não basta depender de um teste amplo que passe por criação de transação e resumo mensal.

## Critérios de Aceite
```gherkin
Cenário: Adapter mapeia corretamente payloads e resultados de leitura
  Dado um `transactions_ledger_adapter` com use cases fake para `GetCardInvoice`, `SearchTransactions` e `GetTransaction`
  Quando cada operação for executada com principal válido
  Então o teste deve comprovar que o payload enviado ao use case e o resultado retornado pelo adapter preservam os campos esperados
  E campos opcionais como `SubcategoryID` devem ser verificados explicitamente

Cenário: Adapter falha quando a identidade inbound está ausente ou inválida
  Dado um `transactions_ledger_adapter` chamado sem principal / autenticação válida
  Quando `GetCardInvoice`, `SearchTransactions` ou `CreateRecurringTemplate` forem invocados
  Então o teste deve falhar com erro de identidade
  E o use case downstream não deve ser chamado

Cenário: Adapter propaga erro do caso de uso com contexto suficiente
  Dado um `transactions_ledger_adapter` cujo use case downstream retorna erro
  Quando a operação pública correspondente for executada
  Então o teste deve comprovar que o erro sai envelopado com contexto do adapter
  E nenhum resultado parcial inválido é retornado como sucesso
```

## Dados e Permissões
- Dados obrigatórios: principal / autenticação válida e inválida; doubles para `GetCardInvoice`, `SearchTransactions`, `CreateRecurringTemplate` e `GetTransaction`; payloads com `SubcategoryID` nulo e preenchido.
- Perfis / permissões: uso interno autenticado via principal WhatsApp / Header já exigido pelo adapter.

## Dependências
- Depende de `principalCtx` em `internal  /  agents / infrastructure / binding / transactions_ledger_adapter.go:52-65`.
- Depende das interfaces e DTOs do módulo `transactions` já existentes.
- Pode reutilizar o padrão de suite já adotado em `internal  /  agents / infrastructure / binding / transactions_ledger_adapter_test.go`.

## Fora de Escopo
- Não inclui alterar a lógica de negócio do módulo `transactions`.
- Não inclui ampliar a suite de integração para a cobertura inteira do adapter se a mesma prova puder ser feita unitariamente.
- Não inclui redesign do adapter ou mudança de assinatura pública.

## Evidências
- Entrada:
  - Pedido do usuário para verificar se os testes realmente testam edge cases, lacunas e desvios de fluxo do módulo.
- Base de código:
  - `internal  /  agents / infrastructure / binding / transactions_ledger_adapter.go:207-245` — `GetTransaction` faz transformação de `SubcategoryID`.
  - `internal  /  agents / infrastructure / binding / transactions_ledger_adapter.go:247-286` — `GetCardInvoice` transforma itens de fatura.
  - `internal  /  agents / infrastructure / binding / transactions_ledger_adapter.go:288-331` — `SearchTransactions` transforma lista de entradas com `SubcategoryID` opcional.
  - `internal  /  agents / infrastructure / binding / transactions_ledger_adapter.go:333-360` — `CreateRecurringTemplate` monta DTO de saída do adapter.
  - `internal  /  agents / infrastructure / binding / transactions_ledger_adapter_test.go:43-162` — cobertura atual restrita a `GetMonthlySummary` e `ListMonthlyEntries`.
  - `internal  /  agents / infrastructure / binding / transactions_integration_test.go:1-260` — suite de integração centrada em criação / idempotência / resumo, sem evidência textual das operações públicas acima.
- Inferências:
  - As operações públicas sem prova dedicada representam blind spots reais porque contêm mapeamento manual de campos e wrapping de erro.
- Não evidenciado:
  - Não foi encontrada ocorrência textual de `GetCardInvoice`, `SearchTransactions` ou `CreateRecurringTemplate` em `transactions_ledger_adapter_test.go` nem em `transactions_integration_test.go`.

## Notas de Validação
- Esta história foca um recorte de alto risco: adapters com transformação manual e dependência de identidade.
- O escopo foi dividido do `write_ledger_repository` porque os riscos e doubles são diferentes e cabem em uma fatia de sprint independente.

# US-03: Sincronizar a prova de cobertura das tools com o inventário real do módulo

## Declaração
Como mantenedor do módulo `internal  /  agents`, quero que o harness de cobertura das tools derive do inventário real montado no módulo, para que a suite não declare “cobertura total” com contagem desatualizada ou incompleta quando novas tools forem adicionadas.

## Contexto
- Problema: o módulo monta `23` tool handles em `buildFinancialTools` (`internal  /  agents / module.go:289-323`) e possui um teste unitário que fixa esse total em `23` (`internal  /  agents / module_test.go:42-44`). Já o scorer `realllm` registra `29` cenários com a mensagem “`22 tools existentes + 7 cenários C1-C7`” (`internal  /  agents / application / scorers / mecontrola_tools_realllm_test.go:274-306`). Mesmo quando o conjunto atual de inputs parece cobrir as tools principais, a fonte de verdade da cobertura está dividida entre contagem fixa no módulo e contagem textual no harness.
- Resultado esperado: qualquer divergência entre inventário real de tools e harness de prova deve falhar automaticamente, preferencialmente por derivação programática dos IDs ou por uma asserção única centralizada.
- Fonte: leitura do composition root do módulo, do teste unitário do inventário e do scorer `realllm` nesta sessão.

## Regras de Negócio
- A fonte de verdade do inventário de tools deve ser única e verificável pelo teste.
- Ao adicionar ou remover tool do módulo, a suite deve falhar se o harness não for atualizado no mesmo diff.
- O harness não pode depender apenas de comentário / mensagem textual para afirmar cobertura total.
- Cobertura de tool deve distinguir “tool existente no módulo” de “cenário adicional de roteamento”, para evitar falso positivo de completude.

## Critérios de Aceite
```gherkin
Cenário: Inventário real de tools e harness ficam sincronizados
  Dado o conjunto de tools retornado por `buildFinancialTools`
  Quando a suite de cobertura for executada
  Então ela deve comprovar que cada `tool.ID()` existente no módulo possui ao menos um cenário correspondente no harness
  E a contagem total não pode depender de número hardcoded divergente

Cenário: Nova tool adicionada sem cenário correspondente
  Dado que uma nova tool foi incluída no módulo
  Quando o harness não tiver caso para esse `tool.ID()`
  Então a suite deve falhar explicitamente apontando a tool descoberta sem cobertura

Cenário: Cenário adicional não mascara cobertura incompleta
  Dado cenários compostos ou extras de roteamento
  Quando a suite computar completude
  Então ela deve separar cobertura de inventário e cenários complementares
  E não pode declarar “todas as tools cobertas” apenas porque o total de cenários atingiu um número esperado
```

## Dados e Permissões
- Dados obrigatórios: lista de `tool.ToolHandle` retornada por `buildFinancialTools`, IDs das tools e mapa de cenários do harness.
- Perfis / permissões: sem permissão de negócio; escopo técnico interno ao módulo.

## Dependências
- Depende de `buildFinancialTools` em `internal  /  agents / module.go`.
- Depende do harness `internal  /  agents / application / scorers / mecontrola_tools_realllm_test.go`.
- Pode reutilizar helpers de construção de tools fake já existentes no próprio scorer.

## Fora de Escopo
- Não inclui redefinir o comportamento funcional das tools.
- Não inclui trocar o provider OpenRouter ou remover o scorer `realllm`.
- Não inclui discussão de produto sobre quais intenções o agente deve suportar; o foco é a prova de cobertura do inventário atual.

## Evidências
- Entrada:
  - Pedido do usuário com foco em verificar se os testes “realmente testam o que prometem testar”.
- Base de código:
  - `internal  /  agents / module_test.go:42-44` — teste fixa `23` tools no módulo.
  - `internal  /  agents / module.go:289-323` — composição real de `23` tool handles.
  - `internal  /  agents / application / scorers / mecontrola_tools_realllm_test.go:25-30` — harness depende de `RUN_REAL_LLM` e `OPENROUTER_API_KEY`.
  - `internal  /  agents / application / scorers / mecontrola_tools_realllm_test.go:274-306` — lista de cenários e mensagem de contagem “22 tools existentes + 7 cenários C1-C7”.
- Inferências:
  - Contagem textual divergente da fonte de verdade do módulo é um sinal de drift de teste, mesmo quando parte dos cenários ainda passa.
- Não evidenciado:
  - Não foi encontrada asserção única no harness que derive automaticamente o conjunto de IDs a partir de `buildFinancialTools`.

## Notas de Validação
- Esta história reduz falso positivo de cobertura declarada, que é exatamente o tipo de desvio de fluxo de qualidade solicitado pelo usuário.
- O resultado observável é um harness que falha por drift estrutural, não apenas por regressão de NLP / LLM.

# US-04: Dar prova offline aos invariantes críticos hoje presos a suites `integration` e `realllm`

## Declaração
Como mantenedor do módulo `internal  /  agents`, quero que invariantes críticos do agente tenham uma camada de prova offline e reproduzível além das suites `integration / realllm`, para que robustez de parsing, roteamento e guardrails não fique invisível quando `RUN_REAL_LLM` ou `OPENROUTER_API_KEY` não estiverem disponíveis.

## Contexto
- Problema: várias provas críticas do módulo estão acopladas a suites `integration` com skip por ambiente. O provider real do agente usa `t.Skip("RUN_REAL_LLM=1 e OPENROUTER_API_KEY obrigatórios")` em `internal  /  agents / application / agents / mecontrola_agent_realllm_test.go:26-31` e o harness de tools faz o mesmo em `internal  /  agents / application / scorers / mecontrola_tools_realllm_test.go:25-30`. Há fluxos relevantes cuja prova visível está concentrada nessas suites, como o gate de extração combinada de objetivo+valor do onboarding em `internal  /  agents / application / agents / onboarding_goal_value_realllm_test.go:18-40`.
- Resultado esperado: invariantes críticos devem ter uma camada de teste determinística que falhe sem rede e sem chave externa, deixando as suites `realllm` como prova complementar de aderência ao provider real.
- Fonte: leitura das suites `realllm` do módulo e execução local da suite padrão nesta sessão.

## Regras de Negócio
- Testes `realllm` continuam úteis para smoke / integration com provider real, mas não podem ser a única prova de invariantes críticos de produto.
- Invariante crítico, para esta história, inclui pelo menos: extração de meta / valor no onboarding, honestidade em erro de tool e roteamento mínimo das tools financeiras principais.
- A camada offline deve validar contratos estáveis: schema, decisão de fluxo, serialização e guardrails textuais determinísticos.
- A suite padrão do módulo deve continuar verde sem segredos externos e ainda assim exercer esses invariantes.

## Critérios de Aceite
```gherkin
Cenário: Invariante crítico é provado sem provider real
  Dado um teste offline com provider fake ou fixtures determinísticas
  Quando o fluxo crítico for executado
  Então a suite deve comprovar o contrato esperado sem depender de rede ou variável `OPENROUTER_API_KEY`
  E a regressão deve falhar no `go test . / internal  /  agents / ...` padrão

Cenário: Suite realllm permanece complementar
  Dado que `RUN_REAL_LLM` não está ativo
  Quando a suite padrão do módulo for executada
  Então os testes críticos continuam exercitados por camada offline
  E os testes `realllm` podem ser pulados sem esconder regressão de contrato

Cenário: Guardrail textual ou de schema é alterado indevidamente
  Dado um fluxo crítico como onboarding de meta / valor ou honestidade em falha de tool
  Quando o comportamento sair do contrato estabelecido
  Então um teste offline deve falhar antes da execução das suites `integration / realllm`
```

## Dados e Permissões
- Dados obrigatórios: fixtures determinísticas de prompt / resposta, provider fake ou stubs equivalentes, cenários críticos já mapeados pelas suites `realllm`.
- Perfis / permissões: sem permissão de negócio; escopo técnico do módulo.

## Dependências
- Depende do builder real / fake do agente em `internal  /  agents / application / agents`.
- Depende das suites `realllm` atuais como fonte de cenários a serem espelhados deterministicamente.
- Pode reutilizar estruturas já existentes de harness e de workflow para evitar duplicação desnecessária.

## Fora de Escopo
- Não inclui remover os testes `realllm`.
- Não inclui trocar o provider OpenRouter ou redesenhar a arquitetura agentiva do módulo.
- Não inclui provar offline a cobertura integral de comportamentos possíveis do LLM; o foco é o subconjunto crítico de invariantes de produto.

## Evidências
- Entrada:
  - Pedido do usuário com exigência de robustez e confiabilidade “production-ready / proof de forma inegociável”.
- Base de código:
  - `internal  /  agents / application / agents / mecontrola_agent_realllm_test.go:26-31` — suite depende de `RUN_REAL_LLM` e `OPENROUTER_API_KEY`, senão faz `Skip`.
  - `internal  /  agents / application / scorers / mecontrola_tools_realllm_test.go:25-30` — harness de tools também depende do mesmo gate de ambiente.
  - `internal  /  agents / application / agents / onboarding_goal_value_realllm_test.go:18-40` — prova atual de extração combinada de meta e valor está em suite `integration / realllm`.
  - `internal  /  agents / application / agents / pending_entry_harness_test.go` e `internal  /  agents / application / workflows / pending_entry_decisions_test.go` — o módulo já possui precedente de prova determinística forte em fluxos diferentes, mostrando que a abordagem é compatível com a arquitetura existente.
- Inferências:
  - Onde já existe harness determinístico forte, a robustez observável do módulo é superior; replicar esse padrão para invariantes críticos hoje presos ao provider real reduz risco de regressão invisível.
- Não evidenciado:
  - Não foi encontrada, nesta análise, uma camada offline equivalente para o gate de `OnboardingGoalValue_CombinedExtractionGate`.

## Notas de Validação
- Esta história não afirma que os testes `realllm` são inválidos; afirma que eles são insuficientes como única prova de invariantes críticos no fluxo padrão.
- O recorte foi mantido deliberadamente pequeno e verificável: prova offline dos invariantes mais sensíveis antes de qualquer ambição de cobertura total de comportamento emergente do LLM.

## Apêndice: Inventário Completo dos `_test.go` de `internal / agents`

Cobertura desta auditoria: 46 arquivos `_test.go` identificados no módulo.

Legenda da classificação usada abaixo:
- `forte`: o arquivo evidencia happy path + alternativa + erro ou edge cases relevantes no próprio arquivo.
- `parcial`: o arquivo prova uma parte importante do contrato, mas deixa operações públicas, ramos de erro ou comportamento estrutural sem prova suficiente no próprio recorte.
- `dependente de ambiente`: a prova existe, mas depende de `integration`, container ou `RUN_REAL_LLM` / `OPENROUTER_API_KEY`, portanto não protege o fluxo padrão sozinho.

### application / agents
- `application / agents / ca03_honest_confirmation_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 1 cenário focado em honestidade de resposta quando tool falha.
  Leitura da auditoria: útil como guarda de regressão comportamental, mas não roda no baseline padrão.
- `application / agents / mecontrola_agent_chain_realllm_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 4 cenários de chain real (`ResolveClassifyRegister`, gate de confirmação, consulta de fatura, última transação).
  Leitura da auditoria: cobre encadeamento real, porém depende de provider externo.
- `application / agents / mecontrola_agent_e2e_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 2 cenários E2E de persistência e não duplicação.
  Leitura da auditoria: forte como prova integrada; não substitui camada offline.
- `application / agents / mecontrola_agent_realllm_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 5 cenários e `t.Skip` condicionado a `RUN_REAL_LLM=1` e `OPENROUTER_API_KEY`.
  Leitura da auditoria: cobre tool calling, scorer, formatação de onboarding e honestidade em erro, mas fica invisível no baseline sem credenciais.
- `application / agents / mecontrola_agent_test.go`
  Classificação: `forte`.
  Evidência textual: builder, instruções, orçamento de tokens e truncation guard de onboarding.
  Leitura da auditoria: boa prova estrutural do agente e de invariantes estáveis sem depender de provider real.
- `application / agents / onboarding_goal_value_realllm_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 1 gate amplo de extração combinada de meta e valor.
  Leitura da auditoria: cenário crítico hoje preso a `integration / realllm`.
- `application / agents / onboarding_methodology_realllm_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 1 cenário de parsing da abordagem de alocação e confirmação de percentuais/valores.
  Leitura da auditoria: valioso para aderência do provider, mas não protege o baseline sozinho.
- `application / agents / pending_entry_decision_g1g6_test.go`
  Classificação: `forte`.
  Evidência textual: 19 testes cobrindo cancelamento, replace, expiração, reprompt, confirmação, escolha de categoria e pares G1-G6.
  Leitura da auditoria: boa prova determinística das decisões puras.
- `application / agents / pending_entry_harness_test.go`
  Classificação: `forte`.
  Evidência textual: 31 testes com fluxo completo, replay, ambiguidades, cartões, datas, recorrência e antissimulação.
  Leitura da auditoria: um dos arquivos mais fortes do módulo para robustez de produção.
- `application / agents / pending_entry_realllm_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 13 cenários de clareza, honestidade, data relativa, receitas, múltiplos itens e cancelamento.
  Leitura da auditoria: forte como smoke real; insuficiente como única prova.
- `application / agents / scoring_hooks_test.go`
  Classificação: `forte`.
  Evidência textual: observa run com tool calls, pula em erro, pula sem `run_id`, pula tool call inválida.
  Leitura da auditoria: bom recorte de happy path e erros de observabilidade.

### application / scorers
- `application / scorers / mecontrola_scorers_test.go`
  Classificação: `forte`.
  Evidência textual: accuracy, completeness, categorization, expected tool, kind e cobertura declarada de tools.
  Leitura da auditoria: boa camada offline para scorer code-based; ainda sofre drift de inventário vs harness real.
- `application / scorers / mecontrola_tools_realllm_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 3 testes, incluindo cobertura declarada de tools e gates EP-01 / EP-05.
  Leitura da auditoria: importante, mas com drift textual identificado entre “22 tools” e `23` tools reais do módulo.

### application / tools
- `application / tools / classify_category_test.go`
  Classificação: `parcial`.
  Evidência textual: 1 suite focada em classificação.
  Leitura da auditoria: prova funcional existe, mas o arquivo é estreito se comparado ao papel da tool; depende de `financial_tools_test.go` para completar ramos.
- `application / tools / create_recurrence_test.go`
  Classificação: `forte`.
  Evidência textual: pending sem escrita síncrona, subcategoria ausente, `userID` inválido e erro do registrar.
  Leitura da auditoria: happy path e erros relevantes presentes.
- `application / tools / financial_tools_test.go`
  Classificação: `forte`.
  Evidência textual: 34 testes cobrindo replay, clarify, identidade ausente, delegate error, cartão, defaults, confirmation gate, already exists, ceilings e outcome routing.
  Leitura da auditoria: arquivo forte e central para robustez das tools de mutação.
- `application / tools / read_tools_test.go`
  Classificação: `forte`.
  Evidência textual: 31 testes cobrindo sucesso, erro de binding, identidade ausente, defaults e campos opcionais.
  Leitura da auditoria: bom recorte para tools de leitura.
- `application / tools / register_expense_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: 1 cenário de identity injection e abertura de pendência.
  Leitura da auditoria: útil como integração, não como prova suficiente isolada.
- `application / tools / validate_entry_amount_test.go`
  Classificação: `parcial`.
  Evidência textual: 1 função de teste agregada.
  Leitura da auditoria: o nome indica validação importante de fronteira; a prova existe, mas o arquivo deveria ser tratado como componente de baixo nível que merece revisão contínua sempre que regras monetárias mudarem.

### application / usecases
- `application / usecases / destructive_confirm_continuer_test.go`
  Classificação: `parcial`.
  Evidência textual: 1 `TestContinue`.
  Leitura da auditoria: prova principal existe, mas o nome agregado sugere que vários ramos podem estar concentrados em um único teste amplo.
- `application / usecases / handle_inbound_test.go`
  Classificação: `parcial`.
  Evidência textual: 1 `TestExecute`.
  Leitura da auditoria: mesmo padrão de concentração; pede atenção para ramos alternativos e falhas de composição.
- `application / usecases / idempotent_write_test.go`
  Classificação: `forte`.
  Evidência textual: `TestExecute` amplo e `TestNoAdvisoryLockRequired`.
  Leitura da auditoria: somado aos cenários de integração, forma prova robusta de idempotência.
- `application / usecases / pending_entry_continuer_test.go`
  Classificação: `parcial`.
  Evidência textual: 1 `TestContinue`.
  Leitura da auditoria: importante, mas o arquivo isoladamente não demonstra a mesma granularidade que os workflows relacionados.
- `application / usecases / register_attempt_test.go`
  Classificação: `forte`.
  Evidência textual: 13 cenários cobrindo clarify, confirmação, cartão ausente, erro do engine, recorrência, edição e propagação de `itemSeq`.
  Leitura da auditoria: boa prova do orchestration layer.
- `application / usecases / resolve_onboarding_or_agent_test.go`
  Classificação: `forte`.
  Evidência textual: `TestExecute` e `TestStartOnboarding`.
  Leitura da auditoria: apesar de dois entrypoints, o escopo lido sugere suite ampla do roteador de onboarding vs agente.

### application / workflows
- `application / workflows / category_resolution_test.go`
  Classificação: `forte`.
  Evidência textual: sucesso, múltiplos candidatos, root-only, rejeições, erro de busca, correção de descrição e rendering de lista.
  Leitura da auditoria: boa prova de edge cases de resolução categórica.
- `application / workflows / destructive_confirm_workflow_test.go`
  Classificação: `forte`.
  Evidência textual: mais de 30 cenários cobrindo confirmação, cancelamento, TTL, cleanup, delete card, edit entry, recurrence, update card e ambiguidades.
  Leitura da auditoria: arquivo robusto e com forte sinal production-ready.
- `application / workflows / onboarding_workflow_test.go`
  Classificação: `forte`.
  Evidência textual: decisões puras, schemas, prompts, merge-patch, steps, persistência com e sem valor e mensagem final.
  Leitura da auditoria: a camada offline de onboarding é forte; ainda assim há invariantes complementares hoje presos ao `realllm`.
- `application / workflows / pending_entry_card_test.go`
  Classificação: `forte`.
  Evidência textual: resolução, not found, max reprompts, cancelamento e fluxo completo.
  Leitura da auditoria: bom recorte determinístico do slot de cartão.
- `application / workflows / pending_entry_confirm_summary_test.go`
  Classificação: `forte`.
  Evidência textual: labels de data, pix, income, crédito parcelado, à vista e prefixo.
  Leitura da auditoria: boa prova de rendering sem dependência externa.
- `application / workflows / pending_entry_decisions_test.go`
  Classificação: `forte`.
  Evidência textual: replace, cancelamento, reprompt, replay, datas e payment methods conhecidos.
  Leitura da auditoria: boa malha de decisão pura.
- `application / workflows / pending_entry_state_test.go`
  Classificação: `forte`.
  Evidência textual: round-trip e inválidos para tipos fechados.
  Leitura da auditoria: pequena, porém alinhada ao contrato state-as-type.
- `application / workflows / pending_entry_workflow_test.go`
  Classificação: `forte`.
  Evidência textual: start, resume, cancelamentos, confirmação, isolamento por thread, merge-patch, reaper e slot de payment method.
  Leitura da auditoria: suite forte do workflow durável.
- `application / workflows / transactions_ledger_pending_test.go`
  Classificação: `forte`.
  Evidência textual: create transaction, income, replay, ledger error, nil ID, edit entry, recorrência, root sem folha e antissimulação.
  Leitura da auditoria: forte para a borda entre workflow e ledger.

### infrastructure / binding
- `infrastructure / binding / ca09_reconciled_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: concorrência e mapeamento de `reconciled`.
  Leitura da auditoria: bom cenário de integração para race/idempotência.
- `infrastructure / binding / card_manager_adapter_test.go`
  Classificação: `parcial`.
  Evidência textual: 1 cenário de conflito de nickname.
  Leitura da auditoria: o arquivo prova um erro importante, mas fica estreito frente ao número de operações do adapter.
- `infrastructure / binding / categories_reader_adapter_test.go`
  Classificação: `forte`.
  Evidência textual: kind inválido, preservação de campos, resolve success, invalid kind e repo error.
  Leitura da auditoria: boa cobertura do adapter.
- `infrastructure / binding / pending_entry_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: start-resume-write, expiração, substituição e cancelamento.
  Leitura da auditoria: bom smoke integrado do fluxo durável.
- `infrastructure / binding / recurrence_manager_adapter_test.go`
  Classificação: `parcial`.
  Evidência textual: 3 testes focados só em `principalCtx`.
  Leitura da auditoria: útil, porém não prova todas as operações públicas do adapter.
- `infrastructure / binding / transactions_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: resumo mensal sem dupla contagem, idempotência, origin ref e parcelado.
  Leitura da auditoria: forte como integração, mas não cobre todas as operações públicas do adapter.
- `infrastructure / binding / transactions_ledger_adapter_test.go`
  Classificação: `parcial`.
  Evidência textual: cobre `GetMonthlySummary` e `ListMonthlyEntries`.
  Leitura da auditoria: principal blind spot do binding; faltam provas explícitas para `GetCardInvoice`, `SearchTransactions`, `CreateRecurringTemplate` e parte do contrato de `GetTransaction`.

### infrastructure / messaging / database / consumers
- `infrastructure / messaging / database / consumers / subscription_bound_welcome_consumer_test.go`
  Classificação: `forte`.
  Evidência textual: dedup insertido vs já processado, handled false, falha de envio, falha de onboarding e payload incompleto.
  Leitura da auditoria: boa malha de sucesso, alternativa e compensação.
- `infrastructure / messaging / database / consumers / whatsapp_inbound_consumer_test.go`
  Classificação: `forte`.
  Evidência textual: `TestHandle` amplo, timeout cancelando chamada LLM, restauração de traceparent e ordering de pending entry.
  Leitura da auditoria: um dos arquivos fortes da camada de consumers.

### infrastructure / persistence
- `infrastructure / persistence / write_ledger_repository_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: insert/find, not found, on conflict, concorrência, múltiplos `itemSeq` e `DeleteBefore`.
  Leitura da auditoria: bom conjunto integrado, mas ainda sem camada unitária no baseline padrão.

### módulo
- `module_boot_integration_test.go`
  Classificação: `dependente de ambiente`.
  Evidência textual: boot do composition root e falha sem chave LLM.
  Leitura da auditoria: bom teste de montagem real do módulo.
- `module_test.go`
  Classificação: `parcial`.
  Evidência textual: inventário de tools, validação de deps, rota WhatsApp e adapter de idempotência.
  Leitura da auditoria: forte em partes do composition root, mas revelou drift de contagem entre `23` tools reais e harness `realllm` que ainda fala em `22 tools`.

### Síntese do inventário completo
- Arquivos com prova determinística forte no próprio recorte:
  `mecontrola_agent_test.go`, `pending_entry_decision_g1g6_test.go`, `pending_entry_harness_test.go`, `scoring_hooks_test.go`, `mecontrola_scorers_test.go`, `create_recurrence_test.go`, `financial_tools_test.go`, `read_tools_test.go`, `register_attempt_test.go`, `resolve_onboarding_or_agent_test.go`, `category_resolution_test.go`, `destructive_confirm_workflow_test.go`, `onboarding_workflow_test.go`, `pending_entry_card_test.go`, `pending_entry_confirm_summary_test.go`, `pending_entry_decisions_test.go`, `pending_entry_state_test.go`, `pending_entry_workflow_test.go`, `transactions_ledger_pending_test.go`, `categories_reader_adapter_test.go`, `subscription_bound_welcome_consumer_test.go`, `whatsapp_inbound_consumer_test.go`.
- Arquivos com prova parcial e necessidade de endurecimento:
  `classify_category_test.go`, `validate_entry_amount_test.go`, `destructive_confirm_continuer_test.go`, `handle_inbound_test.go`, `pending_entry_continuer_test.go`, `card_manager_adapter_test.go`, `recurrence_manager_adapter_test.go`, `transactions_ledger_adapter_test.go`, `module_test.go`.
- Arquivos fortes, mas dependentes de `integration` ou `realllm`:
  `ca03_honest_confirmation_integration_test.go`, `mecontrola_agent_chain_realllm_test.go`, `mecontrola_agent_e2e_test.go`, `mecontrola_agent_realllm_test.go`, `onboarding_goal_value_realllm_test.go`, `onboarding_methodology_realllm_test.go`, `pending_entry_realllm_test.go`, `mecontrola_tools_realllm_test.go`, `register_expense_integration_test.go`, `ca09_reconciled_integration_test.go`, `pending_entry_integration_test.go`, `transactions_integration_test.go`, `write_ledger_repository_integration_test.go`, `module_boot_integration_test.go`.

### Conclusão operacional da auditoria 100%
- Nenhum `_test.go` do módulo ficou fora do inventário desta análise.
- O módulo tem núcleos muito fortes de prova determinística em workflows, pending entry, tools de mutação/leitura e consumers.
- As lacunas centrais permanecem concentradas em três frentes:
  `infrastructure / jobs / handlers` sem `_test.go`,
  `infrastructure / persistence / write_ledger_repository` sem camada unitária no baseline padrão,
  e adapters / harnesses com drift ou cobertura parcial (`transactions_ledger_adapter_test.go`, `card_manager_adapter_test.go`, `recurrence_manager_adapter_test.go`, `module_test.go` vs harness de tools).

### Rechecagem com skills obrigatórias
- `go-implementation`
  Evidência aplicada: leitura de `architecture.md` e de `go.mod`.
  Achado relevante: `go.mod` declara `go 1.26.4`.
  Drift registrado: o script citado pela skill, `scripts / verify-go-mod.sh`, não existe no workspace nesta sessão; a auditoria registrou a ausência em vez de inventar substituto silencioso.
- `mastra`
  Evidência aplicada: leitura de `core-concepts.md`.
  Impacto na auditoria: o documento foi mantido ancorado no substrato real `internal / platform / {agent,llm,memory,workflow,tool,scorer}` e no consumidor `internal / agents`, distinguindo claramente lacunas de workflow durável, tool adapter fino, Thread -> Run e scorer / eval.
- `domain-modeling-production`
  Evidência aplicada: uso dos princípios de estados, comandos, eventos, invariantes e fronteiras para não reduzir a auditoria a contagem de arquivos.
  Impacto na auditoria: os pontos críticos foram modelados por comportamento e contrato, especialmente onboarding, destructive confirmation, pending entry, idempotent write e runtime agentivo.
- `design-patterns-mandatory`
  Evidência aplicada: execução do seletor ` .agents / skills / design-patterns-mandatory / scripts / select_pattern.py`.
  Resultado observado: `status = needs_more_evidence`, sem pattern aprovado.
  Decisão operacional desta auditoria: não recomendar novo design pattern para o endurecimento dos testes; a direção correta aqui é reforço de prova, sincronização de harness e cobertura determinística, não nova abstração estrutural.
