# Tarefa 8.0: Mapa capacidade→tool, relatório de gaps e gate anti-falso-positivo

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Produzir e versionar o mapa formal capacidade→tool e o relatório de gaps reproduzível a partir do
código, com classificação em 3 buckets, fonte única versionada e gaps abertos = 0, mais o gate de
validação go.mod como verificação final. Depende da 6.0 e é paralelizável com a 7.0. Ver techspec.md,
"Sequenciamento de Desenvolvimento" (passo 8).

<requirements>
- RF-01, RF-03, RF-04, RF-06, RF-07, RF-08, RF-36.
- Dependência: 6.0. Paralelizável com 7.0.
</requirements>

## Subtarefas

- [ ] 8.1 Script/relatório reproduzível code-vs-tools: comparação entre as tools registradas em
  `internal/agents/module.go` e os use cases dos módulos.
- [ ] 8.2 Mapa capacidade→tool versionado sob `.specs/prd-mecontrola-agent-tools/`, com classificação
  em 3 buckets (RF-01/RF-03) e fonte única versionada (RF-06).
- [ ] 8.3 Gate go.mod (`go mod verify` + `go build ./...` + `go vet ./...`) como verificação final
  (RF-36, substitui `scripts/verify-go-mod.sh` inexistente) e relatório de gaps abertos = 0
  (RF-07/RF-08).

## Detalhes de Implementação

Ver techspec.md, "Sequenciamento de Desenvolvimento" (passo 8) e "Conformidade com Padrões". O
relatório de gaps é reproduzível a partir do código (fonte de verdade das tools registradas em
`internal/agents/module.go`), garantindo que capacidades do bucket 3 não estejam expostas. O gate
go.mod substitui o `scripts/verify-go-mod.sh` inexistente pelos comandos padrão da toolchain.

## Critérios de Sucesso

- O relatório de gaps retorna 0 gaps abertos.
- O mapa capacidade→tool está commitado sob `.specs/`.
- `go mod verify` + `go build ./...` + `go vet ./...` verdes.
- Nenhuma capacidade de bucket 3 exposta.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — registro de tools, instruções do agente, scorers e verificação da superfície seguem o molde internal/agents sobre internal/platform.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Execução do relatório de gaps (esperado vazio) + comandos de validação go.mod (`go mod verify`,
`go build ./...`, `go vet ./...`). Unitário N/A (artefato de verificação).

## Arquivos Relevantes
- `.specs/prd-mecontrola-agent-tools/` (mapa/relatório)
- `internal/agents/module.go` (fonte de verdade das tools registradas)
