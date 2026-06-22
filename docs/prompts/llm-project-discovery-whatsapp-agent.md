Voce vai executar um discovery tecnico profundo neste repositorio Go para produzir um dossie confiavel sobre a experiencia conversacional do MeControla, com foco na LLM, no fluxo de entrada do WhatsApp, no `internal/agent`, nos prompts existentes e nas funcionalidades reais disponiveis hoje no codigo.

Seu trabalho NAO e implementar nada. Seu trabalho e ler, rastrear, comparar, consolidar e reportar com evidencias. Nao invente fluxos, nao presuma features ausentes, nao complete lacunas com suposicao silenciosa e nao trate docs antigas como verdade se o working tree atual divergir.

## Objetivo central

Produzir um material que sirva como base para:

1. evolucao do system prompt e dos prompts auxiliares da LLM;
2. entendimento completo do fluxo de mensagens WhatsApp ate o roteamento para onboarding ou agent;
3. mapeamento das capacidades reais ja implementadas nos modulos internos;
4. identificacao de gaps entre o que os prompts prometem e o que o codigo realmente suporta;
5. descoberta de oportunidades reais de novas funcionalidades conversacionais sem inventar capacidades que ainda nao existem.

## Regras inegociaveis

1. Trabalhe apenas com fatos observaveis no repositorio atual.
2. Quando fizer inferencia, rotule explicitamente como `Inferencia`.
3. Quando houver duvida nao resolvida, rotule como `Hipotese em aberto`.
4. Cite evidencias com caminho de arquivo e, quando possivel, funcao, tipo, metodo ou template.
5. Nao use nomes conceituais genericos quando o codigo tiver nomes reais.
6. Nao misture comportamento do onboarding com comportamento do agent sem explicar exatamente onde eles se conectam.
7. Diferencie claramente artefatos ativos, artefatos paralelos e artefatos aparentemente legados.
8. Se encontrar duplicidade de prompts, schemas, catalogos de tools ou pipelines, registre isso explicitamente.

## Contexto tecnico obrigatorio do repositorio

- Linguagem principal: Go.
- Arquitetura: monolito modular em `internal/`.
- Modulos reais encontrados:
  - `internal/agent`
  - `internal/billing`
  - `internal/bootstrap`
  - `internal/budgets`
  - `internal/card`
  - `internal/categories`
  - `internal/identity`
  - `internal/onboarding`
  - `internal/platform`
  - `internal/transactions`
- Entrypoint principal relevante para o discovery:
  - `cmd/server/server.go`
  - `cmd/server/whatsapp_wiring.go`

## Trilha obrigatoria de leitura

Voce DEVE inspecionar no minimo estes pontos antes de concluir:

### 1. Fluxo de entrada do WhatsApp

- `cmd/server/server.go`
- `cmd/server/whatsapp_wiring.go`
- `internal/identity/infrastructure/http/server/whatsapp_router.go`
- `internal/platform/whatsapp/handlers/*`
- `internal/platform/whatsapp/dispatcher/*`
- `internal/platform/whatsapp/payload/*`
- `internal/platform/whatsapp/signature/*`
- `internal/platform/whatsapp/ratelimit/*`

Objetivo desta leitura:
- entender como o webhook e registrado;
- entender verificacao, assinatura, rate limit e captura do body;
- rastrear como a mensagem inbound vira `payload.Message`;
- entender deduplicacao por `wamid`;
- entender validacao de timestamp/stale webhook;
- entender quando a mensagem vai para onboarding;
- entender quando a mensagem vai para agent;
- entender quais outcomes existem e como sao metricados.

### 2. Composicao do `internal/agent`

