# Tarefa 5.0: Atualizar 3 sites de identity (use cases + module)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Atualiza 3 sites de identity que constroem `outbox.EventInput` direta ou indiretamente: `establish_principal.go`, `mark_user_deleted.go`, e `module.go` (cabeamento).

<requirements>
- RF-14 (parcial): 3 sites identity populam AggregateUserID
- Sem mudança de assinatura pública dos use cases
- Sem comentário em `.go`
- Sem nova dep
</requirements>

## Subtarefas

- [ ] 5.1 Atualizar `internal/identity/application/usecases/establish_principal.go`: ao construir `outbox.EventInput`, popular `AggregateUserID: principal.UserID.String()`.
- [ ] 5.2 Atualizar `internal/identity/application/usecases/mark_user_deleted.go`: idem, `AggregateUserID: userID.String()`.
- [ ] 5.3 Inspecionar `internal/identity/module.go` referenciado pelo grep: se houver construção direta de evento (improvável; provavelmente apenas cabeamento), atualizar. Senão, marcar como inspeção concluída.
- [ ] 5.4 Atualizar testes existentes de cada use case com asserção do novo campo.

## Detalhes de Implementação

Ver techspec. Use cases em identity já consomem `Principal` ou `UserID` por argumento; popular é trivial.

## Critérios de Sucesso

- `go test -count=1 ./internal/identity/...` PASS.
- Asserções de teste cobrindo `AggregateUserID`.
- `task lint && task test && task vulncheck` PASS.

## Skills Necessárias

<!-- MANDATÓRIO -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Asserção `AggregateUserID` em testes dos 2 use cases
- [ ] Inspeção `module.go` documentada

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/identity/application/usecases/establish_principal.go`
- `internal/identity/application/usecases/mark_user_deleted.go`
- `internal/identity/module.go` (inspecionar)
- Testes correspondentes
