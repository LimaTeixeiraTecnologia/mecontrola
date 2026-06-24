# Tarefa 2.0: Kernel puro — Step, combinadores e codec

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os primitivos genéricos do kernel em `internal/platform/workflow`, **sem dependência de
domínio**: `Step[S]`, `StepOutput[S]`, estados fechados, os combinadores Core (`Sequence`, `Branch`,
`Parallel`, `Retry`) e o `codec` de serialização do estado. Tudo testável de forma pura, sem mocks de
infraestrutura.

<requirements>
- RF-01: kernel em `internal/platform/workflow` sem import de pacote de domínio.
- RF-02: `Step[S]` composável, IO tipada, id único, testável isolado.
- RF-03: composição sequencial (`Sequence`).
- RF-04: composição condicional (`Branch`) por decisão pura.
- RF-05: composição paralela (`Parallel`) com agregação determinística e cancelamento cooperativo.
- RF-15: estados como tipos fechados (`RunStatus`/`StepStatus`/`SuspendReason`).
- RF-16: `Parallel`/passos longos canceláveis, shutdown cooperativo, sem goroutine leak.
- RF-31: control-flow puro testável sem mocks de infraestrutura.
</requirements>

## Subtarefas

- [ ] 2.1 `step.go`: `Step[S]`, `StepOutput[S]`, `StepFunc[S]`, `Suspension`, tipos fechados
  `RunStatus`/`StepStatus`/`SuspendReason` com `String()`/`IsValid()`/`Parse*` no padrão de `run.go`.
- [ ] 2.2 `combinators.go`: `Sequence[S]` (ordem + threading de estado + short-circuit), `Branch[S]`
  (rota por `decide(S) string`), `Parallel[S]` (cópias de estado, `merge` determinístico, cancelamento
  via `context`), `Retry[S]` (tentativas + backoff exponencial com jitter no molde de outbox).
- [ ] 2.3 `codec.go`: serialização do estado `S` para `[]byte` (JSON) e de volta, com teste de round-trip.
- [ ] 2.4 Testes unitários puros por cenário (tabela) para cada combinador, incluindo `Parallel` com
  `context` cancelado (sem leak) e `Retry` esgotando tentativas.

## Detalhes de Implementação

Ver techspec.md → "Design de Implementação / Interfaces Chave" e "Combinadores". Naming `StepOutput[S]`
(não `Result`) para não colidir com o anti-padrão `Result[T,E]` monádico (governance.md). Erros via
`error`; `errors.Join`/`fmt.Errorf %w`. Sem `init()`, sem `panic`, `context.Context` nas fronteiras.

## Critérios de Sucesso

- Pacote compila sem importar `intent`/`agent`/`transactions` (gate R-WF-KERNEL-001).
- `Sequence`/`Branch`/`Parallel`/`Retry` cobertos por testes puros determinísticos.
- `Parallel` cancela cooperativamente e não vaza goroutine (verificável por teste).
- Zero comentários em `.go`; estados fechados sem string livre em assinatura pública.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/workflow/step.go` (novo)
- `internal/platform/workflow/combinators.go` (novo)
- `internal/platform/workflow/codec.go` (novo)
- `internal/platform/workflow/*_test.go` (novos)
