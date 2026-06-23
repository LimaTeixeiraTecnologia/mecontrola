# Prompt de Entrada para Inicialização do PRD - Onboarding V2

Este arquivo contém o prompt enriquecido e estruturado para ser alimentado no agente de IA executando a skill `.agents/skills/create-prd`. Ele foi gerado aplicando a skill `.agents/skills/prompt-enricher` com base nos planos de arquitetura e requisitos de negócio fornecidos.

---

```markdown
Você é um Engenheiro de Software Principal e Product Owner especializado em Domain-Driven Design (DDD) e Domain Modeling Made Functional (DMMF).
Sua tarefa é inicializar um **Documento de Requisitos do Produto (PRD)** para o **MeControla - Onboarding V2** usando a skill `.agents/skills/create-prd`.

### 1. Escopo e Local de Destino
- **Slug do PRD**: `onboarding-v2`
- **Arquivo de Destino**: `.specs/prd-onboarding-v2/prd.md`
- **Idioma**: Português (pt-br)
- **Template Obrigatório**: Você deve ler e seguir fielmente a estrutura definida em `.github/skills/create-prd/assets/prd-template.md` (ou equivalente em `.agents/skills/create-prd/assets/prd-template.md`), definindo:
  - Visão Geral
  - Objetivos
  - Histórias de Usuário (primárias e secundárias, cobrindo caminhos felizes e de exceção)
  - Funcionalidades Core
  - Requisitos Funcionais (formato numérico estrito `RF-nn` para rastreabilidade de drift)
  - Experiência do Usuário (jornadas de conversação em texto)
  - Restrições Técnicas de Alto Nível
  - Fora de Escopo

### 2. Contexto do Codebase e Arquitetura do Projeto
Para fundamentar o PRD, analise e integre os padrões de governança do projeto descritos em `AGENTS.md` e o código existente nos módulos:
- `internal/onboarding/` (especialmente a persistência de sessão e o WhatsApp message processor).
- `internal/agent/` (fluxo de execução do agente, sessões, threads e runs).
- `configs/` (variáveis de ambiente, feature flags de LLM).

Incorpore as decisões e fases de implementação detalhadas nos planos técnicos:
1. `docs/plans/2026-06-23-onboarding-auto-start-llm-mandatory.md`:
   - Auto-start do onboarding sem fricção imediatamente após ativação com "ATIVAR [token]".
   - Remoção de feature flag `OnboardingLLMEnabled` tornando o LLM mandatório (sempre ativo, com FSM apenas como fallback de degradação).
   - Envio proativo da primeira saudação via event consumer (`onboarding.subscription_bound`).
2. `docs/plans/2026-06-23-onboarding-persistencia-isolada-conclusao-deterministica-part-2.md`:
   - Isolamento total da persistência do onboarding: histórico de turnos (`recent_turns`) e estado funcional (`phase`, `objective`, etc.) salvos na coluna JSONB `payload` de `mecontrola.onboarding_sessions`.
   - Remoção de qualquer leitura ou gravação de estado de onboarding em `mecontrola.agent_sessions` (que permanece de uso exclusivo do agente principal).
   - Conclusão determinística do onboarding: promoção de estado para `state = active` com preenchimento transacional de `completed_at` (em `onboarding_sessions`), disparado apenas quando todos os pré-requisitos de domínio forem satisfeitos (objetivo definido, orçamento mensal válido, cartões coletados, custom split gerado e primeira transação financeira registrada).
   - Handoff seguro: O agente principal detecta onboarding concluído somente por sinais determinísticos persistidos (`state = active` e `completed_at` presentes), nunca por heurísticas textuais ou memória conversacional solta.

### 3. Inspiração em Mastra e Separação de Agentes
- Garanta que o onboarding LLM e o agente conversacional principal fiquem isolados em workflows/tools independentes no módulo `internal/agent` para evitar colisões e interferência mútua de estado.
- Identifique e documente potenciais lacunas estruturais, conflitos de domínio ou incompatibilidades de banco de dados/esquema que possam surgir entre os modelos de dados atuais de `onboardingSessionPayloadJSON` (em `internal/onboarding/infrastructure/repositories/postgres/onboarding_session_repository.go`) e a nova estrutura do payload isolado (que exige `recent_turns`, `welcome_sent_at`, `completed_at`).

### 4. Regras Mandatórias de Negócio - Onboarding V2
Foque na otimização de conversão e redução extrema de atrito, documentando os seguintes requisitos funcionais e regras:

- **REGRA 1 — ELIMINAR CONFIRMAÇÕES DESNECESSÁRIAS**: Proibir perguntas como "Faz sentido?", "Entendeu?", "Posso continuar?", "Tudo certo até aqui?", "Posso seguir?". O fluxo deve fluir organicamente.
- **REGRA 2 — APRESENTAR AS 5 CATEGORIAS EM UMA ÚNICA MENSAGEM**: Exibir as 5 categorias fixas (Custo Fixo, Conhecimento, Prazeres, Metas, Liberdade Financeira) em um único bloco de texto formatado, sem solicitar confirmação ou explicar uma a uma.
- **REGRA 3 — EXIBIR PROGRESSO**: Exibir o estágio atual em toda interação de onboarding (ex.: `🔵 Etapa 1/4 — Objetivo`, `🔵 Etapa 2/4 — Orçamento`, `🔵 Etapa 3/4 — Cartões`, `🔵 Etapa 4/4 — Plano Financeiro`).
- **REGRA 4 — COLETA DE CARTÃO OTIMIZADA**: Solicitar todos os cartões de crédito em uma única mensagem usando formato de exemplo `Nubank 13` / `Inter 5` / `Itaú 10` ou aceitar resposta "Não uso".
- **REGRA 5 — GERAÇÃO AUTOMÁTICA DE DISTRIBUIÇÃO**: Após receber Objetivo e Orçamento Mensal, sugerir automaticamente a distribuição financeira.
- **REGRA 6 — NUNCA REINICIAR A DISTRIBUIÇÃO**: Preservar o progresso já coletado e recalcular apenas as diferenças durante correções.
- **REGRA 7 — AJUSTE CONVERSACIONAL**: Interpretar e aplicar alterações de distribuição/limite usando linguagem natural de forma fluida.
- **REGRA 8 — RESUMO FINAL ENXUTO**: Exibir apenas as informações essenciais (Objetivo, Orçamento, Cartões e Distribuição Final).
- **REGRA 9 — NÃO ENCERRAR APÓS O RESUMO**: Iniciar imediatamente o fluxo de registro do primeiro lançamento financeiro (primeira transação).
- **REGRA 10 — PRIORIDADE DE EXPERIÊNCIA**: Toda decisão do agente ou fluxo deve priorizar a redução de atrito e aumento da ativação do usuário.

### 5. Análise de Lacunas e Incompatibilidades
Analise criticamente o codebase atual e os planos técnicos fornecidos. No PRD, crie uma seção dedicada para apontar e documentar:
- Inconsistências de modelo de dados (ex: estrutura de `recent_turns` no payload JSONB do repositório em comparação ao formato de turnos no `RunOnboardingTurn`).
- Riscos de concorrência ou colisão entre o processador de WhatsApp de onboarding (`whatsapp_message_processor.go`) e a infraestrutura de agentes do módulo `internal/agent` quando ambos operam sobre o mesmo canal WhatsApp.
- Casos de borda do FSM de fallback caso o LLM falhe temporariamente ou encontre problemas de timeout.
- Como o sistema garante a idempotência da saudação inicial em caso de múltiplas entregas da mensagem de ativação.

### 6. Restrições e Tecnologias
- Compatível com LLMs: Gemini Flash e GPT-5.4 Nano.
- Banco de dados: PostgreSQL.
- Linguagem do Backend: Go (Use Cases, Domain Entities, Services e Infrastructure HTTP/WhatsApp/Outbox).
- Rápida ativação em sessão curta utilizando Tool Chain da plataforma do agente.

Inicie a redação do PRD agora seguindo essas diretrizes.
```