- `internal/agent/module.go`
- `internal/agent/application/interfaces/*`
- `internal/agent/application/services/intent_router.go`
- `internal/agent/application/services/fallback_chain.go`
- `internal/agent/application/services/circuit_breaker.go`
- `internal/agent/application/usecases/parse_inbound.go`
- `internal/agent/application/usecases/compose_conversational_reply.go`
- `internal/agent/application/usecases/run_onboarding_turn.go`
- `internal/agent/application/usecases/tool_catalog.go`
- `internal/agent/application/usecases/onboarding_tool_catalog.go`
- `internal/agent/application/usecases/configure_budget_conversation.go`
- `internal/agent/application/usecases/log_transaction_from_agent.go`
- `internal/agent/application/usecases/log_card_purchase_from_agent.go`
- `internal/agent/application/usecases/create_recurring_from_agent.go`
- `internal/agent/application/usecases/category_resolution.go`
- `internal/agent/application/usecases/onboarding_scripts.go`

Objetivo desta leitura:
- entender como o `AgentModule` e montado;
- identificar dependencias reais do agent com outros modulos;
- entender quais use cases a LLM aciona;
- entender o pipeline parser -> intent -> router -> fallback -> outbound;
- identificar capacidades de leitura e escrita ja suportadas;
- identificar o que e executado por tool call, por intent parseada ou por fallback conversacional.

### 3. Prompts, templates, schemas e contexto de prompt

- `internal/agent/application/prompting/prompts.go`
- `internal/agent/application/prompting/persona.system.tmpl`
- `internal/agent/application/prompting/budgets.system.tmpl`
- `internal/agent/application/prompting/onboarding.system.tmpl`
- `internal/agent/application/prompting/parse_intent.system.tmpl`
- `internal/agent/application/prompting/parse_intent.user.tmpl`
- `internal/agent/infrastructure/loader/prompt_context_loader.go`
- `internal/agent/infrastructure/providers/openrouter/client.go`
- `internal/agent/infrastructure/llm/prompts/parse_intent.system.tmpl`
- `internal/agent/infrastructure/llm/prompts/parse_intent.user.tmpl`

Objetivo desta leitura:
- listar todos os prompts existentes;
- identificar quais prompts sao efetivamente usados pela carga atual;
- identificar prompts paralelos ou duplicados;
- mapear schemas JSON usados para parsing;
- mapear tools e tool definitions enviados ao provider;
- entender o papel do OpenRouter;
- entender como categorias e cartoes entram como contexto semente do prompt.

### 4. Ponte entre agent e onboarding

- `internal/agent/infrastructure/onboarding/*`
- `internal/onboarding/module.go`
- `internal/onboarding/application/services/*message_processor*.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`
- `internal/onboarding/infrastructure/http/client/meta/*`

Objetivo desta leitura:
- entender o que fica no onboarding tradicional e o que foi absorvido pelo agent;
- entender a continuacao de conversa no onboarding;
- entender uso de WhatsApp outbound via gateway Meta;
- entender quais use cases de onboarding sao expostos ao agent;
- entender o que e onboarding conversacional com LLM versus fluxo guiado/scriptado.

### 5. Modulos que o agent toca diretamente

Leia o suficiente para mapear responsabilidades e interfaces reais de:

- `internal/categories`
- `internal/card`
- `internal/budgets`
- `internal/transactions`
- `internal/identity`
- `internal/billing`

Objetivo desta leitura:
- identificar funcionalidades que ja podem ser expostas melhor pela LLM;
- identificar limitacoes reais por modulo;
- entender dependencias usadas pelo `AgentModule`;
- separar capacidades existentes de capacidades apenas sugeridas pelos prompts.

## Perguntas que voce PRECISA responder

### A. Fluxo WhatsApp ponta a ponta

1. Onde o webhook do WhatsApp e registrado?
2. Como a requisicao inbound passa por verify, signature e rate limit?
3. Onde o raw body e capturado e repassado?
4. Como a mensagem e extraida do payload?
5. Como funciona a deduplicacao por `wamid`?
6. Como funciona a validacao de timestamp e stale webhook?
7. Em quais condicoes a mensagem vai para onboarding?
8. Em quais condicoes a mensagem vai para agent?
9. O que acontece quando o usuario ainda nao existe em `identity`?
10. Quais outcomes, metricas e logs existem no dispatcher?

### B. Arquitetura do `internal/agent`

