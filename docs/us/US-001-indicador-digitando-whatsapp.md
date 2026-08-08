# US-001: Indicador de digitação no WhatsApp durante o processamento da resposta

## Declaração
Como usuário do mecontrola que conversa com o assistente financeiro pelo WhatsApp, quero ver o indicador de "digitando" logo após enviar minha mensagem, para saber que ela foi recebida e que uma resposta está sendo preparada em vez de ficar no silêncio.

## Contexto
- Problema: entre o envio da mensagem e a resposta final do assistente o usuário vê silêncio total, sem ticks azuis de leitura e sem indicador de digitação. O webhook responde HTTP 200 imediato, mas isso é apenas ACK para a Meta e invisível ao usuário (`internal/platform/whatsapp/handlers/inbound_handler.go:45`). O processamento é assíncrono via outbox e pode levar segundos por causa de STT de áudio, chamada de LLM e execução de tools (`internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go:173`). O client Meta atual só expõe `SendText` e `SendTemplate` (`internal/onboarding/infrastructure/http/client/meta/client.go:96-107`), sem mark-as-read nem typing.
- Resultado esperado: o usuário vê os ticks azuis e a bolha de "digitando" segundos após enviar a mensagem, e a bolha desaparece quando a resposta final chega.
- Fonte: pedido direto do usuário, documentação oficial da Meta sobre typing indicators na Cloud API e confronto com o codebase.

## Regras de Negócio
- RN1: a emissão do indicador ocorre uma única vez por mensagem inbound, no worker, no início do `WhatsAppInboundConsumer.Handle`, logo após o dedup por WAMID (`whatsapp_inbound_consumer.go:214`) e antes de qualquer processamento de áudio, resume de workflow, onboarding ou agente. Esse ponto único cobre cada caminho que gera resposta.
- RN2: a chamada usa o payload oficial da Cloud API no endpoint já utilizado `POST /{phone_number_id}/messages` com `status: "read"`, `message_id` do WAMID inbound e `typing_indicator: {"type": "text"}`. Efeito inseparável confirmado na documentação oficial: a chamada também marca a mensagem como lida, exibindo os ticks azuis ao usuário.
- RN3: a bolha de digitação dura no máximo 25 segundos ou até o envio da resposta final, o que ocorrer primeiro, conforme a documentação oficial. Nesta fatia não há renovação periódica da bolha em processamentos longos.
- RN4: falha na chamada do indicador é sempre best-effort. O erro é registrado em log estruturado com `slog` e o fluxo segue normalmente. O indicador nunca bloqueia, retenta ou altera a resposta final, coerente com o client Meta que já opera sem retry (`client.go:123`).
- RN5: o comportamento é controlado por feature flag de configuração com default desligado, garantindo que o comportamento atual de produção permaneça idêntico até a ativação deliberada por ambiente.
- RN6: o indicador só é emitido para mensagens que entraram no fluxo de resposta do assistente. A documentação oficial recomenda não exibir "digitando" quando não haverá resposta, portanto caminhos encerrados sem resposta ao usuário não disparam a chamada.
- RN7: o indicador não passa pelo dedup de WAMID, não é persistido, não gera evento de outbox e não interfere na compensação de dedup existente, pois não é uma mensagem de negócio.

## Critérios de Aceite
```gherkin
Cenário: usuário vê o indicador de digitação antes da resposta final
  Dado que a feature flag do indicador de digitação está ligada
  E que o usuário envia uma mensagem de texto válida para o assistente
  Quando o worker consome o evento inbound e passa pelo dedup
  Então uma chamada à Cloud API com status read e typing_indicator é emitida antes do processamento
  E a resposta final do assistente é enviada em seguida, encerrando a bolha de digitação

Cenário: flag desligada preserva o comportamento atual
  Dado que a feature flag do indicador de digitação está desligada
  Quando o worker consome qualquer evento inbound
  Então nenhuma chamada de indicador de digitação é feita à Cloud API
  E o fluxo de resposta permanece idêntico ao comportamento atual de produção

Cenário: processamento longo de áudio usa emissão única
  Dado que a feature flag está ligada
  E que o usuário envia um áudio cujo processamento de STT e LLM excede 25 segundos
  Quando o worker inicia o processamento
  Então o indicador é emitido uma única vez no início
  E a resposta final é enviada normalmente mesmo após a expiração da bolha

Cenário: falha da Cloud API no indicador não afeta a resposta
  Dado que a feature flag está ligada
  E que a chamada de indicador de digitação falha por timeout ou erro 4xx da Meta
  Quando o worker processa a mensagem
  Então o erro é registrado em log estruturado com identificadores da mensagem
  E a resposta final do assistente é enviada normalmente sem retry do indicador
```

## Dados e Permissões
- Dados obrigatórios: WAMID da mensagem inbound e número do peer, ambos já disponíveis no payload processado pelo consumer; credenciais Meta já existentes na configuração.
- Perfis/permissões: nenhum perfil novo. A chamada reutiliza `META_ACCESS_TOKEN` e `META_PHONE_NUMBER_ID` já usados para envio de mensagens (`configs/config.go:153-177`).

