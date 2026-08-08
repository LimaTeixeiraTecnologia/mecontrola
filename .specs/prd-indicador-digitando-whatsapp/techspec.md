<!-- spec-hash-prd: 961a35c1e75f094b71fb5ce31358202e9eca1386f336c4f9f1defb254c39f849 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica

Funcionalidade: Indicador de digitação no WhatsApp durante o processamento da resposta
PRD de origem: `.specs/prd-indicador-digitando-whatsapp/prd.md` (spec-version 1)

## Resumo Executivo

A solução adiciona uma operação de indicador de digitação à cadeia de envio Meta já existente (`meta.Client` -> `gateway.WhatsAppGateway` -> consumer) e a dispara de forma síncrona, best-effort e controlada por feature flag no início do `WhatsAppInboundConsumer.Handle`, logo após o dedup por WAMID. Nenhum módulo novo, nenhum padrão GoF novo (seletor determinístico retornou `reject`, registrado em `adr-003`), nenhuma mudança de domínio: a alteração é de infraestrutura e adapter, com contrato de zero regressão garantido por flag com default desligado e por operação separada de `SendTextMessage`, preservando mocks, contagens de e2e e suíte existente.

Decisões arquiteturais chave: emissão síncrona com timeout dedicado curto (`adr-001`), flag de ambiente com default desligado como mecanismo de rollout e rollback (`adr-002`), solução direta sem pattern formal (`adr-003`).

## Arquitetura do Sistema

### Visão Geral dos Componentes

Componentes modificados (nenhum componente novo):

- `internal/onboarding/infrastructure/http/client/meta/client.go`: ganha o método `SendTypingIndicator`, reutilizando `checkStatus` e o `httpclient` com `WithoutRetry()` já existentes.
- `internal/onboarding/infrastructure/http/client/meta/models.go`: ganha os tipos de payload do mark-as-read com typing indicator.
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go`: ganha `SendTypingIndicator`, reutilizando `classifyError`.
- `internal/agents/module.go`: a interface local `whatsAppGateway` (module.go:54-56) ganha o método novo; o módulo passa sender e flag ao consumer via option.
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`: ganha a interface de consumidor `whatsAppTypingIndicatorSender`, a option `WithTypingIndicator`, os campos correspondentes, o contador de métrica e o ponto de emissão em `Handle`.
- `configs/config.go`: `AgentConfig` ganha `WhatsAppTypingIndicatorEnabled` com default `false`, espelhando o precedente `AudioEnabled` (config.go:192, 561, 1440).
- `cmd/worker/worker.go`: o `agents.Deps` ganha o campo da flag, populado a partir de `r.cfg.AgentConfig` (worker.go:432-465).

Fluxo de dados:

```text
evento outbox -> Handle -> valida payload -> dedup InsertIfAbsent
  -> [NOVO] emitTypingIndicator (se flag ligada, best-effort, timeout dedicado)
  -> WithTimeout(inboundTimeout) -> audio/resume/onboarding/agent -> sendReply
```

## Design de Implementação

### Interfaces Chave

Interface definida no consumidor (R6.3), no arquivo do consumer:

```go
type whatsAppTypingIndicatorSender interface {
    SendTypingIndicator(ctx context.Context, wamid string) error
}
```

Option no padrão já existente do consumer (functional options, `ConsumerOption`):

```go
func WithTypingIndicator(sender whatsAppTypingIndicatorSender, enabled bool) ConsumerOption
```

Interface local do módulo agents (module.go:54), estendida com um método:

```go
type whatsAppGateway interface {
    SendTextMessage(ctx context.Context, toE164, text string) error
    SendTypingIndicator(ctx context.Context, wamid string) error
}
```

Gateway de infraestrutura (onboarding), satisfeito pela struct concreta existente:

```go
func (g *WhatsAppGateway) SendTypingIndicator(ctx context.Context, wamid string) error
```

Client Meta:

