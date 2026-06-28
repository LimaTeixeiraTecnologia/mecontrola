# Tarefa 3.0: Adicionar Servidor HTTP de Health Check ao Worker

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar um servidor HTTP mínimo ao `cmd/worker` na porta `8081`, expondo endpoints `/livez` e `/readyz`, para permitir health checks significativos no Docker Swarm.

<requirements>
- Cobrir RF-05: worker expõe readiness/liveness que validam capacidade real de processar jobs.
- Cobrir RF-07: shutdown gracioso do worker, incluindo encerramento do health server.
</requirements>

## Subtarefas

- [ ] 3.1 Criar `cmd/worker/health.go` com struct `healthServer` e handlers `/livez` e `/readyz`.
- [ ] 3.2 `/readyz` deve validar conexão com o banco via `db.PingContext` com timeout de 2s.
- [ ] 3.3 `/livez` deve retornar 200 se o processo está vivo.
- [ ] 3.4 Integrar o health server em `cmd/worker/worker.go`, iniciando-o após a inicialização do banco.
- [ ] 3.5 Garantir graceful shutdown do health server no `shutdown` do worker.
- [ ] 3.6 Adicionar testes unitários para os handlers.
- [ ] 3.7 Atualizar `compose.swarm.yml` com `healthcheck` apontando para `http://localhost:8081/readyz`.

## Detalhes de Implementação

Ver seção "3. Health Checks" e ADR-004 de `techspec.md`. O health server deve usar `http.Server` com `Shutdown` via contexto. A porta `8081` não deve ser exposta publicamente.

```go
type healthServer struct {
    db      *sqlx.DB
    manager *worker.Manager
}

func (h *healthServer) readyz(w http.ResponseWriter, r *http.Request) {
    ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
    defer cancel()
    if err := h.db.PingContext(ctx); err != nil {
        http.Error(w, err.Error(), http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

## Critérios de Sucesso

- `go test ./cmd/worker/...` passa com testes dos handlers.
- `docker inspect mecontrola_worker-1` mostra `Health.Status: healthy`.
- Simular falha de banco faz `/readyz` retornar 503.
- Durante shutdown, o health server é encerrado sem goroutines vazadas.

## Skills Necessárias

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários para `/livez` e `/readyz`.
- [ ] Teste de integração com banco real (testcontainers ou staging).
- [ ] Teste de graceful shutdown: enviar SIGTERM e verificar que o health server para.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `cmd/worker/worker.go`
- `cmd/worker/health.go` (novo)
- `internal/platform/worker/manager.go`
- `deployment/compose/compose.swarm.yml`
