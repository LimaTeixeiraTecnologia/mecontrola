# Tarefa 4.0: Webhook WhatsApp e ativacao direta por ATIVAR token

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar webhook Meta Cloud API, validacao HMAC, verify token, deduplicacao por WAMID, parse de mensagens inbound e consumo direto de magic token pelo comando `ATIVAR <token>`.

<requirements>
- Cobrir `RF-06`, `RF-07`, `RF-08`, `RF-13`, `RF-14`, `RF-15`, `RF-16`.
- `GET /webhooks/whatsapp` deve responder o desafio Meta quando o verify token conferir.
- `POST /webhooks/whatsapp` deve validar `X-Hub-Signature-256` sobre raw body e deduplicar WAMID.
- Comando `ATIVAR <token>` deve mapear todos os estados previstos em `RF-06`.
- Ativacao paga deve ser transacional: user por WhatsApp real, subscription vinculada, token consumido e evento `onboarding.subscription_bound`.
- Reenvio pelo mesmo numero deve ser idempotente; numero diferente deve gerar signal, metrica e log mascarado.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 4.1 Implementar middleware Meta HMAC com suporte a segredo atual e proximo.
- [ ] 4.2 Implementar handlers de verify e inbound WhatsApp.
- [ ] 4.3 Implementar parser de mensagens Meta para `from`, `wamid`, timestamp e texto.
- [ ] 4.4 Implementar `ConsumeMagicToken` com UoW e publicacao `onboarding.subscription_bound`.
- [ ] 4.5 Implementar deduplicacao de WAMID e limpeza preparada para job posterior.
- [ ] 4.6 Testar matriz de estados, pais nao suportado e reuso por numero diferente.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 5.1, 5.2, 6.2, 6.7, 6.8, 7.1, 7.2, 8.2, 8.3, 9.2 e 9.4. Handlers devem ser finos: decodificam, chamam use case e traduzem resposta.

## Critérios de Sucesso

- Meta verify funciona sem expor segredo.
- Assinatura invalida retorna rejeicao e incrementa metrica.
- `ATIVAR` e case-insensitive e tolera espacos conforme techspec.
- Transacao de consumo nao deixa subscription vinculada sem token consumido ou vice-versa.
- Reuso por numero diferente gera `support_signals(kind='token_reuse_attempt')`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes de middleware HMAC valido, invalido, ausente e rotacao.
- [ ] Testes de handler Meta verify e inbound.
- [ ] Testes de `ConsumeMagicToken` para todos os estados de `RF-06`.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/usecases/consume_magic_token.go`
- `internal/onboarding/infrastructure/http/server/handlers/whatsapp_verify_handler.go`
- `internal/onboarding/infrastructure/http/server/handlers/whatsapp_inbound_handler.go`
- `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go`
- `internal/onboarding/infrastructure/messaging/database/producers/onboarding_event_publisher.go`
