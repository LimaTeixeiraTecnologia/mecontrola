# Prompt para `create-prd` — próximo épico a detalhar

## Prompt original

> Eu quero analisar o próximo passo que deve ser construido com base em: `docs/epics`, `docs/discoveries/discovery-billing-hotmart-kiwify.md`, `docs/discoveries/discovery-identity-entitlement.md`, `docs/discoveries/discovery-onboarding-flow.md`, quero iniciar um prd utilizando `create-prd` de forma eficiente, robusta, sem falso positivo e cobrindo todos os gaps, production-ready esse prd.

## Ambiguidades removidas

1. "Próximo passo" foi tornado objetivo e auditável: identificar **qual épico deve receber o próximo PRD agora**.
2. O prompt passa a exigir seleção baseada no estado real dos épicos, não em suposição.
3. O resultado esperado deixa explícito que o agente deve **abrir PRD só para o candidato válido**, registrar gaps e não avançar para techspec/tasks.
4. O prompt força o uso do `AGENTS.md` e do working tree atual como fonte de verdade, reduzindo falso positivo.

## Prompt enriquecido

```md
Use `AGENTS.md` como fonte canônica e trate o working tree atual como fonte da verdade.

Quero iniciar o **próximo PRD do roadmap** usando a skill `create-prd`, com foco em eficiência, robustez, cobertura de gaps e qualidade production-ready. **Não implemente código, não gere techspec, não crie tasks e não avance para execução.**

## Objetivo

Analisar os artefatos abaixo, identificar **qual é o próximo épico correto para abrir PRD agora**, justificar a escolha com base no estado real do roadmap e então materializar **somente esse PRD**.

## Fontes obrigatórias

- `AGENTS.md`
- `docs/epics/epic-01-identity-foundation.md`
- `docs/epics/epic-02-billing-pipeline.md`
- `docs/epics/epic-03-onboarding-magic-token.md`
- `docs/epics/epic-04-reconciliation-hardening.md`
- `docs/discoveries/discovery-billing-hotmart-kiwify.md`
- `docs/discoveries/discovery-identity-entitlement.md`
- `docs/discoveries/discovery-onboarding-flow.md`

## Regras obrigatórias

1. Não assumir contexto fora dos arquivos lidos.
2. Não abrir PRD para épico que já esteja com `prd_done`.
3. Não abrir PRD para épico cujo próprio documento diga explicitamente para **não abrir PRD agora**.
4. Se existir mais de um candidato plausível, decidir pelo mais coerente com:
   - status do épico;
   - campo `next_skill`;
   - dependências e bloqueios declarados;
   - prontidão real do roadmap;
   - menor risco de falso positivo.
5. Se o working tree atual contradizer os docs, prevalece o working tree e a opção mais segura.
6. Não inventar requisitos, integrações, APIs, fluxos ou dependências não sustentadas pelos artefatos.
7. Se houver gap crítico que impeça um PRD confiável, registrar explicitamente o gap e tratá-lo no PRD como dependência, risco ou pendência validável — sem mascarar a incerteza.

## Processo esperado

1. Ler os épicos e discoveries.
2. Determinar qual épico é o próximo candidato real para `create-prd`.
3. Explicar por que os demais **não** devem receber PRD agora.
4. Criar o PRD apenas do épico selecionado.

## Expectativa de decisão

Faça a seleção usando critérios auditáveis. Em especial:

- Épicos com `prd_done` não são candidatos.
- Épicos com `next_skill: create-prd` e sem PRD materializado são candidatos prioritários.
- Épicos marcados como backlog e com instrução explícita de não abrir PRD agora devem ser excluídos.

## Saída esperada

Entregue em **pt-BR** e com estas partes, nesta ordem:

1. **Decisão do próximo épico**
   - nome do épico escolhido;
   - status atual;
   - justificativa curta e objetiva.

2. **Candidatos descartados**
   - tabela com: épico, motivo do descarte, evidência.

3. **Gaps, riscos e dependências**
   - liste apenas o que for material para um PRD confiável;
   - destaque dependências entre épicos;
   - destaque validações externas pendentes;
   - não trate hipótese como fato.

4. **PRD completo do épico escolhido**
   - problema;
   - objetivo;
   - contexto consolidado;
   - personas/atores;
   - escopo incluído;
   - fora de escopo;
   - restrições e invariantes;
   - dependências;
   - riscos;
   - métricas de sucesso;
   - critérios de aceite mensuráveis;
   - perguntas em aberto, somente se realmente restarem.

## Requisitos de qualidade do PRD

O PRD precisa:

1. Ser específico o suficiente para permitir a próxima etapa técnica sem reabrir discussão básica.
2. Separar claramente o que é requisito, o que é restrição, o que é dependência e o que é risco.
3. Cobrir integrações e contratos somente no nível necessário para PRD, sem virar techspec.
4. Evitar duplicação desnecessária do que já estiver consolidado em épicos anteriores.
5. Deixar explícitos os pontos de acoplamento com épicos anteriores ou paralelos.
6. Ser production-ready no nível de produto/escopo: sem lacunas óbvias de fluxo, operação, observabilidade mínima, idempotência, segurança ou fallback quando isso já estiver exigido pelos documentos base.

## Critérios de aceite da sua resposta

Sua resposta só é válida se:

1. identificar explicitamente qual é o próximo épico correto para abrir PRD agora;
2. justificar por evidência por que os outros não entram agora;
3. não abrir PRD para algo já coberto ou explicitamente fora de timing;
4. produzir um PRD consistente com discoveries + épicos + AGENTS;
5. registrar gaps reais sem esconder incertezas;
6. não implementar nada além do PRD.

## Resultado persistente esperado

Salve o PRD no caminho coerente com o épico escolhido, seguindo o padrão do repositório em `.specs/prd-<slug>/prd.md`.

Se o épico selecionado for `epic-03-onboarding-magic-token.md`, o caminho esperado é:

- `.specs/prd-onboarding-magic-token/prd.md`

Antes de finalizar, confirme que a decisão respeita o roadmap atual e que não houve avanço indevido para techspec, tasks ou implementação.
```

## Observação de uso

Este prompt já está orientado para levar a skill a concluir que, **no estado atual do roadmap**, o próximo PRD candidato é **E3 — onboarding-magic-token**, salvo divergência real no working tree.
