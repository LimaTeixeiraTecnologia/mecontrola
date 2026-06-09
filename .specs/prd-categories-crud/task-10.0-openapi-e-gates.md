# Tarefa 10.0: OpenAPI, testes de cenários canônicos e gates R0–R7

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `openapi.yaml` versionado em `internal/categories/`, implementar testes de integração cobrindo todos os cenários canônicos (CC-B1 a CC-V4), e executar gates de qualidade R0-R7 e R-ADAPTER-001. Esta é a tarefa de fechamento do MVP.

<requirements>
- RF-07: OpenAPI deve declarar apenas endpoints de leitura (sem POST/PUT/DELETE/PATCH)
- RT-10: `openapi.yaml` versionado + CI lint + artifact
- RF-39: testes positivos para alta confiança, testes negativos para termos ambíguos
- RF-40a: migration inválida falha integralmente (validado em testes)
- CC-B1 a CC-B5, CC-D1 a CC-D5, CC-L1 a CC-L5, CC-V1 a CC-V4: cenários canônicos de aceitação
- R-ADAPTER-001: gates de zero comentários e adaptadores finos
</requirements>

## Subtarefas

- [ ] 10.1 Criar `internal/categories/openapi.yaml` (OpenAPI 3.0.3) cobrindo os 4 endpoints
- [ ] 10.2 Implementar testes de integração para CC-B1 a CC-B5 (busca)
- [ ] 10.3 Implementar testes de integração para CC-D1 a CC-D5 (input degenerado)
- [ ] 10.4 Implementar testes de integração para CC-L1 a CC-L5 (listagem)
- [ ] 10.5 Implementar testes de integração para CC-V1 a CC-V4 (cache e versionamento)
- [ ] 10.6 Executar gates R0-R7: `go build`, `go vet`, `go test -race -count=1`, `golangci-lint run`
- [ ] 10.7 Executar gate R-ADAPTER-001: verificação de comentários e SQL direto em adapters
- [ ] 10.8 Validar cobertura de todos os RF-nn

## Detalhes de Implementação

Ver PRD seção **Cenários Canônicos de Aceitação**.

Regras Go mandatórias:
- Carregar obrigatoriamente `go-implementation`
- Carregar `references/build.md` para gates de validação
- Verificar `go.mod` antes de usar recursos da linguagem
- Partir de `cmd/server/server.go`
- Zero comentários em arquivos `.go`

Pontos críticos:
- OpenAPI: primeiro do projeto; servir como referência para futuros módulos.
- Testes canônicos: devem rodar contra servidor HTTP real (ou usar `httptest` com router real) e Postgres real (testcontainers).
- CC-V3: aplicar migration editorial em teste e validar que `ETag` incrementa.
- CC-V4: migration inválida NÃO deve incrementar versão.
- Gate R0: `grep` por `init()` fora de `main`.
- Gate R-ADAPTER-001.1: `grep` por comentários `//` em `.go` (exceto `//go:`, `//nolint:`, `// Code generated`).
- Gate R-ADAPTER-001.2: `grep` por SQL direto em handlers/consumers/jobs/producers.

## Critérios de Sucesso

- [ ] `openapi.yaml` descreve contrato completo dos 4 endpoints
- [ ] Todos os cenários canônicos CC-B1 a CC-V4 têm teste automatizado que passa
- [ ] Gate R0-R7 passa sem violações
- [ ] Gate R-ADAPTER-001 passa sem violações
- [ ] Todos os RF-nn estão cobertos por pelo menos um teste
- [ ] `go test ./... -race -count=1` passa

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Integration test E2E: CC-B1 a CC-B5
- [ ] Integration test E2E: CC-D1 a CC-D5
- [ ] Integration test E2E: CC-L1 a CC-L5
- [ ] Integration test E2E: CC-V1 a CC-V4
- [ ] Script de gate R0-R7
- [ ] Script de gate R-ADAPTER-001

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/categories/openapi.yaml`
- `internal/categories/infrastructure/http/server/handlers/*_integration_test.go`
- `cmd/server/server.go`
- Scripts de gate (podem ser adicionados ao `Taskfile.yml` ou `Makefile`)
