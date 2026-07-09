# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Dia de fechamento explícito opcional (sentinela `ClosingDay int` + `ClosingDayProvided bool`) e reconhecimento de banco tool-gated
- **Data:** 2026-07-08
- **Status:** Aceita
- **Decisores:** Usuário (product owner), engenharia de plataforma agentiva
- **Relacionados:** PRD (RF-07, RF-08, RF-09, RF-10, RF-11, RF-20); techspec `techspec.md`; `internal/card/application/usecases/create_card.go`; ADR-002 do `prd-onboarding-valor-opcional-meta` (precedente de sentinela+bool)

## Contexto

Hoje `CreateCard.Execute` (`internal/card/application/usecases/create_card.go:49`) **sempre** deriva o
dia de fechamento: `BankDaysReader.DaysBeforeDue(ctx, bank)` retorna `days` (fallback silencioso de 7
para banco não reconhecido) e `PurchaseDayService.Decide(in.DueDay, days, now, tz)` calcula o
`ClosingDay`. O `input.CreateCard` só tem `UserID`, `Nickname`, `Bank`, `DueDay`.

Requisitos conflitantes sobre o MESMO usecase (usado por onboarding e pela nova tool):

- RF-08: cadastro conversacional para banco **não reconhecido** deve usar o dia de fechamento
  **informado pelo usuário**, proibido o fallback silencioso de 7 dias.
- RF-09: o fluxo de **onboarding permanece inalterado** (continua derivando, inclusive o fallback).
- RF-07: banco **reconhecido** deriva de forma autoritativa; valor informado pelo usuário é ignorado.

Um branch do usecase baseado apenas em "banco reconhecido" quebraria o onboarding (passaria a exigir
fechamento explícito para bancos não reconhecidos). É preciso um discriminador que dependa da
**intenção do chamador**, não só do reconhecimento.

## Decisão

1. **Modelagem do valor opcional (DMMF, sentinela + bool):** estender `input.CreateCard` (card) e
   `interfaces.NewCard` (agents, `types.go:122`) com o par:
   - `ClosingDay int`
   - `ClosingDayProvided bool`
   Sem ponteiro e sem `omitempty` — evita as armadilhas de `*int`/`null` no merge-patch do snapshot do
   kernel e segue o precedente do ADR-002 de `prd-onboarding-valor-opcional-meta`.

2. **Branch do usecase por intenção do chamador (não por reconhecimento):**
   - Se `ClosingDayProvided` → `NewBillingCycle(in.ClosingDay, in.DueDay)` diretamente (caminho novo,
     banco não reconhecido).
   - Senão → caminho atual inalterado: `DaysBeforeDue` + `PurchaseDayService.Decide` (onboarding e
     banco reconhecido). **Zero mudança comportamental para o onboarding.**

3. **Reconhecimento tool-gated (RF-07/RF-08/RF-09):** a decisão "preciso perguntar o fechamento?"
   pertence à **tool** `create_card`, não ao usecase. A tool consulta um sinal aditivo de
   reconhecimento e:
   - banco **reconhecido** → NÃO envia `ClosingDayProvided` (força `false`), mesmo que o LLM tenha
     fornecido um `closingDay` (garante RF-07 de forma determinística);
   - banco **não reconhecido** e `closingDay` ausente → retorna `ToolOutcomeClarify` pedindo o dia de
     fechamento (slot conversacional, sem estado durável — RF-06);
   - banco **não reconhecido** e `closingDay` presente → envia `ClosingDay`+`ClosingDayProvided=true`.

4. **Sinal aditivo de reconhecimento:** adicionar leitura `IsBankRecognized(ctx, bank) (bool, error)`
   (read-only `SELECT EXISTS(...) FROM mecontrola.banks WHERE code = $1`) exposta ao consumidor via
   `CardManager.BankRecognized`. **Aditivo:** `BankDaysReader.DaysBeforeDue` permanece com a mesma
   assinatura e o mesmo fallback de 7 dias — onboarding intocado (RF-09).
   **Normalização unificada (resolvido 2026-07-08):** `IsBankRecognized` aplica exatamente a mesma
   normalização do smart constructor `NewBankCode` (NFD + hyphen-join, lowercase) antes do `SELECT`,
   garantindo que reconhecimento e derivação enxergam o mesmo `code`. Isso **elimina** o risco de
   divergência descrito abaixo — fonte única de normalização, sem duplicação.

