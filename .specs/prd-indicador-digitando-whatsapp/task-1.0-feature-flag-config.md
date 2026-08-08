# Tarefa 1.0: Feature flag AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED na configuração

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar a chave de configuração que controla a emissão do typing indicator, com default desligado, espelhando o precedente exato de `AGENT_AUDIO_ENABLED`. Nenhum comportamento muda nesta tarefa; ela apenas disponibiliza a chave para o wiring da tarefa 5.0.

<requirements>
- RF-05: configuração de ambiente com default desligado, ativação sem deploy e sem mudança de código.
- RF-06: com a flag desligada, comportamento idêntico ao atual.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar `WhatsAppTypingIndicatorEnabled bool` com tag `mapstructure:"AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED"` em `AgentConfig` (`configs/config.go:180-199`).
- [ ] 1.2 Adicionar a chave na lista de chaves conhecidas (`configs/config.go:550-575`, junto das demais chaves `AGENT_`).
- [ ] 1.3 Adicionar `l.v.SetDefault("AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED", false)` junto dos defaults de agente (`configs/config.go:1433-1442`).
- [ ] 1.4 Documentar a variável em `.env.example` na seção de agente, com valor `false` e comentário curto em pt-br.
- [ ] 1.5 Adicionar teste em `configs/config_test.go`: default `false` quando a env está ausente e leitura correta quando definida.

## Detalhes de Implementação

Ver `techspec.md`, seção "Arquitetura do Sistema" (item configs) e ADR-002. Seguir o padrão existente de `AudioEnabled` sem introduzir variação de estilo.

## Critérios de Sucesso

- `go test -race -count=1 ./configs/...` verde com o teste novo.
- Nenhuma outra suíte afetada: `go build ./...` e `go vet ./...` verdes.
- A chave tem default `false` comprovado por teste.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (config default e override)
- [ ] Testes de integração (não aplicável: sem fronteira de IO nova; justificativa registrada)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `configs/config_test.go`
- `.env.example`
