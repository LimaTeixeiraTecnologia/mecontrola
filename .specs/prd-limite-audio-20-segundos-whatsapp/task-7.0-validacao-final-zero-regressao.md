# Tarefa 7.0: Validação final e evidência de zero regressão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Executar o gate final de validação proporcional definido na techspec e consolidar a evidência de zero regressão exigida pelo PRD, incluindo a prova dos requisitos que não geram implementação nova.

<requirements>
- RF-06: auditoria de rejeição por duração com outcome, reason e `duration_ms`, provada pela suíte de integração existente.
- RF-07: fluxo de texto e demais validações de áudio inalterados, provado pelas suítes existentes sem alteração de expectativa.
- RF-09: nenhuma comunicação proativa adicionada; conformidade por ausência, verificada por grep.
</requirements>

## Subtarefas

- [ ] 7.1 `go build ./...` e `go vet ./...` verdes.
- [ ] 7.2 `go test -race -count=1 ./configs/... ./internal/agents/... ./internal/platform/whatsapp/...` verde.
- [ ] 7.3 Suíte de integração de auditoria (`audio_audit_repository_integration_test.go`, incluindo `TestInsertTerminalAcceptsPreSTTRejectionReasons`) executada com a tag/infra de integração do projeto e verde (RF-06).
- [ ] 7.4 Suítes de consumer (`whatsapp_inbound_consumer_test.go`) verdes sem nenhum diff de expectativa (RF-07).
- [ ] 7.5 `golangci-lint run` no escopo dos pacotes alterados sem findings novos, e gates de governança do repositório (taskfiles/gates.yml) verdes.
- [ ] 7.6 Evidência RF-09: grep repo-wide confirmando que nenhum código de envio proativo (template/broadcast) foi adicionado no diff da feature.
- [ ] 7.7 Evidência de fronteira de zero regressão: diff final confinado à lista de arquivos modificados da techspec; qualquer arquivo fora da lista invalida a tarefa.
- [ ] 7.8 Consolidar o relatório de evidências com comandos executados e resultados, e marcar as tarefas 1.0 a 6.0 como `done` em `tasks.md` após conferir seus critérios.

## Detalhes de Implementação

Ver `techspec.md` seções `Conformidade com Padrões` (perfil de validação `boundary`), `Arquivos Relevantes e Dependentes` (lista fechada de modificados vs. somente leitura) e `Mapeamento Requisito -> Decisão -> Teste`.

## Critérios de Sucesso

- Todos os comandos de validação executados e verdes, com saída registrada.
- Cobertura de RF-01 a RF-09 confirmada contra a tabela de `tasks.md`.
- Nenhum arquivo fora da lista de modificados da techspec no diff final.
- `ai-spec check-spec-drift .specs/prd-limite-audio-20-segundos-whatsapp` sem drift ao final.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-limite-audio-20-segundos-whatsapp/techspec.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/tasks.md`
- `taskfiles/gates.yml`
- `internal/agents/infrastructure/persistence/audio_audit_repository_integration_test.go` (somente execução)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer_test.go` (somente execução)