5. **Validação (RF-10/RF-11):** reusar smart constructors existentes sem duplicação
   (`NewNickname` 1..32, `NewBankCode` não vazio, `NewBillingCycle` `ClosingDay`/`DueDay` 1..31). Sem
   restrição cruzada fechamento×vencimento. Range de dia é **1..31** (não 1..28). Para feedback rápido
   antes da confirmação, o **schema JSON da tool** declara `minimum:1`/`maximum:31` em `dueDay`/
   `closingDay` (validação declarativa de schema, permitida a adapters — R-AGENT-WF-001.2); a validação
   **autoritativa e única** continua nos smart constructors no write (sem duplicação de whitelist em Go
   — R-DTO-004).

## Alternativas Consideradas

- **Branch do usecase por reconhecimento de banco.** Vantagem: autoritativo no domínio. Desvantagem:
  quebra RF-09 (onboarding passaria a falhar para banco não reconhecido). Rejeitada.
- **`ClosingDay *int` com `omitempty`.** Vantagem: idiomático para opcional. Desvantagem: semântica de
  `null` no merge-patch RFC 7386 (remoção de chave) e nil-deref no snapshot; diverge do precedente do
  projeto. Rejeitada.
- **Mudar `DaysBeforeDue` para retornar `recognized bool`.** Vantagem: um único ponto de verdade.
  Desvantagem: altera a assinatura do caminho quente compartilhado com onboarding e exige `tx`; a
  tool não abre `tx`. Preferida a leitura aditiva dedicada.

## Consequências

### Benefícios Esperados

- Onboarding 100% inalterado (RF-09) — caminho de derivação não muda.
- RF-07 garantido de forma determinística no ponto de entrada (tool), não dependente do LLM.
- Fallback silencioso de 7 dias eliminado no caminho conversacional (RF-08) sem removê-lo do onboarding.
- Reuso total dos smart constructors (RF-10/RF-11), zero duplicação de validação.

### Trade-offs e Custos

- Um campo sentinela + bool propagado por 3 structs (`input.CreateCard`, `NewCard`, `CardCreateState`).
- Um método de leitura novo (`IsBankRecognized`) e sua fiação no binding.
- A garantia de RF-07 vive na tool; um chamador futuro que ignore a tool precisaria replicar a regra.

### Riscos e Mitigações

- **Risco:** LLM não fornecer o `closingDay` mesmo após o clarify. **Mitigação:** instruções do agente
  + harness real-LLM (RF-22); a tool re-emite clarify enquanto faltar (RF-06).
- **Risco (RESOLVIDO):** divergência entre reconhecimento (`IsBankRecognized`) e derivação
  (`DaysBeforeDue`). **Resolução:** ambos consultam a mesma tabela `mecontrola.banks` por `code` e
  aplicam a mesma normalização `NewBankCode` antes da consulta; teste cobrindo banco com acento/espaço
  ("banco XP", "Nubank") garante paridade.

## Plano de Implementação

1. Card: `input.CreateCard` + `NewCard` (agents) ganham `ClosingDay`/`ClosingDayProvided`; `Validate()`
   valida `ClosingDay` 1..31 apenas quando `ClosingDayProvided`.
2. Card: branch no `Execute` (provided → cycle direto; senão → derivação atual).
3. Card: leitura `IsBankRecognized` (read repo) + método no `CardManager` binding.
4. Agents: `card_manager_adapter.CreateCard` mapeia os novos campos; `BankRecognized` delega à leitura.

## Monitoramento e Validação

- Teste unitário do usecase cobrindo: provided→cycle direto; não provided→derivação; onboarding
  inalterado; banco reconhecido ignora closing informado.
- Cenário de harness: banco não reconhecido pede fechamento antes de confirmar (RF-08).

## Impacto em Documentação e Operação

- Runbook: comportamento de banco não reconhecido (pergunta o fechamento) vs onboarding (deriva).

## Revisão Futura

- Reavaliar quando a lista `mecontrola.banks` for expandida (fora de escopo deste PRD) ou se o
  onboarding passar a perguntar o fechamento (follow-up), unificando os caminhos.
