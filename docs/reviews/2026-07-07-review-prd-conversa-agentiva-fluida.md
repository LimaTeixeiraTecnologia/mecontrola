<!-- review-prompt-enriched: 2026-07-07 -->

# Prompt Enriquecido — Revisão Criteriosa do PRD "Conversa Agentiva Fluida para Registro Financeiro"

## 1. Objetivo da Revisão

Executar `@.claude/skills/review/` (versão 1.3.0 ou superior) de forma **criteriosa, literal e sem flexibilização**, validando o estado atual da implementação **estritamente contra** `.specs/prd-conversa-agentiva-fluida/prd.md` (spec-version 3).

A revisão deve verificar se o produto entregue cobre **integralmente** os objetivos, requisitos funcionais, regras de negócio, critérios de aceite, decisões funcionais fechadas e restrições técnicas do PRD, com ênfase nos bounded contexts `internal/agents`, `internal/categories` e `internal/transactions` e no substrato compartilhado `internal/platform/{agent,memory,workflow,tool,scorer}`.

**Restrição fundamental:** esta tarefa é **apenas revisão**. Não implemente código, migrations, handlers, adapters, rotas, jobs, consumers, prompts, scorers ou qualquer artefato de produção. Se a revisão identificar defeitos, delegue a correção à skill `bugfix` e repita o ciclo até obter veredito `APPROVED`.

---

## 2. Escopo de Validação

### 2.1 Documento de referência obrigatório

- `.specs/prd-conversa-agentiva-fluida/prd.md` — única fonte de verdade funcional.
- Se houver `techspec.md` e `tasks.md` no mesmo diretório, usá-los como material de apoio apenas para rastreabilidade de implementação, **sem reduzir ou ampliar o escopo do PRD**.

### 2.2 Áreas de negócio a validar

1. **Continuidade conversacional (O-01, RF-01 a RF-03, RF-17, RF-26):**
   - Preservação de operação pendente entre turnos.
   - Avaliação da mensagem subsequente primeiro como resposta à pendência ativa.
   - Uma única pendência ativa por thread.
   - Estados de pendência como tipos fechados (`aguardando categoria`, `aguardando pagamento`, `aguardando cartão`, `aguardando data`, `aguardando confirmação`, `aguardando correção`, `cancelado`, `expirado`, `concluído`, `substituída`).

2. **Preservação de slots e menos atrito (O-02, RF-02, RF-04 a RF-07, RF-41):**
   - Slots preservados: tipo de operação, valor, descrição, data/competência, forma de pagamento, cartão, parcelas, candidatos de categoria, motivo da pendência, identificador de correlação.
   - Apenas uma pergunta por mensagem, exclusivamente sobre o dado faltante.
   - Nenhuma repetição de dados já coletados.
   - Correção natural de slots já preenchidos.

3. **Zero sucesso simulado (O-03, RF-21, RF-22, CA-06):**
   - Confirmação de sucesso somente após retorno real de tool/use case de escrita.
   - Em caso de erro, a resposta não pode afirmar sucesso.
   - Pendência preservada quando ainda for possível corrigir.

4. **Clarificação categorial segura (O-04, RF-10 a RF-14, RF-27 a RF-30, RF-35, RF-42, CA-04, CA-09, CA-15):**
   - `internal/categories` como autoridade canônica.
   - Proibição de fallback para categoria genérica, raiz sem folha, primeira da lista ou estimativa do LLM.
   - Toda opção apresentada deve conter raiz canônica (`id` + `slug`) e subcategoria folha canônica (`id` + `slug`).
   - Revalidação por contrato canônico antes de persistir, mesmo quando o usuário responde por nome ou número.

5. **Fluidez em português natural (O-05, RF-06, RF-15, RF-16, RF-24):**
   - Respostas curtas e elípticas interpretadas dentro do contexto pendente.
   - Preferência pela interpretação de pendência quando houver compatibilidade clara.
   - Esclarecimento sem descarte automático da operação quando a resposta for incompatível.
   - Mensagens em português do Brasil, curtas, sem markdown incompatível com WhatsApp e sem revelar infraestrutura interna.

6. **Cancelamento e expiração (RF-08, RF-09, RF-31, RF-32, RF-39, CA-05, CA-08, CA-11):**
   - Cancelamento inequívoco encerra sem escrita posterior.
   - Expiração após 30 minutos de inatividade com status fechado e mensagem clara.
   - Nova frase completa substitui pendência anterior, que fica com status `substituída` e não pode gerar escrita posterior.

