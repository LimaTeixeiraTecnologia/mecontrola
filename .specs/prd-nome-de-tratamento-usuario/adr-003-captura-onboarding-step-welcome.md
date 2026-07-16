# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Captura do nome no onboarding via reaproveitamento do `step-welcome` (no-op), com carry-in-state e materialização na conclusão
- **Data:** 2026-07-15
- **Status:** Aceita
- **Decisores:** Autor do PRD/techspec, confirmado pelo solicitante (múltipla escolha)
- **Relacionados:** `prd.md` (RF-01, RF-02, RF-04, RF-11), `techspec.md`, ADR-001

## Contexto

- RF-01 pede a pergunta do nome no início do onboarding, integrada à boas-vindas. RF-04 torna o nome opcional (nunca bloqueia).
- Fatos de código: o `Sequence` do onboarding tem `step-welcome` como 1º child, hoje um no-op que só seta `PhaseWelcome` e completa (`onboarding_workflow.go:1009-1015`); o 1º prompt ao usuário é emitido pelo `step-goal` (`welcomeCombinedPrompt`, `:696-701`, `:1019-1022`). Como o welcome nunca suspende, nenhum run em andamento fica suspenso nele — todos os runs suspensos estão no goal ou adiante.
- O `OnboardingPhase` é `iota` persistido como int no snapshot durável; inserir uma fase no meio renumera snapshots suspensos (`:69-78`). O valor do objetivo (`GoalValueCents`) é o precedente de carry-in-state → materializar-na-conclusão (`:1046-1048`, `:1567-1569`).

## Decisão

Reaproveitar o `step-welcome` (no-op) como o passo de captura do nome, sem alterar a estrutura do `Sequence` (mesma quantidade, ordem e IDs de steps; mesmo `PhaseWelcome`):

- Na primeira entrada, o step suspende com a mensagem de boas-vindas + "Antes da gente começar, como você gostaria que eu te chamasse? 💚".
- No resume, extrai o nome via `agent.Execute` com schema estrito; `DecideTreatmentName` normaliza/valida (≤ 40 chars, rejeita vazio/recusa). Se usável, grava em `OnboardingState.TreatmentName`; caso contrário, prossegue sem nome (RF-04) — sem loop de reprompt.
- O `step-goal` deixa de re-saudar (prompt de objetivo sem preâmbulo de boas-vindas).
- O nome é materializado apenas na conclusão (writer único de conteúdo, ADR-001).

## Alternativas Consideradas

- **Inserir um novo child `step-name` no `Sequence` antes do goal (+ `PhaseName`).** Vantagem: fiel à leitura literal de "novo step". Desvantagem: altera a estrutura do `Sequence` durável e o enum de fases, exigindo validar o resume de runs suspensos em andamento (risco de cutover) para garantir 0 regressão. Rejeitada em favor da opção de menor risco.
- **Fundir a pergunta do nome no `welcomeCombinedPrompt` do goal (nome + objetivo juntos).** Desvantagem: mistura duas perguntas, foge do fluxo verbatim e do "uma pergunta por vez". Rejeitada.
- **Persistir o nome imediatamente no step (não na conclusão).** Desvantagem: segundo writer de `working_memory` → clobber (ADR-001). Rejeitada.

## Consequências

### Benefícios Esperados

- Zero mudança estrutural no workflow durável e zero risco de cutover em runs suspensos.
- UX correta: a primeira mensagem passa a ser boas-vindas + pergunta do nome; nome opcional não bloqueia.

### Trade-offs e Custos

- O `step-welcome` deixa de ser no-op (ganha suspensão/extração); o `step-goal` tem sua copy de abertura ajustada.
- Mudança de copy da primeira interação para todos os novos usuários (esperado e desejado).

### Riscos e Mitigações

- Risco: runs em andamento perderem a captura do nome. Mitigação: aceitável por RF-04 (opcional); esses runs já passaram do welcome e seguem sem nome — sem erro.
- Risco: extração equivocada. Mitigação: `DecideTreatmentName` valida; ausência → segue sem nome; edição posterior corrige.
- Rollback: restaurar o `step-welcome` no-op e o `welcomeCombinedPrompt` original.

## Plano de Implementação

1. Adicionar `TreatmentName`/`TreatmentNameAsked` ao `OnboardingState`.
2. Reescrever `BuildWelcomeStep` como captura (suspende/extrai) + consts de prompt.
3. Ajustar a copy de abertura do `step-goal`.
4. Compor as seções na conclusão (ADR-001).
5. Testes unit do step, da conclusão e da opcionalidade.

## Monitoramento e Validação

- `agents_onboarding_treatment_name_total{outcome ∈ captured|skipped|parse_error}`.
- Critério: unit confirma suspensão/extração e conclusão compondo as duas seções; taxa de captura instrumentada (RF-16).

## Impacto em Documentação e Operação

- Atualizar o roteiro de onboarding (docs/runbooks) com o novo primeiro passo.

## Revisão Futura

- Revisitar se o produto exigir um passo de nome separado e explicitamente numerado, ou reprompt de nome no onboarding.
