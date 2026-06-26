# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Cartão no onboarding — coletar só o vencimento; derivar o fechamento por offset configurável
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** JailtonJunior94 (product owner), time de plataforma
- **Relacionados:** PRD (RF-08/09/10, D-08, QT-08), techspec, mapeamento (M-6), oficial Cap. 08 ETAPA 4 + Cap. 10 (Regra de Cartões / Regra de Competência)

## Contexto

O documento oficial (Cap. 08 ETAPA 4 e Cap. 10) manda **solicitar apenas apelido + dia de vencimento da fatura**; nunca limite, banco, bandeira, dados sensíveis — nem fechamento. Porém os dois conceitos são distintos e ambos importam ao domínio:
- 📅 **Fechamento** = define **quais compras entram na fatura** (usado pela Regra de Competência, Cap. 10:892-906).
- 💳 **Vencimento** = **último dia para pagar** a fatura sem juros (o que o oficial coleta).

O código atual coleta `ClosingDay` (fechamento) e o exibe como "fecha dia N" — divergente do oficial. `card.CreateCard` exige `ClosingDay` (1–31) e tem `DueDay` opcional (`internal/card/application/dtos/input/create_card.go`).

## Decisão

O onboarding coleta **apenas apelido + dia de vencimento** (`DueDay`), fiel ao oficial. O **fechamento é derivado** internamente por uma função pura `DeriveClosingDay(dueDay, offsetDays)` (wrap 1..31), com `offsetDays` **configurável e documentado** (`AGENT_ONBOARDING_CARD_CLOSING_OFFSET_DAYS`, default 10). A derivação ocorre no domínio do onboarding (`Decide*`); o evento `onboarding.card_registered` carrega `DueDay` (coletado) e `ClosingDay` (derivado). O consumer do módulo `card` chama `card.CreateCard` com ambos, mantendo a competência funcional. A obrigatoriedade do vencimento por este caminho é validada **no seam do onboarding**, preservando o contrato HTTP público de `card` (não tornar `ClosingDay` ausente uma quebra do create externo).

## Alternativas Consideradas

- **Coletar ambos (vencimento + fechamento) no onboarding.** Vantagem: competência exata, sem regra assumida. Desvantagem: **diverge do Cap. 10 ("solicitar apenas")** e adiciona atrito (1 pergunta a mais). Rejeitada por infidelidade ao oficial.
- **Só vencimento, competência por vencimento (sem fechamento).** Vantagem: nenhuma regra de offset. Desvantagem: muda a semântica da competência do Cap. 10 e classifica errado compras feitas entre fechamento e vencimento. Rejeitada por incorreção financeira.
- **Assumir fechamento = vencimento.** Rejeitada: semanticamente incorreto, distorce competência/fatura.

## Consequências

### Benefícios Esperados
- Fidelidade ao oficial (coleta mínima) + competência funcional.
- Regra de derivação isolada, pura e testável; offset ajustável sem deploy de código.

### Trade-offs e Custos
- Offset uniforme não reflete a variação real do intervalo fechamento↔vencimento entre cartões.

### Riscos e Mitigações
- **Erro de competência por offset inadequado (R1 da techspec):** mitigar com default documentado e configurável; monitorar reclamações/inconsistências de fatura; reavaliar se houver evidência (possível evolução: coletar fechamento opcionalmente no futuro).
- **Impacto no contrato `card.CreateCard` (R2):** manter `DueDay *int` no DTO público; impor o vencimento apenas no caminho do onboarding; `ClosingDay` continua aceito quando informado externamente.

## Plano de Implementação
1. `DeriveClosingDay` (puro) + config do offset. 2. `SaveOnboardingCard` coleta `DueDay`. 3. Evento `card_registered` com `DueDay`+`ClosingDay` derivado. 4. Consumer `card` cria com ambos. 5. Testes de borda do wrap e de competência.

## Monitoramento e Validação
- Log do offset aplicado por cartão criado; auditoria amostral de competência.
- Critério: cartões criados via onboarding possuem `DueDay` = informado e `ClosingDay` = derivado coerente.

## Impacto em Documentação e Operação
- Documentar `AGENT_ONBOARDING_CARD_CLOSING_OFFSET_DAYS` no runbook/config.
- Atualizar mensagens do onboarding para "Vencimento: dia N".

## Revisão Futura
- Revisar se métricas de fatura indicarem erro material de competência, ou se o produto decidir coletar o fechamento.