1. Como o `AgentModule` e construido?
2. Quais dependencias de outros modulos sao obrigatorias?
3. Quais dependencias sao opcionais?
4. Quais fluxos passam por parse de intent?
5. Quais fluxos passam por resposta conversacional/fallback?
6. Como a LLM aciona tools?
7. Como o router decide entre responder, consultar, criar, editar ou encaminhar?
8. Como o agent lida com onboarding em andamento?
9. Como o agent conversa com WhatsApp e Telegram?
10. Quais trechos mostram claramente o comportamento atual em producao?

### C. Prompts e comportamento da LLM

1. Quais system prompts existem hoje?
2. Quais sao usados para:
   - parser de intent
   - persona/resposta conversacional
   - budgets persona
   - onboarding
   - tools
3. Existem prompts duplicados, antigos ou em paralelo?
4. Existe divergencia entre prompt, schema e codigo executor?
5. O prompt promete funcionalidades que o codigo nao confirma?
6. O codigo suporta funcionalidades que o prompt ainda nao comunica bem?
7. Onde estao os limites reais do comportamento da LLM hoje?

### D. Funcionalidades reais por modulo

Para cada modulo relevante, responda:
- qual responsabilidade de negocio ele cobre;
- o que o agent/LLM ja usa hoje;
- o que ainda nao esta conectado ao chat;
- quais capacidades merecem ser promovidas no system prompt;
- quais riscos existem ao prometer mais do que o modulo entrega.

## Pontos que voce deve observar explicitamente

### 1. Fluxo hibrido onboarding vs agent

Verifique se o sistema atual opera em modo hibrido, onde:
- o dispatcher pode mandar para onboarding em certos casos;
- o `internal/agent` tambem possui fluxo proprio de onboarding conversacional;
- existem adaptadores ligando onboarding tradicional ao agent.

Se isso se confirmar, descreva exatamente:
- quem decide a entrada;
- quem responde ao usuario;
- em quais fases cada mecanismo atua;
- onde ha sobreposicao ou risco de comportamento duplicado.

### 2. Prompts ativos vs prompts suspeitos de legado

Verifique especialmente a relacao entre:
- `internal/agent/application/prompting/*`
- `internal/agent/infrastructure/llm/prompts/*`

Voce deve concluir, com evidencias:
- o que e claramente usado hoje;
- o que parece mantido por compatibilidade, experimento ou legado;
- quais duplicidades podem confundir a evolucao futura da LLM.

### 3. Intents, tool calls e schemas

Mapeie com precisao:
- intents parseadas;
- campos esperados por intent;
- tools declaradas;
- tools exclusivas de onboarding;
- schemas JSON utilizados;
- divergencias entre enum, template, catalogo e executor real.

### 4. Funcionalidades conversacionais reais ja visiveis

Identifique, no minimo, se o chat ja suporta:
- registrar gasto;
- registrar ganho;
- registrar compra parcelada no cartao;
- listar lancamentos;
- apagar ultimo lancamento;
- editar ultimo lancamento;
- criar recorrencia;
- listar recorrencias;
- listar cartoes;
- criar cartao;
- contar cartoes;
- resumo mensal;
- leitura de saude financeira;
- configuracao de orcamento;
- onboarding conversacional;
- leitura de categorias;
- perguntas sobre metas;
- perguntas sobre cartao/fatura/limite.

Para cada item, classifique:
- `Confirmado no codigo`
- `Parcial / com restricoes`
- `Prometido pelo prompt, mas nao comprovado`
- `Nao encontrado`

## Formato de saida obrigatorio

Sua resposta final DEVE ser um relatorio em Markdown com estas secoes, nesta ordem:

1. `Resumo executivo`
2. `Fluxo ponta a ponta do WhatsApp`
3. `Arquitetura do internal/agent`
4. `Inventario de prompts, templates e schemas`
5. `Catalogo de intents, tools e acoes suportadas`
6. `Capacidades reais por modulo`
7. `Gaps, inconsistencias, duplicidades e drifts`
8. `Oportunidades de evolucao da LLM e do system prompt`
9. `Backlog priorizado de melhorias`
10. `Evidencias`

## Regras para cada secao

### `Resumo executivo`