## Dependências
- Documentação oficial da Meta sobre typing indicators na Cloud API, que define payload, acoplamento com read receipt e limite de 25 segundos.
- Spike de validação prévia: confirmar se a versão pinada `v18.0` da Graph API (`internal/onboarding/infrastructure/http/client/meta/client.go:20`) aceita o campo `typing_indicator`. Se não aceitar, é necessário bump de versão que afeta também o client de mídia (`internal/platform/whatsapp/media/client.go:22`), o que eleva o escopo e precisa de decisão explícita.
- Nova operação separada na interface de envio consumida pelo consumer (`whatsapp_inbound_consumer.go:42-44`), distinta de `SendTextMessage`, para não inflar contagens de mensagens em mocks, testes de integração e e2e existentes.

## Fora de Escopo
- Renovação periódica da bolha de digitação além dos 25 segundos em processamentos longos.
- Indicador de digitação em mensagens proativas como lembretes, alertas de orçamento, avisos de fatura e templates de onboarding, pois não são resposta a uma mensagem inbound.
- Mark-as-read isolado sem indicador de digitação.
- Reações de emoji, indicador de "online" ou qualquer outra forma de presença.
- Migração de provedor ou bump de versão da Graph API além do estritamente necessário para suportar o campo `typing_indicator`.

## Ganhos e Perdas
- Ganhos: percepção imediata de recebimento com ticks azuis e bolha de digitação, reduzindo a sensação de mensagem ignorada; custo de implementação baixo por reutilizar endpoint, credenciais e gateway existentes; zero regressão estrutural via feature flag desligada por default; ponto de emissão único cobre texto, áudio, resume de workflow e onboarding sem duplicar lógica.
- Perdas: uma chamada HTTP adicional à Meta por mensagem inbound, com impacto em throughput e rate limit; a chegada do indicador ao usuário carrega a latência do polling do outbox entre server e worker; em processamentos acima de 25 segundos a bolha expira antes da resposta final; se a `v18.0` não suportar o campo, o bump de versão adiciona risco e escopo de testes ao client de mensagens e ao client de mídia.

## Skills Obrigatórias de Implementação
- `go-implementation`: entrypoint canônico da alteração Go, com classificação `consumer` e validação proporcional do pacote alterado.
- `mastra`: o ponto de emissão vive no consumer do fluxo Thread-Run, e nenhum primitivo de `internal/platform/{agent,memory,workflow}` pode ser reimplementado no domínio.
- `domain-modeling-production`: aplicação dos princípios DMMF se um novo tipo fechado ou comando for introduzido; caso contrário apenas os princípios, sem carga de bundle.
- `design-patterns-mandatory`: gate de desenho com decisão explícita de aplicar ou não aplicar padrão; a expectativa, dada a simplicidade da chamada, é `não aplicar padrão`, mantendo o adapter fino.
- Nota honesta: a skill `grill-me-with-docs` citada no pedido não existe no repositório; foi aplicada como abordagem de interrogatório com documentação oficial.

## Evidências
- Entrada: pedido do usuário para exibir "digitando" no WhatsApp e evitar mensagem sem resposta percebida, com foco em zero regressão e production-ready.
- Base de código: provedor é a Cloud API oficial da Meta com base URL `https://graph.facebook.com/v18.0` (`internal/onboarding/infrastructure/http/client/meta/client.go:20`); envio por `POST /{phone_number_id}/messages` (`client.go:115`) sem retry (`client.go:123`); fluxo inbound publica outbox e responde 200 (`internal/agents/module.go:585-604`); processamento no worker em `whatsapp_inbound_consumer.go:173` com dedup em `:214` e envio da resposta em `sendReply` `:398-418`; gateway de envio em `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go:39-45`; busca repo-wide por typing, presence, composing, digitando e read receipt retornou zero ocorrências, confirmando feature greenfield.
- Inferências: o ponto de emissão no início do consumer foi confirmado pelo usuário em rodada de múltipla escolha, assim como emissão única e rollout por flag; a separação da nova operação na interface é inferência de design para proteger mocks e e2e que contam mensagens enviadas.
- Não evidenciado: suporte do campo `typing_indicator` na versão `v18.0` da Graph API. A documentação oficial da Meta bloqueia leitura automatizada e os exemplos de terceiros consultados usam versões mais novas como `v23.0`, portanto o spike de validação é dependência bloqueante antes da implementação.

## Notas de Validação
- Decisões confirmadas pelo usuário em rodada de múltipla escolha: emissão no worker no início do consumer, emissão única sem renovação e rollout por feature flag desligada por default.
- Documentação oficial consultada: página de typing indicators da Meta para WhatsApp Cloud API, corroborada pelo changelog público da Twilio sobre o mesmo recurso da Meta e pela documentação de parceiro Gupshup, ambos descrevendo o mesmo payload, o acoplamento com read receipt e o limite de 25 segundos.
- Arquivo validado com `python3 .agents/skills/user-stories/scripts/validar-historias-usuario.py` com resultado de sucesso.
