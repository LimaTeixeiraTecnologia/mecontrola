# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Distribuição financeira por objetivo via perfis fixos com classificação híbrida
- **Data:** 2026-06-23
- **Status:** Aceita
- **Decisores:** Dono do produto, time de plataforma
- **Relacionados:** PRD (RF-13, RF-13a, RF-15, DR-06, DR-07, LG-10, EB-13), techspec.md
- **Conformidade:** R-AGENT-WF-001.4 (LLM só no parse), `domain-modeling.md` (state-as-type, Decide* puro)
- **Inspiração:** https://github.com/mastra-ai/mastra — tool input-schema com enum preenchido pelo LLM

## Contexto

`buildAutoSplits(incomeCents)` aplica um template fixo (40/10/15/20/15) independente do objetivo;
o objetivo só alimenta a WorkingMemory. A decisão de produto (DR-06) é variar a distribuição por
objetivo, mantendo o cálculo determinístico (sem LLM nos percentuais). A tool
`save_onboarding_objective` hoje captura apenas `objective` (string). É preciso um mecanismo de
classificação objetivo→perfil que seja robusto a linguagem livre sem violar R-AGENT-WF-001.4.

## Decisão

Classificação **híbrida** em três níveis, com cálculo de percentuais 100% determinístico:

1. **LLM no parse**: a tool `save_onboarding_objective` ganha um campo enum opcional
   `objective_profile` (`payoff_debt | emergency_fund | invest | specific_goal | organize_spending`).
   O LLM preenche durante o parse (única call-site de LLM — conforme R-AGENT-WF-001.4).
2. **Fallback determinístico**: se o enum vier ausente/inválido, uma função pura
   `classifyByKeyword(objective)` (sem IO, sem LLM) classifica por palavras-chave em pt-br.
3. **Default**: se nada casar, `ProfileOrganizeSpending` (40/10/15/20/15) — RF-13a/EB-13.

A resolução vive em função pura `ResolveObjectiveProfile(llmProfile, objective string) ObjectiveProfile`.
`ObjectiveProfile` é tipo fechado (state-as-type) e o template retorna **basis points**. O **cálculo
cents** (basis points × renda → cents por categoria) **NÃO** é feito no onboarding nem no agente: é
delegado a `internal/budgets` (`AllocationDistributor`, exposto via usecase `SuggestAllocation`),
dono da distribuição (ADR-006). A materialização do budget real permanece via evento
`onboarding.splits_calculated` → `CreateBudget`+`ActivateBudget` (já existente). O ajuste posterior em
linguagem natural (RF-15) continua via fase de plano financeiro.

## Alternativas Consideradas

- **Só LLM no parse (enum)**: menos código, mas sem rede determinística além do default; classificação
  fica 100% dependente do LLM. Rejeitada por robustez.
- **Só classificador determinístico no domínio**: máxima testabilidade, porém heurística por keyword é
  frágil a linguagem livre e diverge do "classificado no parse" aprovado. Rejeitada como única via.
- **LLM gera os percentuais**: viola R-AGENT-WF-001.4 (LLM no cálculo) e é não-determinístico.
  Rejeitada.

## Consequências

### Benefícios Esperados

- Distribuição alinhada ao objetivo (DR-06) com cálculo determinístico e testável.
- Robustez: LLM primário + fallback determinístico + default sem fricção (RF-13a).
- Conformidade R-AGENT-WF-001.4 preservada (LLM só no parse).

### Trade-offs e Custos

- Schema da tool cresce (campo enum); mais um ponto de teste (resolução híbrida).
- Tabela de perfis exige curadoria de produto.

### Riscos e Mitigações

- **Risco:** LLM classificar errado. **Mitigação:** fallback por keyword + default + ajuste em
  linguagem natural (RF-15). Métrica de distribuição por perfil para auditoria.
- **Rollback:** ignorar `objective_profile` e usar sempre `OrganizeSpending` (comportamento atual).

## Plano de Implementação

1. Domínio puro: `ObjectiveProfile`, `ResolveObjectiveProfile`, `classifyByKeyword`, `SplitTemplate`.
2. Tool `save_onboarding_objective`: adicionar enum `objective_profile` (opcional).
3. Persistir o perfil resolvido (ou o objetivo) no payload para reprodutibilidade.
4. `buildAutoSplits(profile, income)` no `budgetPhase`.

## Monitoramento e Validação

- Métrica `agent_onboarding_split_profile_total{profile,source}` (`source=llm|keyword|default`).
- Testes unitários: tabela objetivo→perfil; cada template soma o orçamento; ambíguo→default.

## Impacto em Documentação e Operação

- Runbook/diálogos: refletir distribuição variável por objetivo.
- Documentar a tabela de perfis como contrato de produto.

## Revisão Futura

- Revisar percentuais por perfil com dados de uso; avaliar mais perfis ou personalização.