7. **Auditabilidade (O-06, RF-23, RF-33, RF-34, CA-12):**
   - Evidência do motivo da clarificação, slot respondido, decisão tomada e desfecho.
   - Harness determinístico como fonte oficial para M-01 e M-06.
   - Scorers LLM-judged apenas como observabilidade complementar.

8. **Gate de confirmação obrigatório (O-07, RF-38 a RF-43, CA-01, CA-13, CA-14, CA-16, CA-17):**
   - Toda escrita (registro, edição, recorrência) exige confirmação humana explícita no turno imediatamente anterior.
   - Semântica estrita: aceite explícito efetiva; cancelamento descarta; ambiguidade gera um único reprompt e depois cancela.
   - Confirmação final não reintroduz pergunta por dados já preservados.
   - Edição preserva identificador e versão da transação alvo.
   - Recorrência delega à autoridade real de persistência (`internal/transactions`), sem reimplementar template no consumidor.

9. **Pipeline e arquitetura (RF-18, RF-19, RF-36, RF-37, Restrições Técnicas):**
   - Pipeline funcional `parse → validate → decide → persist → publish/respond`.
   - Decisões de negócio fora de handlers, consumers, jobs e tools finas.
   - Regras determinísticas testáveis quando não dependem de LLM.
   - Uso obrigatório de `go-implementation`, `mastra` e princípios DMMF (state-as-type, smart constructors, decisões puras).
   - Primitivos de thread, run, working memory, workflow e tool apenas em `internal/platform/{agent,memory,workflow,tool}` e no consumidor `internal/agents`.

### 2.3 Critérios de aceite a confrontar obrigatoriamente

Confrontar **um a um** os critérios CA-01 a CA-17 do PRD. Para cada um, registrar:

- `atendido` — há evidência concreta no código/testes/harness.
- `não atendido` — achado bloqueante.
- `não verificável` — falta evidência ou escopo não implementado; registrar como risco.

Critérios CA-01, CA-05, CA-06, CA-09, CA-13, CA-14, CA-16 e CA-17 devem ser considerados de severidade mínima `high` se não atendidos.

---

## 3. Definição de Pronto (DoD) Esperada

A revisão deve considerar que o DoD do PRD está 100% atendido **somente se**:

1. Todos os RF-01 a RF-43 estão implementados ou explicitamente identificados como fora de escopo com justificativa aceitável.
2. Todos os CA-01 a CA-17 possuem evidência de implementação e validação.
3. Existe harness determinístico cobrindo os cenários canônicos de retomada, substituição, ambiguidade, cancelamento, expiração, confirmação, edição e recorrência.
4. Testes de regressão existem para cada decisão pura e cada transição de estado crítica.
5. Não há strings livres governando transições críticas de estado.
6. Não há prompts, scorers ou texto livre de LLM como autoridade final para escolha de categoria.
7. Ferramentas e adapters são finos: sem SQL direto, sem regra de negócio e sem decisão categorial complexa dentro da tool.
8. Métricas M-01 a M-07 são mensuráveis e a fonte oficial é o harness determinístico.
9. Idempotência de escrita é verificável por identidade de inbound/correlação.
10. Auditabilidade por Thread/Run é verificável.
11. A política anterior de escrita sem confirmação está revogada e nenhum caminho de código a mantém.

---

## 4. Critérios de Qualidade da Revisão

A revisão deve ser conduzida com os seguintes critérios de qualidade absolutos:

- **0 gaps:** nenhum RF ou CA pode ser omitido da análise sem registro explícito.
- **0 lacunas:** toda decisão de negócio, transição de estado e validação de categoria deve ter evidência no código ou no teste.
- **0 falsos positivos:** não aprovar um item sem evidência concreta de implementação e validação.
- **0 ressalvas:** não aceitar "atendido com ressalvas"; o veredito final só pode ser `APPROVED` se todos os itens obrigatórios estiverem plenamente atendidos.
- **Rigor DMMF:** validar state-as-type, smart constructors, decisões puras (`Decide*`) e workflow pipeline.
- **Rigor de arquitetura:** verificar fronteiras de camada (`domain` puro, `application` sem IO concreto, `infrastructure` com implementações tecnológicas, `platform` como capacidade transversal).

---

## 5. Procedimento de Execução

### 5.1 Preparação