```go
func (c *Client) SendTypingIndicator(ctx context.Context, wamid string) error
```

Deliberadamente NÃO alterada: a interface `interfaces.WhatsAppGateway` de `internal/onboarding/application/interfaces/whatsapp_gateway.go` não recebe o método, pois nenhum use case de onboarding consome typing (R6.3, interface no consumidor). Consequência direta: o mock gerado em `internal/onboarding/application/interfaces/mocks/whats_app_gateway.go` não muda e nenhuma regeneração de mockery é necessária.

### Modelos de Dados

Novos tipos em `internal/onboarding/infrastructure/http/client/meta/models.go` (payload oficial da Meta, sem campo `to`, conforme contrato de read receipt da Cloud API):

```go
type markAsReadRequest struct {
    MessagingProduct string                  `json:"messaging_product"`
    Status           string                  `json:"status"`
    MessageID        string                  `json:"message_id"`
    TypingIndicator  *typingIndicatorPayload `json:"typing_indicator,omitempty"`
}

type typingIndicatorPayload struct {
    Type string `json:"type"`
}
```

Valores fixos: `MessagingProduct: "whatsapp"`, `Status: "read"`, `Type: "text"`. A resposta da Meta para essa chamada é `{"success": true}` e não contém `messages[]`; por isso o método novo NÃO reutiliza `doSend` (que exige `result.Messages[0].ID`, client.go:148-152) e sim um fluxo próprio que valida apenas o status HTTP via `checkStatus` (client.go:155-172).

Sem migration, sem tabela, sem evento de outbox, sem tipo de domínio novo: a feature não toca `domain/`, portanto `domain-modeling-production` se aplica apenas em princípios (sem bundle), conforme os gatilhos de materialização do AGENTS.md.

### Endpoints de API

Nenhum endpoint inbound novo. Chamada outbound nova:

- `POST {baseURL}/{phone_number_id}/messages` (mesmo endpoint do envio de texto, client.go:115)
- Body: `markAsReadRequest` serializado
- Headers: `Authorization: Bearer <META_ACCESS_TOKEN>`, `Content-Type: application/json` (mesmo padrão de client.go:121-122)
- Resposta de sucesso: HTTP 2xx com `{"success": true}`

## Pontos de Integração

- Serviço externo: WhatsApp Business Cloud API (Graph API), base URL atual `https://graph.facebook.com/v18.0` (client.go:20).
- Autenticação: `META_ACCESS_TOKEN` já existente; nenhuma credencial ou escopo novo.
- Tratamento de erro: `checkStatus` mapeia para `ErrMetaAuth`, `ErrMetaBadRequest`, `ErrMetaServer` (errors.go:5-9); o gateway classifica via `classifyError` (whatsapp_gateway.go:47-56); o consumer trata qualquer erro como best-effort: log `Warn` com `message_id` e incremento de contador, sem retry (coerente com `WithoutRetry()`, client.go:123), sem propagação.
- Gate de versão (RF-07): validar em ambiente de teste que a versão pinada aceita `typing_indicator` antes de ligar a flag em qualquer ambiente. Se rejeitado, decisão separada de bump de versão cobrindo também `internal/platform/whatsapp/media/client.go:22`.

## Abordagem de Testes

### Testes Unitários

- `whatsapp_inbound_consumer_test.go`: o mock de sender escrito à mão ganha o método `SendTypingIndicator`. Cenários novos:
  - flag desligada: nenhuma chamada de typing em nenhum fluxo (texto, áudio, resume, onboarding); os testes existentes passam sem alteração (evidência de zero regressão);
  - flag ligada + texto: typing chamado exatamente uma vez com o WAMID, antes do processamento, e resposta final enviada;
  - flag ligada + falha do typing: resposta final enviada normalmente, contador de erro incrementado, nenhum erro retornado por `Handle`;
  - mensagem duplicada (dedup retorna `inserted=false`): nenhuma chamada de typing;
  - payload inválido: nenhuma chamada de typing.
