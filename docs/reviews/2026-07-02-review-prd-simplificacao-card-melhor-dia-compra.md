# Prompt Enriquecido — Review Criterioso PRD `simplificacao-card-melhor-dia-compra`

> Gerado por `$prompt-enricher` em 2026-07-02. Este arquivo contém o prompt original, o prompt enriquecido pronto para execução e a justificativa de cada adição. Nenhuma alteração de código foi feita — apenas o enriquecimento do prompt.

---

## 1. Prompt Original (fornecido pelo usuário)

```
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização,
validando estritamente contra .specs/prd-simplificacao-card-melhor-dia-compra

Critérios obrigatórios:
- Todos os critérios de aceite atendidos (implementados)
- DoD 100% atendido (implementados)
- 0 gaps
- 0 lacunas
- 0 falsos positivos
- 0 ressalvas
- Todas Regras de negócio atendidas (implementados)

Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o
ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em
conformidade total com a especificação.
```

### 1.1 Ambiguidades e lacunas identificadas
1. Não define qual é a fonte canônica do escopo dentro do PRD (`prd.md`, `techspec.md`, `tasks.md`, ADRs, ou todos).
2. Não especifica onde estão os artefatos de execução já produzidos (`2.0_execution_report.md`, `3.0_execution_report.md`) e se devem ser tratados como evidência ou re-verificados do zero.
3. Não define o formato de saída do relatório de review (o skill `review` possui `assets/review-report-template.md` — deve ser usado obrigatoriamente).
4. Não define critério de parada do ciclo `review → bugfix → review` (quantas iterações máximas, o que fazer se não convergir).
5. Não explicita que "sem flexibilização" significa: nenhuma ressalva aceita como "aceitável", nenhum critério marcado como "parcialmente atendido" sem virar bug.
6. Não menciona qual branch/diff deve ser usado como base do review (branch atual vs. main, ou commits específicos das tasks 1.0–9.0).
7. Não define alçada para subagentes especializados (quando disparar, com que escopo, e como consolidar os achados).
8. Não define idioma de saída do relatório final (assumido pt-BR, conforme instrução geral do usuário).

### 1.2 Contexto adicional levantado no repositório
- O PRD possui 9 tasks executadas (`task-1.0` a `task-9.0`), 5 ADRs (`adr-001` a `adr-005`) e 2 relatórios de execução já existentes.
- Arquivos-fonte de verdade do escopo: `.specs/prd-simplificacao-card-melhor-dia-compra/prd.md`, `techspec.md`, `tasks.md` e os 5 ADRs.
- Skill `review` usa template obrigatório em `.agents/skills/review/assets/review-report-template.md`.
- Skill `bugfix` usa formato canônico de bug em `.agents/skills/bugfix/references/canonical-bug-format.md` e template em `.agents/skills/bugfix/assets/bugfix-report-template.md`.
- Governança do repositório (`AGENTS.md`) exige, para qualquer tarefa que toque código Go: carregar `agent-governance` + `go-implementation`, validar com `go build`, `go vet`, `go test -race -count=1 ./<pacote>/...` e `golangci-lint run` no escopo alterado.
- Regras arquiteturais obrigatórias (R0–R7, DMMF, padrão de módulo `internal/identity`/`internal/billing`) aplicam-se a qualquer achado de bug neste domínio (`internal/billing`, cards, invoices).

---

## 2. Prompt Enriquecido (pronto para execução)