1. Carregar `.agents/skills/review/SKILL.md` e `.agents/skills/bugfix/SKILL.md`.
2. Carregar `AGENTS.md` e confirmar contrato de carga base.
3. Ler `.specs/prd-conversa-agentiva-fluida/prd.md` na íntegra.
4. Determinar o escopo do diff:
   - Se `AI_REVIEW_PRIOR_SHA` estiver definido (rodada pós-`bugfix`), revisar apenas `git diff "$AI_REVIEW_PRIOR_SHA"..HEAD`.
   - Caso contrário, usar base apropriada (ex.: `git diff --merge-base origin/main`) restrita aos arquivos alterados.
5. Aplicar orçamento de revisão:
   - `AI_REVIEW_MAX_FILES` (default 8)
   - `AI_REVIEW_MAX_DIFF_LINES` (default 400)
   - Se exceder, abrir apenas `git diff --stat` + `git diff --name-only`, amostrar arquivos por categoria de risco e retornar `BLOCKED` pedindo fatiamento, se necessário.

### 5.2 Revisão propriamente dita

1. Confrontar cada RF e CA conforme seção 2.
2. Carregar referências sob gatilho conforme `.agents/skills/agent-governance/triggers/<lang>.yaml`.
3. Priorizar correção, segurança, regressões, testes faltantes e lacunas de evidência.
4. Atribuir severidade canônica a cada achado: `critical`, `high`, `medium`, `low`.
5. Para bugs acionáveis, emitir lista no formato canônico `{ id, severity, file, line, reproduction, expected, actual }`, traduzindo severidade para o enum `critical`/`major`/`minor` do schema de bugfix (`critical→critical`, `high→major`, `medium→minor`, `low→minor`).
6. Se o chamador estiver em fluxo de remediação (`AI_REMEDIATION=1` ou `AI_REVIEW_PRIOR_SHA` definido) e houver bugs, instruir explicitamente o uso de `@.claude/skills/bugfix/` antes de nova rodada.

### 5.3 Uso de subagentes especializados

Disparar subagentes (`explore` ou `coder` read-only) para isolar análises de alto risco quando agregarem qualidade:

- **Subagente de arquitetura DMMF:** validar state-as-type, smart constructors e pureza das decisões em `internal/agents/domain` e `internal/platform/workflow`.
- **Subagente de categorias:** validar integração com `internal/categories` e ausência de fallback inseguro.
- **Subagente de transactions:** validar gate de confirmação, idempotência e autoridade de persistência em `internal/transactions`.
- **Subagente de harness/testes:** validar cobertura e rigor do harness determinístico.
- **Subagente de plataforma:** validar que primitivos de thread/run/workflow/tool não foram reimplementados fora de `internal/platform` e `internal/agents`.

Cada subagente deve retornar findings estruturados com severidade, arquivo, linha, impacto e hint de correção.

### 5.4 Veredito determinístico

| Condição | Veredito |
|---|---|
| Faltam diff, contexto necessário ou evidência de validação | `BLOCKED` |
| Há ao menos um achado `critical` ou `high` | `REJECTED` |
| Apenas achados `medium` ou `low` | `APPROVED_WITH_REMARKS` |
| Sem achados | `APPROVED` |

**O veredito final só pode ser `APPROVED` se todos os CA-01 a CA-17, RF-01 a RF-43, decisões D-01 a D-14 e restrições técnicas estiverem atendidos com evidência verificável.**

---

## 6. Ciclo Review → Bugfix → Review

Se o veredito for `REJECTED` ou `APPROVED_WITH_REMARKS` com achados que exijam correção:

1. **Review:** produzir findings canônicos e relatório estruturado.
2. **Bugfix:** invocar `@.claude/skills/bugfix/` passando a lista de bugs no formato canônico. O bugfix deve:
   - Corrigir a causa raiz.
   - Adicionar teste de regressão para cada bug.
   - Validar com `go build`, `go vet`, `go test -race -count=1` no pacote alterado e `golangci-lint run` quando disponível.
   - Produzir `bugfix_report.md` em `.specs/prd-conversa-agentiva-fluida/`.
3. **Review (repetição):** após o bugfix, reexecutar `@.claude/skills/review/` usando `AI_REVIEW_PRIOR_SHA` apontando para o commit anterior ao bugfix, focando apenas no delta da remediação.
4. **Iterar** até que o veredito seja `APPROVED` sem falsos positivos e em conformidade total com a especificação.

**Controle de profundidade:** respeitar `check-invocation-depth.sh` para evitar recursão infinita entre review e bugfix.

---

## 7. Formato de Saída Esperado

A resposta final da revisão deve conter, no mínimo, um bloco estruturado com:

