# Tarefa 3.0: Wiring: repasse do novo campo em module.go e nos entrypoints server e worker

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Repassar `AudioDurationExceededReply` da configuração até `ProcessAudioInboundConfig` no wiring do módulo e nos dois entrypoints que duplicam o bloco `AudioConfig`, impedindo divergência entre server e worker.

<requirements>
- RF-03: a mensagem dedicada só chega ao use case se o wiring repassar o campo em todos os caminhos de boot.
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar campo `DurationExceededReply string` na struct `AudioConfig` de deps em `internal/agents/module.go:86-97`.
- [ ] 3.2 Repassar o campo na construção de `ProcessAudioInboundConfig` em `internal/agents/module.go:404-412`.
- [ ] 3.3 Repassar o campo no bloco `agents.AudioConfig{...}` de `cmd/server/server.go:247-258`.
- [ ] 3.4 Repassar o campo no bloco duplicado de `cmd/worker/worker.go:444-455`, no mesmo commit da 3.3.
- [ ] 3.5 Não alterar nenhum outro repasse, ordem de construção ou lifecycle do módulo.

## Detalhes de Implementação

Ver `techspec.md` seção `Arquitetura do Sistema` (itens `internal/agents/module.go`, `cmd/server/server.go`, `cmd/worker/worker.go`) e `Sequenciamento de Desenvolvimento` (passo 3). DI manual explícita no padrão `NewIdentityModule`/`InvoiceModule` do AGENTS.md: construtor direto, campos nomeados, sem opções novas.

## Critérios de Sucesso

- `go build ./...` e `go vet ./...` verdes, cobrindo os dois binários.
- `go test -race -count=1 ./internal/agents/...` verde.
- Grep pelo novo campo confirma presença nos quatro pontos (`module.go` struct, `module.go` wiring, `cmd/server`, `cmd/worker`), sem quinto ponto esquecido nem divergente.
- Nenhum diff no consumer nem em outros módulos.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — o wiring altera o módulo do consumer agentivo de referência (`internal/agents`); seguir o padrão de módulo e o fluxo canônico Thread-Run sem reimplementar primitivos do substrato.
- `design-patterns-mandatory` — gate de desenho obrigatório para mudança Go; expectativa `não aplicar padrão` (ADR-003), wiring direto por construtor.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/module.go`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `.specs/prd-limite-audio-20-segundos-whatsapp/techspec.md`
