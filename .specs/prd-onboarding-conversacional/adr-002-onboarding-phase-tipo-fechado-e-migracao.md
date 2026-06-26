# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** `OnboardingPhase` como tipo fechado e migração das sessões em andamento por reset
- **Data:** 2026-06-25
- **Status:** Aceita
- **Decisores:** JailtonJunior94 (product owner), time de plataforma
- **Relacionados:** PRD (RF-22/23), techspec, mapeamento (M-8), governança DMMF (`.claude/rules/governance.md`), R-AGENT-WF-001.3

## Contexto

O campo `Phase` em `onboarding_sessions.payload` é hoje uma `string` livre (`"objective"`, `"budget"`, …), o que viola o princípio DMMF **state-as-type** (status/estado como tipos fechados; nunca `string` solta em assinatura pública). Além disso, o modelo muda de 5 fases para **8 etapas** com nomes diferentes — não há mapeamento 1:1 seguro entre o modelo antigo e o novo. Pode haver sessões `in_progress` no momento do deploy.

## Decisão

Introduzir o tipo fechado `OnboardingPhase` (8 constantes enumeradas: `PhaseWelcome`…`PhaseConclusion`) com `String()`/`ParseOnboardingPhase()`/`IsValid()`, substituindo a `string` livre nas assinaturas públicas. Persistência em coluna TEXT via `String()` (fronteira de código permanece tipada). **Migração:** ao carregar uma sessão `in_progress` cujo `Phase` não mapeie para o novo enum, a sessão é **reiniciada** (`reset` idempotente via `StartBudgetConfiguration`) para `PhaseWelcome` no primeiro contato pós-deploy; dados parciais antigos são descartados.

## Alternativas Consideradas

- **Migração best-effort (mapear fases antigas→novas).** Vantagem: preserva progresso parcial. Desvantagem: etapas novas (apresentação de categorias, resumo) não têm dados equivalentes no modelo antigo, gerando estado híbrido inconsistente e código de migração frágil. Rejeitada.
- **Coexistência (drenar via flag).** Vantagem: não reinicia ninguém. Desvantagem: dualidade de código e fluxos. Rejeitada (ver ADR-001).

## Consequências

### Benefícios Esperados
- Conformidade DMMF/R-AGENT-WF-001.3; erros de fase impossíveis por construção.
- Migração simples, determinística e sem estado híbrido.

### Trade-offs e Custos
- Usuários a meio do onboarding antigo reiniciam (perda de progresso parcial).

### Riscos e Mitigações
- **Impacto de UX do reset:** aceitável pré-escala (poucas sessões). Mitigação: deploy em janela de baixa atividade; logar contagem de resets; comunicar no runbook.

## Plano de Implementação
1. Criar `OnboardingPhase` (VO fechado). 2. Trocar leitura/escrita de `Phase` para o tipo. 3. Lógica de reset na carga de sessão com fase legada. 4. Testes de parse/serialização e de reset.

## Monitoramento e Validação
- Métrica/contagem de `onboarding_session_reset_total` no deploy.
- Critério: nenhuma sessão permanece com `Phase` em formato legado após o primeiro contato.

## Impacto em Documentação e Operação
- Nota no runbook de deploy sobre o reset único de sessões em andamento.

## Revisão Futura
- Revisar se o produto exigir preservação de progresso parcial em migrações futuras (pós-escala).
