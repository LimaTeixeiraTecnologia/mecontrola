# 2026-07-06 — Prompt enriquecido para PRD de tool robusta de categorias entre `internal/agents` e `internal/transactions`

## Prompt original

> Eu quero que use $go-implementation, $mastra e Domain Modeling Made Functional: Tackle Software Complexity with Domain-Driven Design and F# para criar/melhorar uma tool com base em: `internal/categories` com 0 gaps, 0 lacunas, 0 falso positivo, 0 flexibilidade, 0 desvios uma tool extremamente robusta para uso correto das categorias entre os módulos: `internal/agents` e `internal/transactions`.
>
> O output desse prompt deve ser o input para `$create-prd`.
>
> NÃO IMPLEMENTE NADA, APENAS CRIE O PROMPT.

## Ambiguidades que o prompt enriquecido elimina

- `criar/melhorar uma tool` estava aberto demais: no estado atual existem integrações distintas entre categorias, agent tools e validação transacional, então o prompt passa a exigir diagnóstico do codebase antes de propor qualquer escopo.
- `0 gaps / 0 lacunas / 0 falso positivo / 0 flexibilidade / 0 desvios` não pode ser tratado como promessa cega; o prompt converte isso em regra operacional: toda dúvida material deve virar pergunta obrigatória antes da redação final.
- O pedido mistura referência técnica (`$go-implementation`, `$mastra`, DMMF) com um objetivo que precisa virar insumo de produto para `$create-prd`; o prompt agora separa claramente `o que investigar`, `o que decidir` e `como estruturar a saída`.
- A fronteira entre `internal/agents` e `internal/transactions` precisava ser ancorada em símbolos reais já existentes, como `classify_category`, `register_entry`, `CategoriesReader`, `CategoryValidator`, `CategoriesCache` e adapters para `internal/categories`.

## Prompt enriquecido — versão pronta para uso