- Sintetize o estado atual da experiencia conversacional.
- Destaque 5 a 10 achados mais importantes.
- Diferencie rapidamente o que esta claro do que esta confuso.

### `Fluxo ponta a ponta do WhatsApp`

- Descreva a sequencia real de chamadas e componentes.
- Mostre o caminho desde o `cmd/server` ate a resposta/roteamento.
- Inclua decisao entre onboarding e agent.
- Inclua controles tecnicos: verify, signature, raw body, stale timestamp, dedup, rate limit, principal, fallback.

### `Arquitetura do internal/agent`

- Mostre composicao do modulo.
- Liste dependencias por modulo externo.
- Explique pipeline LLM, parser, router, fallback, providers e outbound.
- Explique como onboarding entra dentro do agent.

### `Inventario de prompts, templates e schemas`

- Liste cada prompt/template com funcao, localizacao e status:
  - `ativo`
  - `aparentemente ativo`
  - `paralelo`
  - `suspeita de legado`
- Inclua tabelas comparativas se ajudar.
- Registre inconsistencias de contagem de intents, nomes de campos ou instrucoes.

### `Catalogo de intents, tools e acoes suportadas`

- Liste todas as intents encontradas.
- Liste todas as tools encontradas.
- Relacione intent -> caso de uso -> modulo executor.
- Relacione tool -> caso de uso -> executor -> validacoes/limites.

### `Capacidades reais por modulo`

- Cubra `agent`, `categories`, `card`, `budgets`, `transactions`, `identity`, `onboarding`, `billing`.
- Para cada modulo, diga:
  - responsabilidade principal;
  - integracoes com o chat/agent;
  - funcionalidades observadas;
  - limitacoes relevantes para LLM/system prompt.

### `Gaps, inconsistencias, duplicidades e drifts`

- Compare prompts com codigo executor.
- Compare templates com schemas.
- Compare nomes de intents com contagens descritas em prompts.
- Compare promessas de produto com o que o codigo realmente suporta.
- Destaque duplicidade de fluxos onboarding/agent se houver.

### `Oportunidades de evolucao da LLM e do system prompt`

- Proponha melhorias concretas, priorizadas e aderentes ao codigo atual.
- Separe:
  - melhoria de prompt sem mudar backend;
  - melhoria que exige wiring/adapter/use case;
  - melhoria que exige feature nova de produto.
- Nao proponha nada baseado em feature inexistente sem dizer isso claramente.

### `Backlog priorizado de melhorias`

Monte uma lista priorizada com:
- titulo curto;
- problema atual;
- impacto esperado;
- dependencia tecnica;
- risco;
- tipo:
  - `prompt only`
  - `agent wiring`
  - `backend feature`

### `Evidencias`

- Liste caminhos de arquivo usados.
- Sempre que possivel, cite funcao, metodo, type, const ou template.
- Separe `Fato observado`, `Inferencia` e `Hipotese em aberto`.

## Criterios de aceitacao do seu relatorio

Seu relatorio so sera considerado completo se:

1. cobrir todos os modulos reais em `internal/` que impactam a experiencia conversacional;
2. explicar claramente o fluxo de WhatsApp do entrypoint ate o roteamento final;
3. explicar claramente como `internal/agent` se conecta aos outros modulos;
4. listar e comparar todos os prompts, templates e schemas relevantes;
5. identificar o que esta ativo hoje versus o que parece paralelo ou legado;
6. mapear intents, tools e capacidades reais do chat;
7. mostrar gaps entre prompt e codigo;
8. propor evolucoes praticas para LLM/system prompt;
9. citar evidencias concretas;
10. nao inventar funcionalidades nem mascarar ambiguidades.

## Observacoes finais obrigatorias

- Se o codigo e o prompt divergirem, o codigo atual vale mais.
- Se houver varios caminhos possiveis, mostre qual esta realmente ligado no bootstrap atual.
- Se um comportamento parecer somente testado mas nao necessariamente exposto no runtime principal, registre essa nuance.
- Se uma funcionalidade existir em modulo de negocio mas ainda nao estiver ligada ao agent, nao a promova como capacidade atual do chat.
