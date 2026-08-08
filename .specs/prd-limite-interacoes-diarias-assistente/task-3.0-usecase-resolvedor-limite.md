# Tarefa 3.0: Decisão pura e use case resolvedor do limite diário

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a função pura `DecideDailyInteractionLimit` e o use case `ResolveDailyInteractionLimit`, responsável por capturar o instante corrente, calcular o início do dia em America/Sao_Paulo, consultar a porta de contagem, decidir permitir ou bloquear, emitir métrica e log, e propagar falha de contagem como erro com wrap `%w` (fail closed, sem liberação silenciosa).

<requirements>
- RF-03: janela do dia definida pela meia noite no fuso America/Sao_Paulo, reutilizando a localização injetada no módulo.
- RF-04: fronteira inclusiva, a 30ª interação é processada e somente a partir da 31ª há bloqueio.
- RF-09: cada bloqueio incrementa contador com label `outcome` fechado e gera log estruturado, sem `user_id` em labels.
- RF-11: Runs que falharem após o dispatch consomem cota normalmente; a contagem considera todos os Runs iniciados na janela.
- RF-12: indisponibilidade da consulta de contagem propaga erro pelo fluxo existente, sem resposta inventada e sem liberação silenciosa.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `decide_daily_interaction_limit.go` com `func DecideDailyInteractionLimit(consumed int, limit int) bool` retornando `consumed >= limit`, padrão de `decide_audio_transcription.go:98` (função pura de pacote, exceção R1)
- [ ] 3.2 Criar `daily_interaction_limit.go` com `DailyLimitResult{Blocked bool, Message string}` e a struct `ResolveDailyInteractionLimit` recebendo `dailyInteractionCounter`, `*time.Location`, limite, mensagem, observability e counter por construtor explícito
- [ ] 3.3 Implementar `Execute(ctx, userID string) (DailyLimitResult, error)`: limite menor ou igual a zero retorna permitido sem consultar o contador; caso contrário calcula o início do dia via `time.Now().UTC().In(loc)` convertido de volta para UTC, consulta a contagem e decide pela função pura
- [ ] 3.4 Emitir counter `agents_daily_limit_total` com label `outcome` (`allowed` ou `blocked`) e log estruturado de bloqueio, span `agents.usecase.resolve_daily_interaction_limit`
- [ ] 3.5 Cobrir com suite testify e fake manual do contador, padrão de `handle_inbound_test.go:20-35`

## Detalhes de Implementação

- Seção `Interfaces Chave` da techspec.md define as assinaturas exatas; seção `Modelos de Dados` define o cálculo da janela do dia.
- ADR-001 registra a decisão do resolvedor e o fail closed com erro tipado; ADR-002 registra a porta de contagem.
- R6.7: proibido `clock.Clock`; usar `time.Now().UTC()` inline e receber `*time.Location` pelo construtor.
- Convenção de métrica: snake_case com sufixo `_total`, unidade `"1"`, labels permitidos conforme `observability/STANDARD.md:127-145`.

## Critérios de Sucesso

- Limite zero ou negativo não toca o banco (fake do contador prova ausência de chamada).
- Fronteira inclusiva provada: 29 de 30 permite, 30 de 30 bloqueia.
- Erro do contador retorna erro com wrap `%w` e mensagem estável em lowercase, sem chamar a decisão.
- Nenhum identificador de usuário em labels de métrica; correlação apenas por trace automático.
- `go build ./...`, `go vet ./...` e `go test -race -count=1 ./internal/agents/...` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — regra de negócio nova modelada como função `Decide*` pura e resultado tipado, aplicando os princípios DMMF sem materializar bundle
- `design-patterns-mandatory` — gate obrigatório de desenho respondendo aplicar versus não aplicar padrão para o resolvedor, com expectativa de não aplicar padrão
- `mastra` — resolvedor posicionado no fluxo canônico Thread e Run antes de `AgentRuntime.Execute`, sem reimplementar primitivos do substrato

## Testes da Tarefa

- [ ] Testes unitários: fronteira inclusiva table-driven; desativação por zero sem chamada ao contador; janela do dia com instante fixo (determinismo, sem `sleep`); erro do contador propaga; métrica emitida com outcome correto
- [ ] Testes de integração: não aplicável (fronteira de IO coberta na tarefa 2.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/usecases/decide_daily_interaction_limit.go` (novo)
- `internal/agents/application/usecases/daily_interaction_limit.go` (novo)
- `internal/agents/application/usecases/daily_interaction_limit_test.go` (novo)
- `internal/agents/application/usecases/decide_audio_transcription.go` (padrão de referência)
- `internal/agents/application/usecases/resolve_onboarding_or_agent.go` (padrão de contrato de resolvedor)
- `observability/STANDARD.md` (convenção de métricas e labels)