- `meta/client_test.go` (httptest): assert do JSON exato enviado (`status`, `message_id`, `typing_indicator.type`), ausência do campo `to`, e mapeamento de erro 401/400/500 via `checkStatus`.
- `whatsapp_gateway_test.go`: delegação ao client e classificação de erro via `classifyError`.
- `configs/config_test.go`: default `false` da nova chave e leitura de override por env.

### Testes de Integração

O projeto já mantém testes de integração com banco real (`*_integration_test.go`), portanto os critérios do template estão atendidos e a decisão é manter a prática:

- `whatsapp_inbound_consumer_integration_test.go`: os casos existentes rodam inalterados com flag desligada; um caso novo com flag ligada e sender stub afirma uma única emissão com o WAMID correto.
- `module_boot_integration_test.go:29-115`: o stub de gateway ganha `SendTypingIndicator` retornando nil; boot do módulo segue verde.

### Testes E2E

- Nenhum cenário godog novo. A flag permanece desligada nos ambientes de e2e, preservando os asserts de contagem de mensagens (`feature_e2e_journey_steps_test.go:26-33`) e o fake de servidor WhatsApp.
- Risco mapeado e mitigado: se a flag fosse ligada em e2e, cada chamada de typing apareceria como requisição extra no fake e quebraria contagens; a mitigação estrutural é a operação separada de `SendTextMessage` mais flag off por default.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. `configs/config.go` + teste de config: chave nova com default false (base de tudo, sem efeito comportamental).
2. `meta/models.go` + `meta/client.go` + testes httptest: contrato de payload validado isoladamente (evidência primária do RF-07 em ambiente de teste real).
3. `gateway/whatsapp_gateway.go` + teste: delegação e classificação de erro.
4. Consumer (interface, option, campos, contador, ponto de emissão) + testes unitários.
5. `internal/agents/module.go` (interface local e wiring da option) + ajuste do stub de boot.
6. `cmd/worker/worker.go`: plumbing da flag em `agents.Deps`.
7. Suíte completa do escopo: `go build ./...`, `go vet ./...`, `go test -race -count=1` nos pacotes alterados, `golangci-lint run` no escopo, conforme matriz de validação do AGENTS.md para `application/` + `infrastructure/`.

### Dependências Técnicas

- Gate RF-07 (bloqueante para ativação, não para merge): chamada real de typing em ambiente de teste com a versão pinada da Graph API, com sucesso HTTP e bolha observada no aparelho.
- Nenhuma dependência de infraestrutura, biblioteca ou serviço novo.

## Monitoramento e Observabilidade

- Métrica nova: contador `agents_whatsapp_inbound_typing_total` com labels `channel="whatsapp"` e `outcome` em {`success`, `error`}, criado no construtor do consumer pelo mesmo helper `o11y.Metrics().Counter(...)` dos contadores existentes (consumer :141-155). Cardinalidade controlada; proibido label de usuário ou mensagem.
- Logs: falha do typing em nível `Warn` com `message_id` e erro; emissão com sucesso não loga (volume). Nenhum dado pessoal novo em log.
- Métricas das metas do PRD: taxa de sucesso do typing derivada do contador novo; latência e erro da resposta final derivadas do `agents_whatsapp_inbound_total` existente, que não muda de nome nem de labels.
- Dashboards e alertas: nenhum painel novo nesta fatia; o contador fica disponível para o Prometheus existente.

## Considerações Técnicas

### Decisões Chave

- `adr-001-emissao-sincrona-best-effort-no-consumer.md`: emissão síncrona no início do consumer do worker, com timeout dedicado curto e falha best-effort.
- `adr-002-feature-flag-zero-regressao.md`: rollout por flag de ambiente com default desligado e operação separada de `SendTextMessage`.
- `adr-003-nao-aplicar-design-pattern.md`: seletor determinístico retornou `reject`; solução direta registrada.

Mapeamento requisito -> decisão -> teste:

