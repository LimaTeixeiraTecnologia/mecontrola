# Tarefa 4.0: Emissão best-effort no WhatsAppInboundConsumer com métrica

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar ao consumer o ponto único de emissão do typing indicator: interface de consumidor, option, campos, contador de métrica e chamada síncrona best-effort logo após o dedup e antes do `WithTimeout` de processamento, conforme ADR-001.

<requirements>
- RF-01: emissão única por mensagem, antes de qualquer processamento.
- RF-02: sem renovação periódica; expiração de 25 segundos da Meta aceita.
- RF-03: sem emissão para duplicadas no dedup nem para payloads inválidos.
- RF-04: falha tratada uma única vez no ponto de emissão, com log Warn e contador, sem retry e sem propagação.
- RF-06: flag desligada significa zero chamadas e zero mudança observável.
- RF-08: contador `agents_whatsapp_inbound_typing_total` com labels `channel` e `outcome` apenas.
- RF-10: caminho de áudio emite uma única vez e a resposta final segue normal após a expiração da bolha.
</requirements>

## Subtarefas

- [ ] 4.1 Adicionar a interface `whatsAppTypingIndicatorSender` (`SendTypingIndicator(ctx, wamid string) error`) no arquivo do consumer, seguindo o padrão de `whatsAppTextSender` (`whatsapp_inbound_consumer.go:42-44`).
- [ ] 4.2 Adicionar `WithTypingIndicator(sender whatsAppTypingIndicatorSender, enabled bool) ConsumerOption` e os campos correspondentes na struct.
- [ ] 4.3 Criar o contador `agents_whatsapp_inbound_typing_total` no construtor, pelo mesmo helper `o11y.Metrics().Counter(...)` dos contadores existentes (:141-155).
- [ ] 4.4 Adicionar o método privado de emissão: guard de flag desligada, sender nulo ou `message_id` vazio retorna sem efeito; senão chama o sender com `context.WithTimeout(ctx, 3*time.Second)`, registra sucesso ou erro no contador e log Warn em caso de falha.
- [ ] 4.5 Invocar a emissão em `Handle` imediatamente após o bloco de dedup (:198-214) e antes do `WithTimeout` de inbound (:216-220).
- [ ] 4.6 Estender o mock escrito à mão do teste unitário com `SendTypingIndicator` e adicionar os cinco cenários da techspec (flag off, flag on texto, falha best-effort, duplicada, payload inválido), mais o caminho de áudio com emissão única.

## Detalhes de Implementação

Ver `techspec.md`, seções "Interfaces Chave", "Monitoramento e Observabilidade" e ADR-001. Regras duras: adapter fino (a emissão chama o gateway diretamente, como `sendReply` já faz), erro tratado uma única vez, sem comentários em Go de produção, `log/slog` via `o11y.Logger()`.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/agents/infrastructure/messaging/database/consumers/...` verde, incluindo os testes preexistentes sem alteração de asserts.
- Cenário flag off prova zero chamadas de typing; cenário de falha prova resposta final intacta e erro não propagado.
- Nenhum primitivo de `internal/platform/{agent,memory,workflow}` reimplementado (regra mastra).
- `go build ./...`, `go vet ./...` e `golangci-lint run` no pacote verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — alteração no consumer do fluxo Thread-Run de WhatsApp inbound em `internal/agents`, com regra de não reimplementar primitivos da plataforma.

## Testes da Tarefa

- [ ] Testes unitários (seis cenários da techspec, tabela de Abordagem de Testes)
- [ ] Testes de integração (caso com flag ligada fica na tarefa 5.0, junto do wiring; aqui a suíte de integração existente deve passar inalterada)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_integration_test.go`
