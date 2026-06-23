# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Coleta de cartão por dia de fechamento (`closing_day`)
- **Data:** 2026-06-23
- **Status:** Aceita
- **Decisores:** Dono do produto, time de plataforma
- **Relacionados:** PRD (RF-12, DR-05, LG-09, EB-14), techspec.md, doc de referência `MeControla_Onboarding_V2.md` (REGRA 4)

## Contexto

O documento canônico de produto define a coleta de cartão como "apelido + dia de **fechamento**". O
código atual coleta `due_day` (vencimento): a tool `save_onboarding_card` exige `nickname` +
`due_day`, o dispatcher grava `DueDay` e responde "vence dia X", e `scriptCardQuestion` pede
"vencimento". O dia de **fechamento** é o que determina em qual fatura uma compra cai — funcionalmente
mais relevante para o controle financeiro do MeControla do que o vencimento.

## Decisão

Coletar **apelido + dia de fechamento** no onboarding. A tool `save_onboarding_card` passa a exigir
`closing_day` (inteiro 1..31); `SaveOnboardingCardInput` e o use case mapeiam para
`OnboardingCardDraft.ClosingDay`; o dispatcher lê `closing_day` e responde "fecha dia X"; scripts
atualizados para "dia de fechamento". `due_day` e `limit_cents` **não** são coletados no onboarding.

**Mudança no dono (`internal/card`) — GAP-V1:** como o módulo `card` exigia `closing_day` E
`due_day` (ambos 1..31) e o `billing_cycle` usa `due_day` para calcular o vencimento, o `card` passa
a tornar **`DueDay` opcional** em `input.CreateCard`/`NewBillingCycle` e a **derivar o vencimento
internamente** quando ausente — a regra permanece no dono. Retrocompatível: callers que ainda enviam
`due_day` (HTTP handler, daily) seguem funcionando. O onboarding/agent enviam **somente
`closing_day`**.

## Alternativas Consideradas

- **Manter `due_day`**: diverge do documento canônico e do significado funcional (fatura). Rejeitada.
- **Coletar fechamento + vencimento + limite**: mais completo, porém adiciona atrito e contraria a
  meta de coleta enxuta em uma única mensagem (RF-12). Rejeitada para o MVP.

## Consequências

### Benefícios Esperados

- Alinhamento ao documento de produto e ao ciclo de fatura correto.
- Coleta enxuta preservada (apelido + um número).

### Trade-offs e Custos

- Cartão criado no onboarding fica incompleto (sem vencimento/limite) até configuração posterior.
- Ajuste em tool, DTO, use case, dispatcher e scripts (mudança coesa).

### Riscos e Mitigações

- **Risco:** entrada inválida (dia fora de 1..31). **Mitigação:** validação no smart constructor do VO
  e re-pergunta curta da etapa (EB-14), sem avançar com dado inválido.
- **Rollback:** reverter o schema da tool para `due_day` (mudança localizada).

## Plano de Implementação

1. Tool catalog: `required: [nickname, closing_day]`, 1..31.
2. DTO/use case `SaveOnboardingCard`: campo `ClosingDay` + `Validate()`.
3. Dispatcher: ler `closing_day`, gravar `ClosingDay`, resposta "fecha dia %d".
4. Scripts: "apelido + dia de fechamento".

## Monitoramento e Validação

- Teste unitário: parse/persistência mapeiam para `ClosingDay`; inválido → re-pergunta.
- E2E: etapa de cartões com `Nubank 13` grava fechamento dia 13.

## Impacto em Documentação e Operação

- Runbook/diálogos de onboarding: refletir "dia de fechamento".

## Revisão Futura

- Revisar quando o cadastro completo de cartão (limite/vencimento) for incorporado ao onboarding ou a
  um fluxo pós-onboarding.
