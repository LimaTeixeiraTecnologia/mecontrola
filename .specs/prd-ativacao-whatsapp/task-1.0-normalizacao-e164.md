# Tarefa 1.0: Normalização E.164 única (`internal/platform/phone`)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar uma fonte única de normalização de telefone BR para E.164 e fazer billing e identity a usarem, garantindo que o `Customer.mobile` da Kiwify e o `msg.From` da Meta fiquem no mesmo formato (`+55DDD9XXXXXXXX`) para correlação por telefone.

<requirements>
- RF-07: normalizar `Customer.mobile` (Kiwify) para E.164 ao persistir na Activation Session.
- RF-21: correlação por telefone exige número normalizado E.164 com a mesma regra do inbound.
- Smart constructor DMMF (Princípio 1/2, R6.8): tipo com campo não exportado + `New*(...) (T, error)`.
- Telefone inválido/vazio → string vazia (não erro fatal), habilitando o caso de borda RF-30.
</requirements>

## Subtarefas

- [ ] 1.1 Criar `internal/platform/phone/mobile.go` com `Mobile` (campo `e164` não exportado), `NewMobileBR(raw) (Mobile, error)`, `String()` e `NormalizeBR(raw) (string, error)`, replicando a regra hoje privada em `internal/identity/domain/valueobjects/whatsapp_number.go:46` (`^\+55\d{2}9\d{8}$`).
- [ ] 1.2 Refatorar `whatsapp_number.go` para delegar a normalização ao novo pacote (sem mudar comportamento observável).
- [ ] 1.3 Normalizar `Customer.Mobile` em `internal/billing/application/usecases/kiwifypayload/commands.go:18` via `phone.NormalizeBR`; inválido/vazio → `""`.

## Detalhes de Implementação

Ver techspec.md, seções "Interfaces Chave" (smart constructor `phone.Mobile`) e ADR-003 (decisão 2 — normalização única). Sem abstração de tempo; sem comentários em `.go`.

## Critérios de Sucesso

- `phone.NormalizeBR` cobre `11999999999`, `5511999999999`, `+5511999999999`, formatos com espaços/parênteses/hífen, inválidos e vazio.
- `whatsapp_number.go` mantém os mesmos resultados (testes existentes verdes).
- `Customer.Mobile` normalizado antes de virar `CustomerMobileE164`; sem regressão no fluxo billing.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (tabela) de `phone.NormalizeBR`/`NewMobileBR` (domínio puro, sem mock).
- [ ] Testes unitários de regressão em `whatsapp_number.go` e no mapeamento `commands.go`.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/phone/mobile.go` (novo)
- `internal/identity/domain/valueobjects/whatsapp_number.go`
- `internal/billing/application/usecases/kiwifypayload/commands.go`
