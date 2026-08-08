# Tarefa 1.0: Config do limite diário e da mensagem de bloqueio

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar as duas configurações novas do gate de limite diário em `configs/config.go`, seguindo os 4 toques obrigatórios do padrão do módulo (campo com tag mapstructure em `AgentConfig`, entrada em `envKeys()`, default em `setAgentDefaults()` e validação chamada em `Validate()`), com testes de configuração.

<requirements>
- RF-05: mensagem de bloqueio estática e configurável por variável de ambiente no padrão `WA_MSG_*`, informando o limite de 30 interações diárias e a renovação à meia noite.
- RF-08: limite configurável por variável de ambiente com default 30, com valor zero desativando o gate, permitindo ajuste operacional ou desativação sem novo deploy.
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar `DailyInteractionLimit int` com tag `mapstructure:"AGENT_DAILY_INTERACTION_LIMIT"` e `DailyLimitReachedReply string` com tag `mapstructure:"WA_MSG_DAILY_LIMIT_REACHED"` em `AgentConfig` (`configs/config.go:180-199`)
- [ ] 1.2 Registrar as duas chaves em `envKeys()` no bloco de agents (`configs/config.go:551-567`)
- [ ] 1.3 Definir defaults em `setAgentDefaults()` (`configs/config.go:1431-1445`): limite 30 e mensagem em PT-BR informando o limite de 30 interações diárias e a renovação à meia noite
- [ ] 1.4 Adicionar validação: limite na faixa `[0..10000]`; mensagem obrigatória não vazia quando limite maior que zero; registrar o validator na cadeia de `Validate()` (`configs/config.go:824`)
- [ ] 1.5 Cobrir com testes em `configs/config_test.go` no padrão de `config_test.go:953-1060`

## Detalhes de Implementação

- Seção `Modelos de Dados` da techspec.md define os nomes, tipos, defaults e faixas das duas configurações.
- Seção `Decisões Chave` da techspec.md registra a decisão de desativação por valor zero (um único knob, sem flag booleana separada).
- Padrão exato de referência para os 4 toques: campos `AudioUncertainReply` e `AudioRejectedReply` (`configs/config.go:197-198`), defaults (`configs/config.go:1443`), validação de não vazio (`configs/config.go:1076-1082`) e validação de faixa (`configs/config.go:1062`).

## Critérios de Sucesso

- Boot com ambiente limpo aplica default 30 e mensagem default em PT-BR.
- `AGENT_DAILY_INTERACTION_LIMIT=0` é aceito e representa gate desativado.
- Valores fora da faixa `[0..10000]` e mensagem vazia com limite ativo falham a validação com mensagem de erro clara.
- `go build ./...`, `go vet ./...` e `go test -race -count=1 ./configs/...` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: default 30, override por ambiente, faixa rejeitada, mensagem vazia rejeitada com limite ativo, zero aceito como desativação
- [ ] Testes de integração: não aplicável (sem fronteira de IO nova)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `configs/config_test.go`
- `.agents/skills/create-technical-specification` gerou a referência em `.specs/prd-limite-interacoes-diarias-assistente/techspec.md`
