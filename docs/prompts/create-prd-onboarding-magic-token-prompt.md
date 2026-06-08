# Prompt enriquecido — `create-prd` para o próximo PRD do MVP

## Recomendação

A recomendação para iniciar o próximo PRD do MVP de forma robusta é **abrir o PRD do épico E3 `onboarding-magic-token`** em `.specs/prd-onboarding-magic-token/`.

Motivo objetivo:

- `docs/epics/epic-01-identity-foundation.md` já aponta `status: prd_done`.
- `docs/epics/epic-02-billing-pipeline.md` já aponta `status: prd_done`.
- `docs/epics/epic-03-onboarding-magic-token.md` está com `status: pending` e `next_skill: create-prd`.
- `docs/epics/epic-04-reconciliation-hardening.md` está marcado como backlog pós-MVP e declara explicitamente que **não deve ter PRD aberto agora**.

## Prompt original

| Original | Enriquecido |
| --- | --- |
| `Eu quero analisar docs/epics, docs/discoveries/discovery-billing-hotmart-kiwify.md, docs/discoveries/discovery-identity-entitlement.md, docs/discoveries/discovery-onboarding-flow.md e iniciar de forma MVP robusta, eficiente e production-ready, sem falso positivo, o uso da skill create-prd. Analise qual é a recomendação e vamos iniciar de forma robusta o PRD.` | O prompt enriquecido abaixo fixa a recomendação do roadmap, define o alvo correto (`onboarding-magic-token`), ancora as fontes de verdade, trava o escopo em produto, explicita dependências e riscos já conhecidos, e reduz falso positivo ao exigir saída estruturada, sem implementação nem techspec disfarçada. |

## Ambiguidades tratadas

1. O pedido original não dizia qual épico deveria virar PRD primeiro; o enriquecido fixa a recomendação com base no roadmap atual.
2. O pedido original mandava analisar vários documentos, mas não definia qual deles tem precedência; o enriquecido ancora `docs/epics/` como guia de ordem e usa as discoveries apenas como contexto de produto.
3. O pedido original não delimitava o que fazer com dependências já conhecidas; o enriquecido manda registrar dependências e riscos em aberto sem transformar o PRD em especificação técnica.
4. O pedido original não travava o destino do artefato; o enriquecido define `.specs/prd-onboarding-magic-token/prd.md`.
5. O pedido original não explicitava que E4 não deve ser aberto agora; o enriquecido bloqueia esse desvio de escopo.

## Prompt enriquecido

