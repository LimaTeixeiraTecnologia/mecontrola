# Tarefa 1.0: Datas determinísticas — estender ParseInputDate + helpers puros

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender a função pura `ParseInputDate(text, now)` (internal/agents/application/workflows/write_shared.go:295) para resolver deterministicamente `semana passada`, `mês passado` e `dia N`, eliminando o fallback silencioso para o dia corrente em `resolveEntryDate` (register_entry.go:111). Introduzir helpers puros de calendário. Sem IO, sem abstração de tempo (recebe `now`).

<requirements>
- RF-05: manter resolução determinística das formas já suportadas (hoje/ontem/anteontem/dia-da-semana/DD-MM) sem regressão.
- RF-06: resolver `semana passada` = now − 7 dias; `mês passado` = mesmo dia-do-mês do mês anterior com clamp ao último dia válido; `dia N` = ocorrência mais recente não-futura (varredura até 12 meses).
- RF-07: ausência de data continua resolvendo para o dia corrente.
- RF-22: a mesma resolução vale para o caminho de edição (edit_entry usa resolveEntryDate).
</requirements>

## Subtarefas

- [ ] 1.1 Adicionar helpers puros `daysInMonth`, `resolveSameDayPreviousMonth`, `resolveMostRecentDayOfMonth` no pacote `workflows`.
- [ ] 1.2 Inserir em `ParseInputDate` os ramos `semana passada`, `mês passado`/`mes passado` e `dia N` (regex ancorada `^\s*dia\s+([0-9]{1,2})\s*$`) antes do `return ""`, preservando os ramos atuais e o `parseWeekday`.
- [ ] 1.3 Testes unitários table-driven com `now` fixo cobrindo bordas: clamp 31/03→fev, `dia 31` em mês de 30 dias, `dia N` futuro caindo no mês anterior, ano bissexto, viradas de mês/ano, e não-regressão das formas atuais.

## Detalhes de Implementação

Ver `techspec.md` seção "Design de Implementação → Interfaces Chave" e ADR-001 (`adr-001-resolucao-deterministica-data.md`). Não computar data no LLM; o token verbatim chega em `occurredAt` e esta função resolve. Zero comentários em Go de produção (R-ADAPTER-001.1). Função pura e determinística (DMMF): sem `context.Context`, sem `time.Now()` interno.

## Critérios de Sucesso

- `semana passada`, `mês passado` e `dia N` resolvem para a data correta com `now` fixo; nenhum fallback silencioso para hoje nessas formas.
- Formas já suportadas continuam idênticas (0 regressão).
- `go build`, `go vet`, `go test -race` verdes no pacote; `gofmt -l` limpo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — resolução de data é regra de domínio pura (Decide-style), modelada como função determinística sem IO.
- `design-patterns-mandatory` — gate de desenho para os helpers puros de calendário (aplicar vs. não aplicar padrão; evitar abstração desnecessária de tempo).

## Testes da Tarefa

- [ ] Testes unitários (ParseInputDate + helpers, table-driven, sem mock)
- [ ] Testes de integração (não aplicável — lógica pura, sem IO)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/write_shared.go`
- `internal/agents/application/workflows/write_shared_test.go` (ou novo `*_test.go` de datas)
- `internal/agents/application/usecases/register_entry.go` (consumidor `resolveEntryDate:111`)
