# Prompt: Consolidação de Arquitetura Core (Identity, Billing & Onboarding)

Este prompt foi desenhado para orquestrar a consolidação técnica de três frentes críticas do projeto MeControla, garantindo uma base robusta e pronta para produção.

---

## Objetivo
Consolidar as descobertas técnicas de Cobrança, Identidade e Onboarding, confrontando-as com o estado atual do código e a proposta de valor da Landing Page, resultando em um dossiê de arquitetura inegociável e eficiente.

## Contexto Base
- **Discoveries:**
    - `docs/discoveries/discovery-billing-hotmart-kiwify.md`
    - `docs/discoveries/discovery-identity-entitlement.md`
    - `docs/discoveries/discovery-onboarding-flow.md`
- **Codebase:** `internal/identity/` e `internal/finance/` (foco em alinhar os domínios e interfaces).
- **Landing Page:** `https://github.com/LimaTeixeiraTecnologia/mecontrola-landingpage` (verificar promessas de produto e fluxo de conversão).
- **Stack:** Go, Postgres, Redis, WhatsApp Business API, Kiwify.

## Instruções para o Agente (Role: Arquiteto de Soluções Sênior)

1. **Ativação:** Utilize a skill `@.agents/skills/decision-brainstorming` para conduzir este processo.
2. **Fase de Pesquisa:**
    - Leia os 3 arquivos de discovery e mapeie as dependências entre eles (ex: como o Magic Token do onboarding afeta o Entitlement).
    - Inspecione a estrutura atual em `internal/` para identificar "drift" (desalinhamento) entre o planejado e o que está scaffolded.
    - Analise a Landing Page para garantir que o fluxo técnico suporta a promessa de marketing.
3. **Brainstorming Decisório:**
    - Execute as rodadas de entendimento, escopo, alternativas e trade-offs.
    - **Desafio Mandatório:** Force a discussão sobre a economia de recursos (tokens LLM, processamento async vs sync) e resiliência (idempotência em webhooks).
    - **RBAC vs Simplicidade:** Reavalie a decisão de adiar o RBAC confrontando com possíveis planos "Família" ou "Equipe" citados na landing page.
4. **Output Robusto:**
    - Gere o bundle em `.agents/skills/decision-brainstorming/discoveries/brainstorm-consolidacao-core/`.
    - O `decision-brief.md` deve conter uma seção específica de **"Arquitetura Inegociável"** (regras que não podem ser quebradas na implementação).

## Critérios de Sucesso (Definition of Done)
- [ ] Dossiê consolidado que elimina ambiguidades entre cobrança e identidade.
- [ ] Mapeamento claro de como a falha de um webhook da Kiwify é recuperada via reconciliação.
- [ ] Validação de que a normalização E.164 está centralizada no domínio de Identidade.
- [ ] Decisão explícita sobre a estratégia de cache de Entitlement (TTL inteligente).

---
**Nota:** Este prompt foca em ser **production-proof**. Não aceite soluções que não incluam tratamento de erro, auditoria (raw events) e segurança de dados (LGPD).
