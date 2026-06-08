# Tarefa 6.0: Cliente Meta Cloud API e job horario de outreach

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar cliente outbound da Meta Cloud API, gateway de WhatsApp e job horario de outreach para tokens pagos nao consumidos ha mais de 2 horas, respeitando cap rigido de um envio por token.

<requirements>
- Cobrir `RF-09`, `RF-13`, `RF-14`, `RF-16`.
- Cliente Meta deve usar `internal/platform/httpclient`, nunca `&http.Client{}` direto.
- `OutreachJob` deve selecionar tokens `PAID`, com `paid_at < now()-2h`, `customer_mobile_e164` valido e `outreach_sent_at IS NULL`.
- Cada token deve receber no maximo uma tentativa 4xx persistida; erro 5xx pode resetar `outreach_sent_at` para retry posterior.
- Job deve respeitar toggle `WhatsAppConfig.OutreachEnabled`.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 6.1 Implementar `WhatsAppCloudClient` em `infrastructure/http/client/meta`.
- [ ] 6.2 Implementar gateway de envio de template `activation_reminder`.
- [ ] 6.3 Implementar `SendOutreach` com selecao, lock e politicas 4xx/5xx.
- [ ] 6.4 Implementar handler de job via adapter de `internal/platform/worker/job`.
- [ ] 6.5 Emitir metricas `onboarding_outreach_sent_total` e logs PII-safe.
- [ ] 6.6 Testar sucesso, toggle off, erro 4xx, erro 5xx e idempotencia.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 2, 6.8, 7.1, 8.4, 9.2 e ADR-005. O job deve ser fino e delegar para use case.

## Critérios de Sucesso

- Cliente Meta serializa payload de template conforme contrato oficial descrito na techspec.
- Nenhum envio ocorre quando `OutreachEnabled=false`.
- Um token com erro 4xx nao e reenviado em ticks futuros.
- Um token com erro 5xx pode ser retentado no proximo tick.
- Telefone nunca aparece em claro em logs.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitarios do cliente Meta com servidor fake.
- [ ] Testes de `SendOutreach` para filtros e politica de retry.
- [ ] Testes do job handler com contexto cancelavel.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/infrastructure/http/client/meta/`
- `internal/onboarding/application/usecases/send_outreach.go`
- `internal/onboarding/infrastructure/jobs/handlers/outreach_job.go`
- `internal/platform/httpclient/`
- `internal/platform/worker/job/`