```
PAPEL: Você é um revisor técnico sênior atuando como code owner, responsável por
validar 100% da conformidade da implementação do PRD
"simplificacao-card-melhor-dia-compra" contra sua especificação, sem
flexibilização de nenhum critério.

CONTEXTO OBRIGATÓRIO A CARREGAR ANTES DE REVISAR:
1. AGENTS.md (regras canônicas do repositório).
2. .agents/skills/agent-governance/SKILL.md.
3. .agents/skills/review/SKILL.md (fluxo de review) e
   .agents/skills/review/assets/review-report-template.md (template obrigatório
   do relatório de saída).
4. .agents/skills/go-implementation/SKILL.md (etapas 1–5 e checklist R0–R7),
   pois o escopo altera código Go em internal/billing e módulos relacionados.
5. Todos os artefatos do PRD em .specs/prd-simplificacao-card-melhor-dia-compra/:
   - prd.md (requisitos funcionais e critérios de aceite)
   - techspec.md (arquitetura, contratos, ADRs referenciados)
   - tasks.md (lista de tasks e Definition of Done por task)
   - adr-001-bank-free-text-no-fk.md
   - adr-002-closing-day-derived-cached.md
   - adr-003-purchase-day-pure-service.md
   - adr-004-budgets-cardlimit-removal.md
   - adr-005-consolidate-nickname.md
   - task-1.0 a task-9.0 (cada uma com seu próprio DoD)
   - 2.0_execution_report.md e 3.0_execution_report.md (evidências previamente
     reportadas — devem ser re-verificadas contra o código atual, nunca aceitas
     como prova definitiva sem confirmação no working tree)

ESCOPO DE VALIDAÇÃO (fonte de verdade = working tree atual, não os relatórios
de execução):
- Ler cada requisito funcional e critério de aceite de prd.md.
- Ler cada regra de negócio explícita em prd.md e techspec.md (ex.: derivação
  de closing day, purchase day como serviço puro, remoção de cardLimit em
  budgets, bank como texto livre sem FK, consolidação de nickname).
- Para CADA task (1.0 a 9.0), verificar seu DoD específico em tasks.md contra
  o código real no repositório (não contra o relatório de execução).
- Validar contratos de API (OpenAPI) e testes E2E de transactions (task-7.0).
- Validar migrations e alterações de schema (task-1.0).
- Validar que nenhuma regra de arquitetura foi violada (domain puro, DMMF,
  R0–R7, ausência de comentários em Go de produção, ausência de panic).

CRITÉRIOS DE APROVAÇÃO (TODOS obrigatórios e não negociáveis — qualquer falha
em qualquer um bloqueia o status APPROVED):
1. 100% dos critérios de aceite do prd.md implementados e verificados no
   código real (não apenas mencionados em relatório de execução).
2. 100% do Definition of Done de cada task (1.0–9.0) atendido.
3. 0 gaps funcionais (nenhum requisito do PRD sem implementação correspondente).
4. 0 lacunas de cobertura de teste para as regras de negócio críticas
   (closing day, purchase day, remoção de cardLimit, bank free-text).
5. 0 falsos positivos — todo item marcado como "implementado" deve ter
   evidência direta (arquivo + linha, teste passando, comando executado) e não
   apenas inferência ou suposição.
6. 0 ressalvas — nenhum achado pode ser classificado como "aceitável",
   "menor", "para próxima iteração" ou equivalente; todo desvio vira bug a ser
   corrigido antes de aprovar.
7. 100% das regras de negócio (explícitas nos 5 ADRs e no techspec.md)
   implementadas corretamente e testadas.

FORMATO DE SAÍDA DO REVIEW:
- Usar obrigatoriamente o template de
  .agents/skills/review/assets/review-report-template.md.
- Relatório em pt-BR.
- Cada critério de aceite e cada DoD deve aparecer como linha de checklist com
  status (✅ Atendido / ❌ Não atendido) e evidência (caminho de arquivo,
  linha, comando de validação executado e resultado).
- Se status final não for "APPROVED", listar cada achado no formato canônico
  de .agents/skills/bugfix/references/canonical-bug-format.md, pronto para
  virar input direto da skill bugfix.

CICLO OBRIGATÓRIO review → bugfix → review:
1. Executar review completo com os critérios acima.
2. Se status = APPROVED (0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas):
   parar e entregar o relatório final.
3. Se status ≠ APPROVED: para cada achado, invocar
   .agents/skills/bugfix/SKILL.md, aplicando correção pela causa raiz com
   teste de regressão obrigatório (seguindo go-implementation etapas 1–5 e
   validação R0–R7 quando o achado envolver código Go).
4. Após a correção, repetir o review do zero (não incremental) sobre o
   critério corrigido e sobre os demais critérios já aprovados, para garantir
   que a correção não introduziu regressão.
5. Repetir o ciclo até obter APPROVED sem ressalvas. Não há limite artificial
   de iterações — o ciclo só termina quando 100% dos critérios obrigatórios
   forem atendidos e verificados com evidência direta.
6. Registrar em ordem cronológica cada iteração do ciclo (review N → bugfix →
   review N+1) no relatório final, para rastreabilidade.

SUBAGENTES ESPECIALIZADOS:
- Disparar subagentes (ex.: code-review, security-review, explore) sempre que
  agregarem qualidade à revisão, por exemplo:
  - security-review para validar tratamento de dados sensíveis em
    bank/nickname (dados free-text sem FK).
  - explore para mapear todos os call-sites afetados pela remoção de
    cardLimit em budgets (task-8.0) e garantir 0 uso residual.
  - code-review para validar diffs de cada task isoladamente contra
    convenções de AGENTS.md.
- Cada subagente deve reportar achados com evidência (arquivo + linha); os
  achados devem ser consolidados no relatório único final, sem duplicação.

RESTRIÇÕES:
- NÃO implementar nenhuma correção nesta etapa de enriquecimento de prompt —
  a implementação de correções (bugfix) ocorre apenas na execução real deste
  prompt enriquecido, não durante a criação deste arquivo.
- NÃO aceitar evidência de execution_reports anteriores como prova suficiente;
  toda evidência deve ser reconfirmada no working tree atual.
- NÃO flexibilizar nenhum critério listado acima, mesmo que pareça de baixo
  impacto.

CRITÉRIO DE ACEITAÇÃO DESTE PROMPT (para quem executar):
- Entrega final = relatório de review no template padrão, com status
  explícito (APPROVED apenas quando 100% dos critérios acima forem
  satisfeitos), histórico completo do ciclo review → bugfix → review, e
  evidências verificáveis para cada linha de checklist.
```

