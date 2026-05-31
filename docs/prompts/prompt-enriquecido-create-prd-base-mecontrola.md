# Prompt enriquecido — create-prd para a base inicial do MeControla

```text
[PAPEL OU POSTURA]
Atue como especialista em discovery de produto e engenharia orientada a SDD/ai-spec, com foco máximo em robustez, rastreabilidade e clareza operacional.

[OBJETIVO]
Use a skill `create-prd` para definir o primeiro PRD de um repositório greenfield do MeControla, em PT-BR, com foco na base inicial do projeto. O resultado deve ser um PRD forte o suficiente para iniciar um projeto Go robusto, production-proof, production-ready, eficiente e evolutivo, sem pular etapas do fluxo obrigatório do `ai-spec`.

[ENTRADAS]
- Documentação oficial obrigatória do repositório `JailtonJunior94/orchestrator`, consultada via `gh` CLI.
- Discovery técnico obrigatório em: `/Users/jailtonjunior/Git/mecontrola-docs/discoveries/technical-arquitetura-backend-mvp-mecontrola/discovery.md`
- Contexto rígido do solicitante:
  - a base do projeto deve usar `ai-spec` na CLI de forma mandatória
  - linguagem obrigatória: Go
  - ferramentas obrigatórias e inegociáveis: Copilot CLI, Codex CLI e Claude Code CLI
  - o repositório deve nascer na organização `https://github.com/LimaTeixeiraTecnologia`
  - `@JailtonJunior94` deve constar como codeowner obrigatório
  - a base deve ser robusta, production-proof e production-ready desde o início

[RESTRIÇÕES]
- Preserve o fluxo oficial do orchestrator: toda feature ou base nova DEVE começar em `create-prd`; não pule para tech spec, tasks ou implementação.
- Consulte a documentação oficial com `gh` CLI antes de redigir o PRD, priorizando:
  - fluxo mandatório `SDD + Harness`
  - cenário de projeto novo
  - uso de `create-prd`
  - baseline com `ai-spec install`
- Trate o discovery técnico local como fonte dominante para:
  - arquitetura base
  - requisitos não funcionais
  - segurança, LGPD, observabilidade, confiabilidade e escalabilidade
  - trade-offs já decididos
- Considere como decisões já fixadas e não reabra alternativas para:
  - Go
  - Postgres
  - monolito modular
  - arquitetura hexagonal por módulo
  - `devkit-go` v0.4.0 como foundation obrigatória
  - OpenTelemetry full-stack
  - Swagger
  - RBAC
  - rate-limit
  - Meta Cloud API direta
  - OpenAI `gpt-4o-mini`
  - Fly.io região `gru`
- Não implemente nada.
- Não gere código.
- Não escreva tech spec.
- Não decomponha em tasks.
- Não proponha troca de linguagem, troca de CLI, ou remoção do `ai-spec`.
- Se a documentação oficial citar outras CLIs, mantenha como mandatórias apenas `copilot`, `codex` e `claude` para este caso.

[PROCESSO]
1. Consulte a documentação oficial via `gh` CLI. Use pelo menos a leitura do README e das seções relativas a `create-prd`, `SDD + Harness`, projeto novo e instalação baseline.
2. Leia o discovery técnico local e extraia apenas o contexto que altera correção do PRD.
3. Modele o projeto como greenfield e trate a arquitetura do discovery como baseline já decidida, não como hipótese aberta.
4. Se faltar informação realmente bloqueante, faça perguntas curtas, uma por vez, obrigatoriamente em múltipla escolha. Priorize nesta ordem:
   - nome do repositório
   - módulo Go/import path
   - recorte do primeiro ciclo do produto (somente foundation ou foundation + capacidade inicial)
5. Se a informação ausente não for bloqueante, declare a premissa explicitamente no PRD em vez de travar.
6. Redija um PRD que sirva como entrada canônica para a sequência oficial:
   `create-prd -> create-technical-specification -> create-tasks -> execute-task`

[CONTEXTO ESSENCIAL A PRESERVAR]
- Produto: MeControla, agente financeiro conversacional via WhatsApp em PT-BR.
- Tipo de projeto: greenfield.
- Escopo-base: backend do MVP com base monolítica modular em Go.
- Pressão dominante: custo de inferência LLM.
- Requisitos estruturais já definidos no discovery:
  - webhook Meta com idempotência e processamento assíncrono
  - motor conversacional com intent router determinístico, cache exato e hard-cap de budget
  - foundation técnica com `devkit-go` v0.4.0
  - segurança com JWT/refresh, RBAC, audit log append-only e LGPD reforçada
  - observabilidade OTel ponta a ponta
  - Postgres com migrations e UnitOfWork
  - readiness para split futuro entre server/worker sem reescrever domínio
- Critérios operacionais já definidos:
  - webhook availability >= 99,5%
  - latência conversacional p95 <= 8 s
  - DSAR manual com SLA de 15 dias
  - budget LLM <= R$ 2 por usuário por mês
- O PRD deve capturar a base inicial do projeto já preparada para evolução segura, e não apenas um esqueleto simplificado.

[CONTRATO DE SAÍDA]
- Formato: markdown em PT-BR, pronto para virar `.specs/prd-<slug>/prd.md`
- Inclua obrigatoriamente:
  - problema
  - objetivos
  - não objetivos
  - personas/atores envolvidos
  - escopo incluído e excluído
  - requisitos funcionais numerados
  - requisitos não funcionais numerados
  - restrições rígidas
  - critérios de sucesso
  - riscos iniciais
  - dependências externas
  - premissas explícitas
  - pendências em aberto
  - uma seção explícita de `Governança ai-spec obrigatória`
  - uma seção explícita de `Bootstrap inicial do repositório`
  - uma seção explícita de `Mandatórios de tooling e operação`
- Na seção `Governança ai-spec obrigatória`, deixe explícito:
  - que o projeto deve iniciar pelo fluxo SDD oficial
  - que o baseline deve ser instalado com `ai-spec` CLI
  - que o repositório deve suportar obrigatoriamente Copilot CLI, Codex CLI e Claude Code CLI
- Na seção `Bootstrap inicial do repositório`, deixe explícito:
  - criação do repositório na organização `LimaTeixeiraTecnologia`
  - necessidade de primeiro commit já alinhado ao baseline de governança
  - necessidade de `CODEOWNERS` com `@JailtonJunior94` como owner obrigatório
- Exclua:
  - código
  - pseudocódigo
  - YAML detalhado
  - pipeline de CI/CD detalhado
  - migrations detalhadas
  - design técnico profundo além do necessário para fixar escopo e restrições
- Tamanho: compacto, mas completo; sem narrativa longa e sem redundância

[TRATAMENTO DE FALHAS OU PREMISSAS]
- Se houver conflito entre fontes, priorize nesta ordem:
  1. restrições explícitas do solicitante
  2. discovery técnico local
  3. documentação oficial do orchestrator para processo e governança
- Se o nome final do repositório não tiver sido informado, use placeholder claro como `<repo-name>` e registre isso em `pendências em aberto`.
- Se o módulo Go não tiver sido informado, use placeholder claro como `github.com/LimaTeixeiraTecnologia/<repo-name>`.
- Se o recorte do primeiro ciclo estiver ambíguo, assuma `foundation robusta + readiness operacional mínima` e registre a premissa.

[NÃO FAÇA]
- Não sugira pular `create-prd`.
- Não proponha stack alternativa.
- Não substitua `ai-spec` por outro fluxo.
- Não remova nenhuma das CLIs mandatórias.
- Não trate robustez/production-ready como opcional.
- Não responder com implementação; responda com PRD.
```