```md
Use a skill `create-prd` para abrir o próximo PRD do roadmap MVP do repositório de forma robusta, econômica, production-ready e sem falso positivo.

### Objetivo

Analisar os artefatos abaixo, confirmar qual é o próximo épico correto para materialização de PRD e, com base no roadmap atual, produzir o PRD de **E3 `onboarding-magic-token`** no path:

- `.specs/prd-onboarding-magic-token/prd.md`

### Fonte de verdade obrigatória

Leia e use como base:

- `AGENTS.md`
- `docs/epics/README.md`
- `docs/epics/epic-01-identity-foundation.md`
- `docs/epics/epic-02-billing-pipeline.md`
- `docs/epics/epic-03-onboarding-magic-token.md`
- `docs/epics/epic-04-reconciliation-hardening.md`
- `docs/discoveries/discovery-billing-hotmart-kiwify.md`
- `docs/discoveries/discovery-identity-entitlement.md`
- `docs/discoveries/discovery-onboarding-flow.md`
- `docs/prompts-base/create-prd-prompt-base.md`

### Regras mandatórias

1. Use o working tree atual como fonte da verdade.
2. Trate `docs/epics/README.md` e os frontmatters dos épicos como a referência para decidir prioridade e bloqueios do roadmap.
3. Confirme explicitamente que:
   - E1 já tem PRD.
   - E2 já tem PRD.
   - E3 é o próximo alvo de `create-prd`.
   - E4 está fora de escopo agora e não deve ganhar PRD neste momento.
4. Trabalhe somente no escopo de produto. Não implemente código, não escreva techspec, não escreva tasks e não detalhe arquitetura além do nível de restrição de produto.
5. Se `.specs/prd-onboarding-magic-token/` já contiver `techspec.md`, `tasks.md`, `task-*.md`, ADRs ou outros artefatos downstream, pare com `needs_input` antes de editar o PRD.
6. Não invente confirmação de fatos que ainda dependem de validação externa. Em especial, se a propagação de `?s={token}` pela Kiwify não estiver comprovada no workspace, registre isso como hipótese/risco em aberto, sem tratar como fato consumado.
7. Não desvie para E4, multi-provider real, hardening pós-MVP, painel admin completo, antifraude avançada, trial alternativo ou backlog além do que já está sustentado pelos documentos.
8. Preserve a separação entre:
   - **o que/por que** no PRD
   - **como** na futura techspec
9. Se houver conflito entre discoveries e épicos, priorize o roadmap atual dos épicos e registre a divergência em `Suposições e Questões em Aberto`.

### Diretriz de recomendação já consolidada

A recomendação esperada é:

- iniciar por `docs/epics/epic-03-onboarding-magic-token.md`
- gerar `.specs/prd-onboarding-magic-token/prd.md`
- usar E1 e E2 apenas como dependências e contexto, não como alvo de novo PRD
- manter E4 explicitamente fora de escopo por ser pós-MVP

### Conteúdo funcional mínimo que o PRD deve cobrir

Sem entrar em implementação, o PRD de `onboarding-magic-token` deve cobrir de forma auditável:

1. o problema de vincular checkout pago ao número real do WhatsApp
2. a persona/ator principal do fluxo
3. o fluxo de onboarding desejado do ponto de vista de produto
4. escopo incluído do MVP
5. escopo excluído explícito
6. dependências e restrições relevantes
7. critérios de sucesso mensuráveis
8. suposições e questões em aberto

### Dependências e bordas que devem aparecer como contexto de produto

- E1 bloqueia implementação, mas não impede escrever o PRD.
- E2 e E3 podem caminhar em paralelo em nível de PRD/techspec, mas E3 depende operacionalmente do resultado do webhook/entitlement de E2 em runtime.
- O fluxo escolhido é `Landing -> Checkout Kiwify -> Thank-you page propria -> wa.me com ATIVAR <token> -> bot ativa conta`.
- A identidade confiável vem do próprio canal WhatsApp, não do número digitado no checkout.
- O fallback por `whatsapp_input` / match E.164 existe como mitigação, não como fluxo principal.

### Critérios de aceite inegociáveis

Considere o trabalho concluído apenas se:

1. o alvo final for exatamente `.specs/prd-onboarding-magic-token/prd.md`
2. a resposta confirmar por que E3 é o próximo PRD correto
3. o documento permanecer em nível de produto, sem pseudo-código e sem detalhamento de implementação
4. o PRD cobrir as seis categorias obrigatórias da base canônica
5. riscos e hipóteses não confirmadas forem explicitados como abertos, sem invenção
6. E4 for mantido fora de escopo explicitamente

### Formato de saída esperado

Responda em PT-BR retornando apenas:

1. `status_final`
2. `spec_alvo`
3. `path_final`
4. `recomendacao_confirmada`
5. `resumo_funcional`
6. `suposicoes_abertas`
7. `artefatos_downstream_detectados`
8. `proximos_passos` apenas se houver `needs_input`
```

## Justificativa das adições

| Adição | Justificativa curta |
| --- | --- |
| Alvo fixado em E3 | Remove ambiguidade e segue o roadmap real do repositório. |
| Bloqueio explícito de E4 | Evita abrir PRD fora da janela correta do MVP. |
| Path final definido | Alinha o prompt com a base canônica de `create-prd`. |
| Travas contra implementação | Evita techspec disfarçada e reduz falso positivo. |
| Tratamento de hipótese sobre `?s={token}` | Impede inventar validação externa ainda não comprovada. |
| Dependências E1/E2 explicitadas | Mantém contexto suficiente sem desviar o foco do PRD alvo. |

## Variante curta

```md
Use `create-prd` para materializar o próximo PRD correto do roadmap MVP. Analise `AGENTS.md`, `docs/epics/README.md`, os épicos E1-E4, as discoveries de billing/identity/onboarding e `docs/prompts-base/create-prd-prompt-base.md`. Confirme que E1 e E2 já têm PRD, que E3 `onboarding-magic-token` é o próximo alvo e que E4 está fora de escopo agora. Gere somente o PRD em `.specs/prd-onboarding-magic-token/prd.md`, mantendo foco em produto, sem implementação, sem techspec e sem tasks. Use o working tree atual como fonte da verdade; se a propagação de `?s={token}` pela Kiwify não estiver comprovada localmente, registre como hipótese/risco aberto, sem inventar confirmação. Responda em PT-BR com `status_final`, `spec_alvo`, `path_final`, `recomendacao_confirmada`, `resumo_funcional`, `suposicoes_abertas`, `artefatos_downstream_detectados` e `proximos_passos` apenas se houver `needs_input`.
```
