# US-003: Limite de 30 interações por dia no assistente de WhatsApp

## Declaração
Como responsável pelo produto mecontrola, quero que cada usuário possa disparar no máximo 30 interações por dia com o assistente de WhatsApp, recebendo uma resposta clara quando atingir o limite, para tornar o custo de LLM por usuário previsível e limitado sem degradar a experiência de quem usa o produto dentro da faixa normal.

## Contexto
- Problema: hoje não existe nenhum teto de uso no fluxo agentivo. Toda mensagem de texto válida recebida pelo consumer `whatsapp_inbound` é despachada para o agente (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:373-394`), que abre um Run em `mecontrola.platform_runs` e executa o loop de tool calling contra o OpenRouter (`internal/platform/agent/runtime.go:100-172`) com até 12 rodadas de ferramenta por mensagem (arquivo `mecontrola_agent.go:290` no diretório de agents do módulo `internal/agents`). Um único usuário pode gerar custo de LLM ilimitado no dia, por abuso, erro de cliente ou loop de reenvio.
- Resultado esperado: a 31ª mensagem elegível do mesmo usuário no mesmo dia, no fuso America/Sao_Paulo, não chama o LLM e recebe uma resposta estática informando o limite e o horário de renovação; as primeiras 30 seguem o fluxo atual sem nenhuma mudança de comportamento; mensagens de retomada de workflow (onboarding e confirmação destrutiva) não são contadas nem bloqueadas.
- Fonte: pedido direto do usuário com limite explícito de 30 interações por dia, documentação oficial de limites do OpenRouter e confronto com o codebase.

## Regras de Negócio
- RN1: o limite é de 30 interações por usuário por dia, contado por `resource_id` (o `user_id` do payload inbound, `whatsapp_inbound_consumer.go:56`), com janela do dia definida pela meia noite no fuso America/Sao_Paulo, reutilizando a configuração de fuso já existente `TRANSACTIONS_BRAZIL_TIMEZONE` com default `America/Sao_Paulo` (`configs/config.go:83`, `configs/config.go:700`) e a localização já injetada no runtime via `agent.WithClockLocation(brazilLoc)` (`internal/agents/module.go:290`).
- RN2: conta como 1 interação cada mensagem inbound válida, de texto ou de áudio já transcrito, que é despachada para o agente. A fonte de contagem é a tabela `mecontrola.platform_runs`, que já registra um Run por dispatch com `resource_id` e `started_at` (`migrations/000001_initial_schema.up.sql:2367-2388`) e já possui o índice `platform_runs_resource_started_idx` sobre `(resource_id, started_at DESC)` (`migrations/000001_initial_schema.up.sql:2390-2391`), portanto nenhuma migration nova é necessária para a contagem.
- RN3: a verificação do limite ocorre antes de qualquer chamada ao LLM e antes da inserção do Run. Mensagem bloqueada não gera Run, não gera custo de LLM e não incrementa a contagem do dia; somente interações efetivamente despachadas consomem a cota. Runs que falham depois de despachados (erro de LLM, timeout, truncamento) consomem cota normalmente, pois o custo já foi incorrido.
- RN4: a fronteira é inclusiva: a 30ª interação do dia é processada normalmente; somente a partir da 31ª a mensagem é bloqueada.
- RN5: o bloqueio responde ao usuário com mensagem estática configurável por variável de ambiente, seguindo o padrão já existente de mensagens `WA_MSG_*` (`configs/config.go:162-170`, `configs/config.go:197-198`), informando o limite de 30 interações e a renovação à meia noite. Nenhuma chamada ao LLM é feita para compor essa resposta.
- RN6: a decisão de limite vive na camada de aplicação de `internal/agents`, como use case resolvedor consultado pelo consumer no mesmo estilo do resolvedor de onboarding (`whatsapp_inbound_consumer.go:347-365`), que retorna se a mensagem foi tratada e qual a resposta. O consumer permanece adapter fino: apenas consulta o resolvedor e envia a resposta, sem regra de negócio, sem SQL e sem branching de domínio, conforme a regra obrigatória de adapters do repositório.
- RN7: zero regressão nos demais fluxos: o caminho de retomada de workflows suspensos (`tryResume` em `whatsapp_inbound_consumer.go:317-345`, cobrindo onboarding e confirmação destrutiva) e o resolvedor de onboarding não são contados nem bloqueados, pois não executam o loop de LLM por mensagem; o pipeline de áudio, a deduplicação por WAMID (`whatsapp_inbound_consumer.go:198-214`) e o envio de respostas permanecem inalterados.
- RN8: o limite é configurável por variável de ambiente com default 30, permitindo ajuste operacional ou desativação sem novo deploy, seguindo o padrão de configuração do módulo de agentes.
- RN9: cada bloqueio por limite é mensurável: contador de métrica com labels de cardinalidade controlada (sem `user_id`), no padrão das métricas já existentes do consumer (`whatsapp_inbound_consumer.go:141-155`), e log estruturado do evento de bloqueio.

## Critérios de Aceite
```gherkin
Cenário: mensagem dentro da cota segue o fluxo normal
  Dado que o usuário já despachou 12 interações ao agente no dia corrente em America/Sao_Paulo
  Quando envia uma nova mensagem de texto válida
  Então a verificação de limite aprova a mensagem
  E o fluxo segue idêntico ao comportamento atual de produção, com Run aberto e resposta do agente

Cenário: trigésima interação do dia é processada
  Dado que o usuário já despachou 29 interações ao agente no dia corrente
  Quando envia a 30ª mensagem válida
  Então a mensagem é despachada normalmente para o agente
  E nenhum aviso de limite é enviado

Cenário: trigésima primeira interação é bloqueada sem custo de LLM
  Dado que o usuário já despachou 30 interações ao agente no dia corrente
  Quando envia a 31ª mensagem válida
  Então nenhum Run é aberto em platform_runs
  E nenhuma chamada ao OpenRouter é realizada
  E o usuário recebe a mensagem estática informando o limite de 30 interações e a renovação à meia noite
  E a métrica de bloqueio por limite é incrementada

Cenário: cota renova na virada do dia no fuso local
  Dado que o usuário atingiu 30 interações no dia anterior
  Quando envia uma nova mensagem após a meia noite em America/Sao_Paulo
  Então a contagem do novo dia inicia em zero
  E a mensagem é processada normalmente pelo agente

Cenário: retomada de workflow suspenso não é bloqueada nem contada
  Dado que o usuário atingiu 30 interações no dia corrente
  E que existe um workflow de confirmação destrutiva suspenso aguardando resposta
  Quando o usuário responde à confirmação pendente
  Então a retomada do workflow ocorre normalmente
  E a resposta não é bloqueada pelo limite nem incrementa a contagem

Cenário: falha na consulta da contagem não derruba o fluxo
  Dado que a verificação de limite não consegue consultar a contagem por indisponibilidade do banco
  Quando o usuário envia uma mensagem válida
  Então o erro é propagado pelo fluxo de erro já existente do consumer, com registro de erro e métrica
  E o comportamento observável é o mesmo de qualquer falha de infraestrutura atual, sem resposta inventada ao usuário
```

## Dados e Permissões
- Dados obrigatórios: `resource_id` da mensagem inbound e `started_at` dos Runs do dia, ambos já persistidos em `mecontrola.platform_runs`; instante corrente no fuso America/Sao_Paulo para calcular o início do dia.
- Perfis/permissões: nenhum perfil novo. O limite se aplica igualmente a cada usuário nesta fatia; diferenciação por plano fica fora de escopo.

## Dependências
- Nenhuma dependência nova de biblioteca ou serviço externo: a contagem reutiliza `mecontrola.platform_runs` e seu índice existente, o fuso reutiliza `TRANSACTIONS_BRAZIL_TIMEZONE` e a resposta reutiliza o padrão de mensagens `WA_MSG_*` e o gateway de envio já injetado no consumer.
- Leitura nova de contagem por recurso e janela: o port `RunStore` atual expõe apenas `Insert`, `Update` e `Load` (`internal/platform/agent/ports.go:153-157`), portanto a contagem exige um novo contrato de leitura declarado pelo consumidor em `internal/agents`, implementado em repositório Postgres, sem alterar o kernel nem o substrato `internal/platform/agent`.
- Documentação oficial do OpenRouter sobre limites ([API Limits](https://openrouter.ai/docs/api-reference/limits)): confirma que o provedor expõe limites diários próprios e erro 429 com headers `X-RateLimit-*`, mas esses limites são da chave da conta, não por usuário final; o teto de 30 por usuário por dia é regra de produto do mecontrola e precisa existir na aplicação.

## Fora de Escopo
- Limites diferenciados por plano ou assinatura (free versus premium), apesar de existirem `PlanCatalog` no billing (`internal/billing/module.go:53`) e `SubscriptionStatus` no identity (`internal/identity/domain/entitlement.go:5-13`); essa integração cross-module é fatia futura.
- Limitação por minuto, por hora ou por rodadas de tool; o gate de 30/dia não substitui nem altera o `WithMaxToolRounds(12)` existente.
- Enfileiramento de mensagens bloqueadas para processamento no dia seguinte.
- Resposta de limite composta por LLM ou personalizada por usuário.
- Alteração no tratamento de erro 429 do OpenRouter dentro do loop do agente.
- Painel ou endpoint de consulta da cota restante pelo usuário.

## Ganhos e Perdas
- Ganhos: custo de LLM por usuário por dia passa a ter teto determinístico de 30 dispatches, cada um limitado a 12 rodadas de tool, o que torna o custo máximo por usuário calculável; o gate ocorre antes do LLM e antes do Run, então mensagem bloqueada tem custo zero de provedor; nenhuma migration nova, pois a contagem reusa tabela e índice já existentes; a resposta estática segue o padrão `WA_MSG_*` já operante; o fuso America/Sao_Paulo já está injetado no runtime, eliminando ambiguidade de virada de dia; o desenho espelha o resolvedor de onboarding, padrão já validado no consumer, o que mantém o adapter fino e reduz risco de regressão; rollback e ajuste operacional por variável de ambiente sem deploy.
- Perdas: usuários legítimos que ultrapassarem 30 interações ficam bloqueados até a meia noite, mudança deliberada de comportamento que precisa de acompanhamento pela métrica de bloqueio; Runs que falham por erro de provedor ainda consomem cota, podendo frustrar o usuário em dia de instabilidade; um áudio que ultrapassa o limite ainda consome custo de transcrição STT antes do gate, perda aceita para manter o pipeline de áudio intacto; a contagem acoplada a `platform_runs` significa que Runs abertos por outros agentes do mesmo `resource_id` contam na mesma cota, comportamento aceito por haver hoje apenas o agente `mecontrola-agent` (`whatsapp_inbound_consumer.go:23`); a virada de dia à meia noite pode cortar uma conversa longa no meio, mitigado pela mensagem que informa o horário de renovação.

## Skills Obrigatórias de Implementação
- `go-implementation`: entrypoint canônico da alteração Go, com classificação `usecase-write` para o novo resolvedor de limite, `repository` para o novo contrato de contagem e validação proporcional dos pacotes `configs`, `internal/agents/application/usecases`, `internal/agents/infrastructure/repositories/postgres` e `internal/agents/infrastructure/messaging/database/consumers`.
- `mastra`: o gate se posiciona no fluxo canônico Thread-Run antes do `AgentRuntime.Execute`, sem reimplementar primitivos de `internal/platform/{agent,memory,workflow}`; estados novos, se houver, seguem tipos fechados com zero value inválido.
- `domain-modeling-production`: aplicação dos princípios DMMF, com a decisão de limite como função pura e determinística (dados contagem, limite e instante corrente, decide permitir ou bloquear), sem IO e sem `context.Context`; não há novo agregado, comando ou evento, portanto sem materialização de bundle.
- `design-patterns-mandatory`: gate de desenho com decisão explícita; a expectativa, dada a natureza de consulta de contagem mais decisão pura mais resposta estática, é `não aplicar padrão`, mantendo código direto sem novas abstrações; o resolvedor segue o padrão estrutural já existente do resolvedor de onboarding, sem introduzir pattern novo.

## Evidências
- Entrada: pedido do usuário para limitar a 30 interações por dia, com foco em zero regressão, production-ready, análise de ganhos e perdas e sem respostas inventadas.
- Base de código: fluxo inbound completo do webhook ao agente em `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:173-231` e `:373-394`; abertura de Run e loop de LLM em `internal/platform/agent/runtime.go:100-172`; máximo de 12 rodadas de tool no arquivo `mecontrola_agent.go:290` do diretório de agents do módulo `internal/agents`; tabela `mecontrola.platform_runs` com índice `(resource_id, started_at DESC)` em `migrations/000001_initial_schema.up.sql:2367-2391`; port `RunStore` sem operação de contagem em `internal/platform/agent/ports.go:153-157`; precedente de resolvedor consultado pelo consumer com resposta estática em `whatsapp_inbound_consumer.go:347-365` e `internal/agents/application/usecases/resolve_onboarding_or_agent.go:17-21`; padrão de mensagens `WA_MSG_*` em `configs/config.go:162-170` e `:197-198`; fuso America/Sao_Paulo com default em `configs/config.go:700` e injeção no runtime em `internal/agents/module.go:290`; rate limiters existentes apenas em HTTP (onboarding, billing webhook, card) e no envio outbound de WhatsApp (`internal/identity/module.go:134`), nenhum no inbound agentivo.
- Documentação oficial: [OpenRouter API Limits](https://openrouter.ai/docs/api-reference/limits), lida na íntegra, confirmando que os limites diários e o erro 429 do provedor são por chave de API e não por usuário final, o que torna o teto de 30 por usuário uma regra de produto do mecontrola; estrutura de história baseada nas regras oficiais da Atlassian para user stories, referenciadas pela skill em `references/regras-atlassian-historias-usuario.md`.
- Inferências: o posicionamento do gate como resolvedor de aplicação consultado pelo consumer antes do dispatch ao agente é direção de design inferida do precedente do onboarding e confirmada como decisão pela rodada de múltipla escolha (resposta estática sem LLM); a exclusão dos fluxos de retomada de workflow da contagem é inferência baseada no fato de que esses fluxos não executam o loop de LLM por mensagem, registrada aqui para confirmação na implementação.
- Não evidenciado: qualquer mecanismo pré-existente de limite, cota ou contagem diária de interações no fluxo agentivo; as buscas por `rate limit`, `quota`, `throttle` e `daily limit` em `internal/` encontraram apenas limiters de HTTP e de envio outbound de WhatsApp, nenhum aplicável ao inbound do agente. Limite por plano de assinatura também não evidenciado como regra existente, apenas como estruturas de billing e identity disponíveis para fatia futura.

## Notas de Validação
- Decisões confirmadas pelo usuário em rodada de múltipla escolha: persona de negócio/produto; interação contada como mensagem inbound válida que dispara o agente; bloqueio com resposta estática sem LLM; limite fixo de 30 para cada usuário com dia em America/Sao_Paulo.
- Documentação oficial consultada: página de limites do OpenRouter, corroborando que o provedor não oferece limite por usuário final e que a regra precisa viver na aplicação.
- Arquivo validado com `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py` com resultado de sucesso.