```text
Atue como um analista sênior de produto e arquitetura funcional, extremamente rigoroso, direto, mandatório e incapaz de assumir contexto ausente.

Sua missão é produzir uma solicitação de funcionalidade em PT-BR, pronta para ser usada como input da skill `$create-prd`, para definir a criação ou evolução de uma tool extremamente robusta de categorias entre os módulos `internal/agents` e `internal/transactions`, usando `internal/categories` como fonte canônica.

Mandatos inegociáveis:
1. NÃO implemente nada.
2. NÃO escreva código, diff, pseudo-código, migrations, wiring, handlers finais ou desenho técnico detalhado como resposta principal.
3. O seu output final deve ser um briefing de produto/funcionalidade pronto para ser colado em `$create-prd`.
4. Você DEVE usar o codebase atual como fonte da verdade.
5. Você DEVE considerar explicitamente como lentes de análise:
   - `$go-implementation`
   - `$mastra`
   - os princípios de Domain Modeling Made Functional: state-as-type, smart constructors, decide puro, workflow pipeline e eliminação de ambiguidade semântica
6. Essas lentes NÃO autorizam implementação. Elas servem para melhorar a qualidade da definição da feature.
7. Se houver qualquer lacuna material, ambiguidade, conflito de escopo ou dúvida de semântica, você DEVE parar e perguntar.
8. Toda pergunta deve vir com recomendação explícita.
9. Faça uma pergunta por turno quando houver decisão material.
10. Não trate `0 gaps`, `0 lacunas`, `0 falso positivo`, `0 flexibilidade`, `0 desvios` como premissa automaticamente satisfeita. Trate isso como padrão de evidência: se não estiver provado, você deve perguntar ou marcar como pendência.
11. O working tree atual prevalece sobre documentação histórica ou suposições.

Objetivo a transformar em briefing para `$create-prd`:
Definir uma feature para criar ou evoluir uma tool responsável por garantir o uso correto, determinístico, consistente e auditável das categorias entre `internal/agents` e `internal/transactions`, usando `internal/categories` como catálogo e regra canônica, sem lacunas semânticas entre classificação, seleção, validação, persistência e leitura posterior.

Contexto real mínimo do codebase que você DEVE confrontar antes de concluir:
1. Em `internal/agents/application/tools/classify_category.go`, já existe a tool `classify_category`, que usa `CategoriesReader.SearchDictionary(term, kind)` e pode retornar:
   - `candidates`
   - `categoryId`
   - `subcategoryId`
   - `path`
   - `isAmbiguous`
2. Em `internal/agents/application/usecases/register_entry.go`, o fluxo atual:
   - classifica por descrição e `kind`
   - interrompe com `ToolOutcomeClarify` quando não há candidato ou quando há ambiguidade
   - grava `CategoryID` como raiz e `SubcategoryID` como folha ao criar transação
3. Em `internal/agents/infrastructure/binding/categories_reader_adapter.go`, o módulo `agents` já depende dos use cases de `internal/categories` para:
   - `SearchDictionary`
   - `ResolveBySlug`
   - `ListCategories`
4. Em `internal/transactions/application/interfaces/category_validator.go`, o módulo `transactions` já expõe a fronteira `CategoryValidator.Validate(ctx, categoryID, subcategoryID)`.
5. Em `internal/transactions/infrastructure/config/categories_cache.go`, já existe uma política local para:
   - validar se `categoryID` é raiz oficial
   - validar `subcategoryID` contra a raiz esperada
   - cachear subcategorias por versão editorial
6. Em `internal/transactions/infrastructure/repositories/postgres/categories_reader_adapter.go`, `transactions` já consome `internal/categories` para:
   - resolver raízes por slug
   - validar subcategoria por parent esperado
   - ler versão editorial
7. O pedido envolve uma feature nova ou evolutiva na fronteira agentiva, portanto você DEVE considerar o substrato e as regras da skill `$mastra`, mas sem reimplementar primitivos de `internal/platform/{agent,llm,memory,workflow,tool,scorer}`.
8. O domínio de categorias já possui modelagem própria em `internal/categories`, incluindo entidades, value objects e serviços para busca/classificação.

Arquivos mínimos que você DEVE ler e confrontar antes de responder:
- `AGENTS.md`
- `internal/categories/module.go`
- `internal/categories/domain/entities/category.go`
- `internal/categories/domain/entities/dictionary_entry.go`
- `internal/categories/domain/services/candidate_resolver.go`
- `internal/agents/application/interfaces/categories_reader.go`
- `internal/agents/application/interfaces/types.go`
- `internal/agents/application/tools/classify_category.go`
- `internal/agents/application/tools/list_categories.go`
- `internal/agents/application/usecases/register_entry.go`
- `internal/agents/infrastructure/binding/categories_reader_adapter.go`
- `internal/transactions/application/interfaces/category_validator.go`
- `internal/transactions/application/interfaces/types.go`
- `internal/transactions/infrastructure/config/categories_cache.go`
- `internal/transactions/infrastructure/repositories/postgres/categories_reader_adapter.go`

Perguntas e decisões obrigatórias que você DEVE zerar antes do briefing final:
1. A feature desejada é:
   - evolução da tool `classify_category`
   - criação de uma nova tool de orquestração de categorias
   - ou consolidação de múltiplas responsabilidades hoje dispersas
   Recomende a opção mais coerente com o codebase atual.
2. O problema principal a resolver é:
   - ambiguidade na classificação
   - inconsistência entre categoria escolhida no agent e categoria validada em transactions
   - falta de contrato único entre classificação e persistência
   - baixa auditabilidade/explicabilidade da decisão de categoria
   Você deve identificar a principal dor e as dores secundárias.
3. A tool alvo deve operar em qual momento:
   - apenas antes da escrita
   - durante classificação interativa com o usuário
   - como validação determinística antes da persistência
   - como combinação explícita dessas etapas
   Recomende o fluxo mais seguro.
4. A saída da tool desejada deve representar:
   - apenas uma categoria final
   - ranking de candidatos com explicação
   - categoria final + motivo + grau de confiança + necessidade de esclarecimento
   Recomende a opção mais consistente com DMMF e com o comportamento atual de `ToolOutcomeClarify`.
5. O contrato entre `agents` e `transactions` deve tratar explicitamente:
   - categoria raiz
   - subcategoria folha
   - kind
   - path canônico
   - confiança/ambiguidade
   - versão editorial
   Você deve confirmar o conjunto mínimo obrigatório e o que é opcional.
6. A feature deve cobrir apenas despesas, apenas receitas ou ambos.
7. Quando houver ambiguidade, o comportamento desejado deve ser:
   - bloquear e pedir esclarecimento
   - sugerir opções e exigir confirmação
   - escolher automaticamente acima de um limiar verificável
   Recomende a opção mais segura para evitar falso positivo.
8. O briefing final deve deixar explícito se o objetivo é:
   - reduzir erro de classificação
   - padronizar contrato entre módulos
   - melhorar UX do agent
   - aumentar auditabilidade
   - ou combinar esses resultados com prioridade definida

Regras obrigatórias de análise:
1. Você deve confrontar o pedido com o codebase e começar pelo diagnóstico do estado atual.
2. Você deve dizer claramente:
   - o que já existe hoje
   - o que já funciona
   - onde estão as lacunas
   - onde há duplicidade de responsabilidade
   - o que ainda é decisão de produto e não fato
3. Você deve usar `$go-implementation` como lente para:
   - respeitar fronteiras
   - evitar interfaces fictícias
   - mapear superfícies reais de alteração
   - distinguir adapter fino vs regra de negócio
4. Você deve usar `$mastra` como lente para:
   - verificar se a responsabilidade pertence a tool, workflow, use case ou runtime
   - impedir que a solução proposta reimplemente o substrato agentivo
   - garantir que a tool continue sendo adapter fino quando aplicável
5. Você deve usar DMMF como lente para:
   - exigir estados fechados em vez de strings livres quando isso impactar o contrato funcional
   - exigir semântica explícita para ambiguidade, confiança, categoria resolvida e necessidade de clarificação
   - separar claramente decisão pura, validação e persistência no raciocínio da feature
6. Você não pode inventar novos módulos, novas tabelas, novas rotas ou novos componentes sem antes provar que são necessários para o briefing.

Formato obrigatório da sua resposta:
1. Primeiro, entregue um diagnóstico confrontado com o codebase atual.
2. Depois, se houver qualquer ambiguidade material, faça perguntas obrigatórias, uma por turno, sempre com recomendação.
3. Somente quando todas as ambiguidades materiais estiverem zeradas, entregue o briefing final pronto para `$create-prd`.

Formato obrigatório do briefing final pronto para `$create-prd`:
1. Título da funcionalidade
2. Problema e objetivo
3. Usuário/ator principal
4. Escopo incluído
5. Escopo excluído
6. Restrições e conformidade
7. Critérios de sucesso mensuráveis
8. Requisitos funcionais preliminares numerados
9. Suposições e questões em aberto

Critérios de aceitação do seu trabalho:
1. O output final pode ser colado diretamente como input de `$create-prd`.
2. O texto final está em PT-BR, direto, mandatório e sem floreio.
3. Nenhuma ambiguidade material fica sem pergunta.
4. Nenhuma pergunta vem sem recomendação.
5. O briefing final não implementa a solução.
6. O briefing final deixa explícita a relação entre:
   - `internal/categories` como fonte canônica
   - `internal/agents` como consumidor agentivo/classificador
   - `internal/transactions` como consumidor validador/persistente
7. O briefing final define o problema de produto com precisão suficiente para o próximo passo ser `$create-prd`, não implementação.
8. Se algum ponto não puder ser provado pelo codebase ou pelas respostas do usuário, trate isso como pendência explícita, nunca como fato.
```