| Requisito | Decisão | Teste |
| --- | --- | --- |
| RF-01 | Emissão única após dedup, antes do WithTimeout (adr-001) | unit: typing uma vez com WAMID antes do processamento |
| RF-02 | Sem renovação; expiração de 25s da Meta aceita | unit: contagem de chamadas igual a 1 por mensagem |
| RF-03 | Emissão depois da validação e do dedup; rota de ativação fora | unit: duplicada e inválida sem typing |
| RF-04 | Best-effort com log Warn e contador (adr-001) | unit: falha de typing com resposta final intacta |
| RF-05 | `AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED`, default false (adr-002) | config_test: default e override |
| RF-06 | Flag off = fluxo idêntico; operação separada (adr-002) | suíte existente inalterada passando |
| RF-07 | Gate de ativação com evidência real de versão | validação manual em ambiente de teste + httptest do payload |
| RF-08 | Contador com labels de baixa cardinalidade | assert de métrica nos testes do consumer |
| RF-09 | Tipos e método separados; sem outbox/dedup/persistência | e2e e integração existentes inalterados |
| RF-10 | Emissão única mesmo em áudio longo | unit: caminho de áudio emite uma vez e responde |

### Riscos Conhecidos

- Versão pinada `v18.0` pode rejeitar `typing_indicator` (não confirmado na doc oficial, que bloqueia leitura automatizada). Mitigação: RF-07 como gate; bump de versão como decisão separada cobrindo o client de mídia.
- Latência adicionada pela chamada síncrona quando a Meta estiver lenta. Mitigação: timeout dedicado curto na emissão (adr-001) e ausência de retry.
- Reemissão de typing quando o processamento falha após a emissão e o dedup é compensado (consumer :307-315). Impacto: usuário vê a bolha novamente na reentrega. Aceito: comportamento honesto, pois haverá nova tentativa de resposta.
- Latência do polling do outbox entre webhook e worker atrasa a bolha. Aceito na US e no PRD; alternativa de emitir no server foi rejeitada pelo usuário.

### Conformidade com Padrões

- R-ADAPTER-001 (adapter fino): o consumer já chama o gateway diretamente em `sendReply`; a emissão segue o mesmo fluxo `consumer -> gateway -> client`, sem regra de negócio nova no adapter.
- R6.3: interface `whatsAppTypingIndicatorSender` definida no consumidor; `var _ Interface` proibido (R6.4); tipos concretos por padrão (R6.2).
- R5.10: erros com sentinelas existentes (`ErrMetaAuth` etc.) e `%w`; erro do typing tratado uma única vez, no ponto de emissão.
- Zero comentários em Go de produção; `log/slog` via `o11y.Logger()`; `any` em vez de `interface{}`.
- Skills aplicadas: `go-implementation` (classificação `consumer` + `cross-cutting` para o plumbing de config, validação proporcional), `mastra` (consumer do fluxo Thread-Run; nenhum primitivo de `internal/platform/{agent,memory,workflow}` reimplementado), `domain-modeling-production` (sem gatilho de materialização: nenhum diff em `domain/`, `commands/` ou `application/events/`; princípios DMMF preservados), `design-patterns-mandatory` (seletor executado, resultado `reject`, adr-003).

### Arquivos Relevantes e Dependentes

- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (ponto de emissão, interface, option, contador)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go` e `..._integration_test.go`
- `internal/agents/module.go` (interface local :54-56, wiring :415-435)
- `internal/agents/module_boot_integration_test.go` (stub :29-115)
- `internal/onboarding/infrastructure/http/client/meta/client.go`, `models.go`, `client_test.go`
- `internal/onboarding/infrastructure/gateway/whatsapp_gateway.go` e `whatsapp_gateway_test.go`
- `configs/config.go` (AgentConfig :180-199, lista de chaves :550-575, defaults :1433-1442) e `configs/config_test.go`
- `cmd/worker/worker.go` (deps :432-465)
- `.env.example` (documentação da nova variável)
