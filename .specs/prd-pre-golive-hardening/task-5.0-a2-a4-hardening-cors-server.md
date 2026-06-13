# Tarefa 5.0: A2/A4 — Hardening fallback CORS + Server header

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Defesa em profundidade no app Go: garantir que `resolveCORSOrigins()` em `cmd/server/server.go` nunca retorne `[]string{"*"}` em `production` (delegando a falha a B6 task 1.0), e que o servidor não vaze header `Server:` revelando versão da stack.

<requirements>
- RF-25: `resolveCORSOrigins()` em production retorna erro (delegando a Config.Validate) se config insegura — não silenciar com fallback `*`
- RF-26: header `Server:` não é injetado pelo devkit-go nem por chi — verificar; se for, override com middleware global injetando `Server: ""`
- RF-32–34: skills, gates, sem nova dep
- Zero comentário em `.go`
</requirements>

## Subtarefas

- [ ] 5.1 Inspecionar `cmd/server/server.go` `resolveCORSOrigins()` — confirmar comportamento atual em production quando config vazia/wildcard. Se houver fallback inseguro, remover.
- [ ] 5.2 Smoke local: `curl -I http://localhost:<port>/healthz` — verificar se `Server:` aparece. Se aparecer, adicionar middleware global em `server.go` que escreve `Server: ""` antes do `WriteHeader`.
- [ ] 5.3 Testes unit cobrindo fallback de CORS em production (esperado: erro propagado, não `*`).
- [ ] 5.4 Documentar em runbook de deploy a expectativa de ausência do `Server:`.

## Detalhes de Implementação

Ver techspec seção "Modelos de Dados > Config" e plano-fonte §7 passo 7. Esta tarefa é maioritariamente **inspeção + validação**; só implementa middleware override se a inspeção encontrar leak.

## Critérios de Sucesso

- Inspeção documentada: `resolveCORSOrigins()` em production NUNCA retorna `*`.
- `curl -I /healthz` local: header `Server:` ausente ou vazio.
- `task lint && task test && task vulncheck` PASS.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Inspeção manual do código + documentação
- [ ] Smoke `curl -I` local

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `cmd/server/server.go` (modificado se necessário)
- `cmd/server/server_test.go` (modificado se houver testes)
