# Tarefa 6.0: Gate de versão RF-07 e validação completa de zero regressão

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Executar o gate de ativação do RF-07 com evidência real contra a Graph API na versão pinada e rodar a validação ampla de zero regressão. Esta tarefa não escreve código de produção; ela produz a evidência que autoriza (ou bloqueia) ligar a flag.

<requirements>
- RF-06: evidência objetiva de que a suíte completa passa com a flag desligada e sem alteração de asserts.
- RF-07: evidência real de que a versão da Graph API em uso aceita `typing_indicator`; se rejeitado, a ativação fica bloqueada e o bump de versão vira decisão separada.
</requirements>

## Subtarefas

- [ ] 6.1 Em ambiente de teste com número WhatsApp real, enviar mensagem inbound e disparar a chamada de typing via client implementado na tarefa 2.0 (script ou teste manual guiado), registrando: status HTTP, corpo da resposta e captura de tela da bolha e dos ticks azuis no aparelho.
- [ ] 6.2 Se a chamada for rejeitada pela versão pinada, registrar a evidência do erro e abrir decisão de bump de versão (fora deste PRD), mantendo a flag desligada.
- [ ] 6.3 Rodar a validação ampla: `go build ./...`, `go vet ./...`, `go test -race -count=1 ./...`, `golangci-lint run ./...` e os gates de governança do repositório, registrando comandos e resultados.
- [ ] 6.4 Rodar a suíte e2e de onboarding e os testes de integração de WhatsApp com a flag desligada, confirmando zero alteração de comportamento e de contagens.
- [ ] 6.5 Documentar o resultado do gate no PRD ou em ata curta anexa a esta pasta, com veredito explícito: ativação autorizada ou bloqueada.

## Detalhes de Implementação

Ver `techspec.md`, seções "Pontos de Integração" e "Riscos Conhecidos", e PRD RF-07. A documentação oficial da Meta bloqueia leitura automatizada; por isso a evidência exigida é empírica, não bibliográfica.

## Critérios de Sucesso

- Evidência anexa com resposta HTTP e observação no aparelho, ou bloqueio registrado com o erro da Meta.
- Todos os comandos de validação executados e verdes, com saída registrada.
- Veredito explícito sobre a ativação da flag por ambiente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (suíte completa como evidência)
- [ ] Testes de integração (suítes de WhatsApp e onboarding como evidência)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-indicador-digitando-whatsapp/prd.md`
- `.specs/prd-indicador-digitando-whatsapp/techspec.md`
- `internal/onboarding/infrastructure/http/client/meta/client.go`
- `internal/onboarding/e2e/`
