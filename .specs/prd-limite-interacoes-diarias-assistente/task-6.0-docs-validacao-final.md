# Tarefa 6.0: Documentação de ambiente e validação final com gates

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Documentar as duas novas variáveis de ambiente nos arquivos de referência de configuração e executar a bateria final de validação proporcional definida na techspec, fechando a feature com evidência de zero regressão.

<requirements>
- RF-05: a mensagem de bloqueio `WA_MSG_DAILY_LIMIT_REACHED` documentada como configurável por ambiente, com default em PT-BR.
</requirements>

## Subtarefas

- [ ] 6.1 Documentar `AGENT_DAILY_INTERACTION_LIMIT` e `WA_MSG_DAILY_LIMIT_REACHED` em `.env.example` com comentário de comportamento (default 30, zero desativa) e alinhar `deployment/config/prod.env`
- [ ] 6.2 Executar `gofmt -l` nos arquivos alterados, `go build ./...`, `go vet ./...` e `go test -race -count=1 ./internal/agents/... ./configs/... ./cmd/...`
- [ ] 6.3 Executar `go test -tags integration ./internal/agents/...` cobrindo o repositório novo e o boot do módulo
- [ ] 6.4 Executar `golangci-lint run` no escopo alterado, `task ci:agent-boundary`, `task ci:zero-comments` e `task ci:platform-gates`
- [ ] 6.5 Revisar o diff completo da feature confirmando que nenhum cenário preexistente de teste foi alterado e que as seções de `Conformidade com Padrões` da techspec.md estão atendidas

## Detalhes de Implementação

- Seção `Conformidade com Padrões` da techspec.md lista a matriz de validação obrigatória e os gates de CI.
- ADR-001 registra o rollback operacional por `AGENT_DAILY_INTERACTION_LIMIT=0`, que deve constar no comentário da variável.

## Critérios de Sucesso

- Todos os comandos de validação executados com saída registrada; qualquer gate indisponível é registrado explicitamente com o motivo, sem substituto inventado.
- Diff final sem alteração de comportamento fora do gate: consumer fino, substrato intocado, sem migration, sem comentário em Go de produção.
- Documentação de ambiente consistente com os defaults e a validação implementados na tarefa 1.0.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: reexecução completa do escopo como evidência de não regressão
- [ ] Testes de integração: reexecução do escopo com tag `integration`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.env.example`
- `deployment/config/prod.env`
- `.specs/prd-limite-interacoes-diarias-assistente/techspec.md` (matriz de validação)
