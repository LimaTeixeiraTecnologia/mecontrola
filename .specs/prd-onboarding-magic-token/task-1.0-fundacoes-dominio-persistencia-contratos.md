# Tarefa 1.0: Fundacoes de dominio, persistencia e contratos do onboarding

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a base do modulo `internal/onboarding`: entidades, value objects, transicoes, erros, interfaces de aplicacao, migrations e repositorios Postgres necessarios para tokens, sinais de suporte, idempotencia Meta e tentativas de lookup do consumer quando aplicavel.

<requirements>
- Cobrir `RF-01`, `RF-03`, `RF-06`, `RF-07`, `RF-08`, `RF-11`, `RF-12`, `RF-15`, `RF-18`.
- Implementar token opaco de 32 bytes com `base64.RawURLEncoding`, persistindo apenas SHA-256 raw/hex conforme techspec.
- Implementar estados estritos `PENDING`, `PAID`, `CONSUMED`, `EXPIRED` e transicoes puras sem estados intermediarios.
- Criar migrations para `onboarding.onboarding_tokens`, `onboarding.support_signals`, `onboarding.meta_processed_messages` e tabela auxiliar de tentativas se a outbox nao expuser attempt count.
- Criar repositorios Postgres e interfaces consumidas pela aplicacao sem inversao de dependencia indevida.
- A execucao posterior deve carregar obrigatoriamente `go-implementation`, carregar exemplos apenas sob demanda, verificar `go.mod` antes de usar recursos da linguagem, partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nao usar `internal/platform/runtime` como ponto de partida e nao adicionar comentarios em arquivos Go.
</requirements>

## Subtarefas

- [ ] 1.1 Criar estrutura `internal/onboarding` respeitando `domain -> application -> infrastructure`.
- [ ] 1.2 Implementar VOs de token, status, activation path e support signal kind.
- [ ] 1.3 Implementar entidades `MagicToken` e `SupportSignal` com invariantes de dominio.
- [ ] 1.4 Implementar servico puro de transicoes e erros sentinela necessarios.
- [ ] 1.5 Criar interfaces de repositorio, gateway e builder na camada consumidora.
- [ ] 1.6 Criar migrations SQL e repositorios Postgres com testes unitarios/integracao proporcionais.

## Detalhes de Implementação

Referenciar `techspec.md` secoes 4, 5.4, 6.10, 6.12, 7 e 8.1. Nao duplicar logica de normalizacao de WhatsApp; a validacao BR vem de E1 quando consumida por use cases posteriores.

## Critérios de Sucesso

- Modulo `internal/onboarding` compila isoladamente com dominio puro e sem imports proibidos.
- Token claro nunca e persistido nem exposto por `String()`.
- Transicoes cobrem matriz completa da techspec.
- Migrations sao idempotentes quando especificado e preservam indices necessarios para outreach, expiracao e fallback.
- Repositorios oferecem operacoes atomicas necessarias para as tasks seguintes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitarios para token, status, transitions e entidades.
- [ ] Testes de repositorio Postgres com tag de integracao quando houver infraestrutura local adequada.
- [ ] `go test -race -count=1 ./internal/onboarding/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-onboarding-magic-token/prd.md`
- `.specs/prd-onboarding-magic-token/techspec.md`
- `internal/onboarding/`
- `migrations/`
- `go.mod`
