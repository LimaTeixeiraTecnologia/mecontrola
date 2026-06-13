# Tarefa 6.0: Atualizar 3 sites de onboarding + dispatcher whatsapp

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualiza 3 sites de onboarding (`subscription_binding.go`, `subscription_bound.go`, `module.go`) e 1 site em `internal/platform/whatsapp/dispatcher/dispatcher.go` que constroem `outbox.EventInput`.

<requirements>
- RF-14 (parcial): 4 sites populam AggregateUserID
- Para eventos onboarding (`subscription_bound`), user_id já está disponível no input
- Para dispatcher whatsapp, avaliar se o caller tem user_id (provavelmente sim — Principal já estabelecido) ou se é evento de sistema (allowlist)
- Sem mudança de assinatura pública
- Sem comentário em `.go`
</requirements>

## Subtarefas

- [ ] 6.1 Atualizar `internal/onboarding/application/binding/subscription_binding.go` populando `AggregateUserID`.
- [ ] 6.2 Atualizar `internal/onboarding/application/events/subscription_bound.go` idem.
- [ ] 6.3 Inspecionar `internal/onboarding/module.go` — se cabeamento puro, marcar como inspeção; se construir evento, atualizar.
- [ ] 6.4 Inspecionar `internal/platform/whatsapp/dispatcher/dispatcher.go`: identificar se o evento construído tem user_id (Principal disponível) ou é metadata de sistema. Se sistema, adicionar tipo à allowlist (ADR-004). Se per-user, popular `AggregateUserID`.
- [ ] 6.5 Atualizar testes existentes com asserção do novo campo (ou nota da exceção allowlist).

## Detalhes de Implementação

Ver techspec + ADR-004. Dispatcher whatsapp pode ser fronteira entre Principal-aware e sistema; tratar caso a caso durante implementação.

## Critérios de Sucesso

- `go test -count=1 ./internal/onboarding/... ./internal/platform/whatsapp/...` PASS.
- Asserções de teste cobrindo `AggregateUserID` (ou allowlist).
- `task lint && task test && task vulncheck` PASS.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Asserção `AggregateUserID` ou allowlist em cada site
- [ ] Inspeção `module.go` documentada
- [ ] Caso dispatcher whatsapp justificado (per-user ou sistema)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/onboarding/application/binding/subscription_binding.go`
- `internal/onboarding/application/events/subscription_bound.go`
- `internal/onboarding/module.go` (inspecionar)
- `internal/platform/whatsapp/dispatcher/dispatcher.go`
- Testes correspondentes