---

## 3. Justificativa das adições (comparação lado a lado)

| Adição no prompt enriquecido | Por que foi adicionada |
|---|---|
| Papel explícito de "revisor sênior / code owner" | Ancora o tom e o rigor esperado, alinhado ao propósito da skill `review`. |
| Lista explícita de todos os artefatos do PRD (prd.md, techspec.md, tasks.md, 5 ADRs, 9 tasks, 2 execution reports) | Elimina ambiguidade sobre qual é a "fonte de verdade" do escopo; evita que o revisor valide contra documentação incompleta. |
| Instrução para tratar execution reports como evidência a ser **reconfirmada**, não aceita cegamente | Previne falso positivo: um relatório de execução anterior pode estar desatualizado frente ao working tree atual (regra de "working tree como fonte da verdade" já registrada como convenção deste repositório). |
| Referência obrigatória ao `review-report-template.md` e ao `canonical-bug-format.md` | Garante formato de saída consistente e imediatamente reutilizável pela skill `bugfix`, fechando o ciclo sem retrabalho de formatação. |
| Detalhamento de cada critério obrigatório (gaps, lacunas, falsos positivos, ressalvas, DoD, regras de negócio) com definição operacional de "0" | Transforma requisitos qualitativos em critérios mensuráveis e verificáveis, evitando interpretação frouxa de "sem flexibilização". |
| Menção explícita às regras de arquitetura Go (R0–R7, DMMF, zero comentários, sem panic) | Integra a validação técnica ao contrato de governança do repositório (`AGENTS.md`), já que o PRD altera `internal/billing`. |
| Definição do ciclo review → bugfix → review com critério de parada claro (APPROVED sem ressalvas) e registro cronológico das iterações | Resolve a ambiguidade de "quantas vezes repetir o ciclo" e garante rastreabilidade auditável. |
| Orientação sobre quando disparar subagentes especializados (security-review, explore, code-review) com exemplos concretos do próprio PRD | Atende ao pedido do usuário de "disparar subagentes quando agregarem qualidade", tornando o gatilho objetivo em vez de discricionário. |
| Restrição explícita de "não implementar nada nesta etapa" | Reforça o escopo do pedido original (apenas enriquecer e salvar o prompt), evitando que quem executar o prompt confunda esta etapa com a execução do review. |
| Critério de aceitação do próprio prompt | Dá ao executor um "Definition of Done" objetivo para saber quando a tarefa de review está de fato concluída. |

---

## 4. Variante alternativa (opcional)

Caso se deseje um review mais rápido, com menor custo de execução, uma variante viável seria restringir o escopo a apenas 1–2 tasks críticas por vez (ex.: task-2.0 domínio + task-6.0 invoice chain) em vez das 9 tasks completas em uma única passada, repetindo o mesmo prompt enriquecido por fatia. Isso reduziria o risco de contexto excessivo por execução, ao custo de múltiplas rodadas de review. **Não foi adotada como padrão** porque o usuário exigiu explicitamente "0 gaps" e "sem flexibilização" no escopo completo do PRD — fatiar o review poderia mascarar dependências cross-task (ex.: task-8.0 remoção de cardLimit depende de task-4.0/6.0).

---

*Arquivo gerado por `$prompt-enricher`. Nenhum código foi alterado. Próximo passo sugerido: executar o "Prompt Enriquecido" da Seção 2 para iniciar o ciclo review → bugfix → review.*