```markdown
## Veredito

`{APPROVED | APPROVED_WITH_REMARKS | REJECTED | BLOCKED}`

## Resumo Executivo

- Total de RF avaliados: 43
- Total de CA avaliados: 17
- Achados críticos: N
- Achados high: N
- Achados medium: N
- Achados low: N
- RF não atendidos: lista
- CA não atendidos: lista
- Riscos residuais: lista

## Files Reviewed

- `caminho/relativo/1.go`
- `caminho/relativo/2.go`
...

## Referências Carregadas

- `.agents/skills/agent-governance/triggers/go.yaml`
- `.agents/skills/go-implementation/SKILL.md`
- `.agents/skills/mastra/SKILL.md`
...

## Achados

| ID | Severidade | Arquivo | Linha | Impacto | Hint de Correção |
|---|---|---|---|---|---|
| ... | ... | ... | ... | ... | ... |

## Validações Executadas

- `go build ./...`
- `go vet ./...`
- `go test -race -count=1 ./internal/agents/... ./internal/categories/... ./internal/transactions/... ./internal/platform/...`
- `golangci-lint run ...`

## Decisões Funcionais Fechadas Verificadas

- D-01: ...
- D-02: ...
...

## Critérios de Aceite Verificados

- CA-01: `atendido` / `não atendido` / `não verificável` — evidência
- CA-02: ...
...
```

Se o modo `--auto-review` estiver ativo, persistir o relatório em `evidence/<task>/review.md` e validar com o validador apropriado.

---

## 8. Tratamento de Erros e Bloqueios

- **Sem diff disponível:** retornar `BLOCKED` e solicitar o alvo de revisão.
- **Orçamento excedido:** retornar `BLOCKED` com estatísticas do diff e pedir fatiamento.
- **Contexto externo indisponível:** marcar como risco ou `blocked`, nunca assumir implementação.
- **Ambiguidade de severidade:** usar `.agents/skills/agent-governance/references/multiple-choice-protocol.md` antes de decidir.
- **Conflito entre PRD e implementação:** prevalece o PRD; registrar o drift explicitamente.

---

## 9. Checklist Final Antes do Veredito

Antes de emitir o veredito, confirmar:

- [ ] Todos os RF-01 a RF-43 foram avaliados.
- [ ] Todos os CA-01 a CA-17 foram avaliados.
- [ ] Todas as decisões D-01 a D-14 foram verificadas.
- [ ] Todas as métricas M-01 a M-07 são mensuráveis e têm fonte oficial.
- [ ] Nenhum gap ou lacuna foi omitido.
- [ ] Nenhum falso positivo foi registrado como atendido.
- [ ] Nenhuma ressalva foi aceita como suficiente.
- [ ] Harness determinístico é a fonte oficial de aceite para retomada e confusão.
- [ ] Nenhuma escrita ocorre sem confirmação humana explícita.
- [ ] Nenhuma categoria é persistida sem raiz + folha canônicos validados.
- [ ] Primitivos de plataforma não foram reimplementados fora de `internal/platform` e `internal/agents`.
- [ ] Testes de regressão cobrem bugs encontrados (após ciclo bugfix).
- [ ] Validações de build, vet, test e lint foram executadas ou explicitamente registradas como não aplicáveis.

---

## 10. Notas sobre o Prompt Original vs. Enriquecido

| Aspecto | Prompt Original | Adição no Enriquecido |
|---|---|---|
| Escopo | "validar estritamente contra o PRD" | Listagem explícita de cada RF, CA, decisão e métrica a ser confrontada |
| DoD | "DoD 100%" | Definição detalhada do que compõe o DoD para este PRD |
| Qualidade | "0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas" | Checklist operacional e critérios de severidade canônica |
| Ciclo | "review → bugfix → review" | Procedimento passo a passo com controle de profundidade e uso de `AI_REVIEW_PRIOR_SHA` |
| Subagentes | "dispare subagentes especializados" | Sugestão concreta de 5 subagentes com escopo definido |
| Formato de saída | Implícito | Template markdown completo com veredito, achados, validações e checklist |
| Restrição | "Não implemente nada" | Reforçada no objetivo e em múltiplas seções |

---

## 11. Comando Sugerido de Invocação

```text
@.claude/skills/review/ --auto-review
```

Com variáveis de ambiente, se aplicável:

```bash
export AI_REVIEW_MAX_FILES=8
export AI_REVIEW_MAX_DIFF_LINES=400
export AI_REMEDIATION=0
```

Se estiver em rodada pós-bugfix:

```bash
export AI_REVIEW_PRIOR_SHA=<sha-antes-do-bugfix>
export AI_REMEDIATION=1
```
