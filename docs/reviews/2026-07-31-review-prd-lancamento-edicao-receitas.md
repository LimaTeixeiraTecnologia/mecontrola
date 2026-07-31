# Prompt Enriquecido — Review Crítico do PRD `prd-lancamento-edicao-receitas`

<!-- gerado por: prompt-enricher | data: 2026-07-31 | não executar automaticamente -->

## Prompt original (entrada do usuário)

> Execute `@.claude/skills/review/` de forma criteriosa e sem flexibilização, validando estritamente
> contra `.specs/prd-lancamento-edicao-receitas`. Critérios obrigatórios: todos os critérios de
> aceite atendidos, DoD 100% atendido, 0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas, todas as
> regras de negócio atendidas. Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e
> repita o ciclo review → bugfix → review até obter `APPROVED`, sem falsos positivos e em
> conformidade total com a especificação. Dispare subagentes especializados quando agregarem
> qualidade à revisão.

## Lacunas identificadas no prompt original

- Não define **escopo do diff** a revisar (branch atual vs. `origin/main`, ou arquivos citados nas
  task files da feature).
- Não lista as **task files/RFs específicos** contra os quais confrontar critério a critério.
- Não define **quando parar** o ciclo review→bugfix→review (limite de rodadas, o que fazer se um
  achado for `blocked` por dependência externa).
- Não define **onde e como registrar evidência** de cada rodada (a skill `review` já define template
  e path canônico — precisa ser citado explicitamente para não divergir).
- "Sem flexibilização" e "0 ressalvas" conflitam com o veredito determinístico da skill `review`
  (`APPROVED_WITH_REMARKS` existe como estado legítimo para achados `medium`/`low`) — é preciso
  deixar explícito que o objetivo é **forçar o ciclo até `APPROVED` puro**, tratando também achados
  `medium`/`low` como bloqueantes para efeito desta revisão, não apenas `critical`/`high`.

---

## Tarefa (contexto completo para execução futura)

<context>
Repositório `mecontrola` (Go, arquitetura modular DDD com governança AGENTS.md/CLAUDE.md e
regras hard `.claude/rules/*.md`). A feature em revisão é **Lançamento e Edição de Receitas em
Linguagem Natural**, documentada em `.specs/prd-lancamento-edicao-receitas/`:

- `prd.md` (spec-version 1) — US-01 (lançamento) e US-02 (edição de receita), RF-01 a RF-16+
  (lançamento) e requisitos de edição na sequência do arquivo.
- `techspec.md` — arquitetura técnica, contratos, riscos.
- 3 ADRs: `adr-001-resolucao-deterministica-data.md`, `adr-002-confirmacao-receita-data-condicional.md`,
  `adr-003-gate-golden-por-grupo.md`.
- `tasks.md` + 5 task files: `task-1.0-datas-deterministicas-parseinputdate.md`,
  `task-2.0-contrato-data-verbatim-tools.md`, `task-3.0-confirmacao-receita-data-condicional.md`,
  `task-4.0-golden-lancamento-gate.md`, `task-5.0-golden-edicao-gate.md`.
- Relatórios de execução já presentes: `1.0_execution_report.md` a `4.0_execution_report.md`
  (verificar se `5.0_execution_report.md` existe; se não existir, é sinal de tarefa não concluída
  ou não reportada — tratar como gap a investigar, não presumir).

Regras de negócio centrais do PRD a validar linha a linha contra o código real (não apenas contra
os relatórios de execução, que podem estar desatualizados ou otimistas):

- RF-01: 9 famílias de intenção de receita reconhecidas via LLM-first (guard determinístico é só
  atalho, nunca único caminho — RF-16).
- RF-02: origem gravada com termo **literal** do usuário, nunca paráfrase.
- RF-03/RF-04: valor numérico, separador BR, gíria, por-extenso simples e composto → centavos.
- RF-05/RF-06: resolução determinística de data, incluindo as 3 formas novas (`semana passada`,
  `mês passado`, `dia N`) sem fallback silencioso para o dia corrente.
- RF-07: ausência de data → dia corrente.
- RF-08: clarify pede só o dado ausente, nunca repete dado já identificado.
- RF-09: clarify de categoria quando subcategoria-folha não resolvida.
- RF-10/RF-11: bloco de confirmação oficial antes de gravar; grava só com confirmação explícita,
  uma única vez; cancelamento descarta sem efeito.
- RF-12: mensagem de sucesso oficial pós-gravação.
- RF-13: idempotência por `wamid`/`itemSeq` (replay não duplica).
- RF-14: limites de valor (`≤0` ou `>R$10.000.000,00`) e origem vazia/ilegível bloqueiam gravação
  com orientação de correção.
- RF-15: forma de pagamento não é capturada nem persistida para receita.
- RF-16: arquitetura LLM-first é mandatória; guard determinístico não pode ser único caminho.
- Requisitos de edição de receita (US-02, sequência após RF-16 em `prd.md`) — ler o restante do
  arquivo para RFs de edição, localização de lançamento, desambiguação e atualização.

Regras de plataforma que também se aplicam por o código tocar `internal/agents` e possivelmente
`internal/platform/agent`/`workflow`: `R-AGENT-WF-001` (`.claude/rules/agent-workflows-tools.md`),
`R-WF-KERNEL-001` (`.claude/rules/workflow-kernel.md`), `R-ADAPTER-001` (`.claude/rules/go-adapters.md`),
`R-DTO-VALIDATE-001` (`.claude/rules/input-dto-validate.md`), `R-TESTING-001` (`.claude/rules/go-testing.md`).
Violação de qualquer regra `[HARD]` conta como achado `critical`/`high` na revisão, mesmo que não
esteja listada explicitamente como RF do PRD.
</context>

<task>
1. **Não implementar nada.** Este arquivo é apenas o roteiro — a execução do ciclo
   review → bugfix → review acontece em uma sessão separada, quando o usuário disparar este prompt.
2. Ao executar, invocar a skill `review` (`.claude/skills/review/SKILL.md`) com escopo de diff
   restrito às mudanças da feature `prd-lancamento-edicao-receitas` (branch local vs.
   `origin/main`, ou os arquivos citados nas 5 task files se a feature já estiver mergeada e a
   revisão for "as-built").
3. Cumprir a Etapa 1.5 da skill `review` (RF-14 da própria skill): para **cada** task file
   (`task-1.0` a `task-5.0`), ler `## Critérios de Sucesso`/`## Critérios de Aceite` e confrontar
   cada critério contra o diff/código real, mesmo que o diff não toque os arquivos citados.
   Marcar `atendido` (com evidência de arquivo:linha), `não atendido` (achado bloqueante mínimo
   `high`) ou `não verificável pelo diff` (registrar como risco residual).
4. Confrontar **todos** os RFs do `prd.md` (lançamento RF-01–RF-16+ e os RFs de edição da US-02)
   e as decisões dos 3 ADRs contra o código real — não contra os `execution_report.md`, que são
   autodeclarados pela sessão de implementação e podem conter falso positivo.
5. Verificar explicitamente:
   - Existência e cobertura do gate golden real-LLM por grupo de intenção (ADR-003), com limiar
     ≥ 0,90 e 0 falso-sucesso, conforme métrica de sucesso do PRD.
   - Ausência de fallback silencioso de data (RF-06) — testar mentalmente os 3 casos novos contra
     a implementação de `parse_input_date` (ou equivalente) e seus testes.
   - Confirmação obrigatória e idempotência (RF-11/RF-13) no workflow de escrita de transação.
   - Regras `[HARD]` de plataforma (Decide* puro, zero comentários, tools finas, sem SQL fora de
     adapter, estados como tipos fechados) nos arquivos tocados.
   - Presença e completude de `5.0_execution_report.md` (ou report equivalente da task 5.0); se
     ausente, registrar como achado de DoD incompleto — não presumir que a tarefa foi concluída.
6. Disparar subagentes especializados quando agregarem qualidade, por exemplo:
   - Um agente `Explore` para varrer todos os arquivos tocados pelas 5 tasks e mapear cobertura de
     teste antes da revisão principal.
   - Um agente de revisão adversarial (ex.: `reviewer` ou `code-review` em effort alto) para uma
     segunda opinião independente sobre os achados antes de finalizar o veredito.
   - Um agente para rodar e validar o gate golden real-LLM (`RUN_REAL_LLM=1`) separadamente,
     dado o padrão do projeto de exigir validação real-LLM em vez de apenas mocks
     (ver memória do projeto: validação real-LLM obrigatória em fixes do agent).
7. Para efeito desta revisão, tratar o veredito-alvo como **apenas `APPROVED` sem nenhum achado**
   — inclusive achados `medium`/`low` que normalmente resultariam em `APPROVED_WITH_REMARKS` devem
   ser corrigidos antes de encerrar o ciclo, já que o usuário exigiu explicitamente "0 ressalvas".
8. Se houver achados (`critical`, `high`, `medium` ou `low`), emitir a lista no formato canônico
   `bug-schema.json` (traduzindo severidade via `severity-mapping.md`) e invocar a skill `bugfix`
   (`.claude/skills/bugfix/SKILL.md`) para corrigir pela causa raiz, com teste de regressão
   obrigatório por bug e evidência de validação (`bugfix_report.md` em
   `.specs/prd-lancamento-edicao-receitas/`).
9. Repetir o ciclo review → bugfix → review — usando `AI_REVIEW_PRIOR_SHA` para escopar cada nova
   rodada apenas ao delta da remediação — até obter veredito `APPROVED` limpo, sem achados
   residuais de nenhuma severidade.
10. Documentar o resultado final: veredito, lista de rodadas executadas, achados por rodada e como
    foram corrigidos, comandos de validação rodados (build/vet/test race/lint conforme risco da
    camada tocada — ver `AGENTS.md` seção `Validação`), e confirmação de que os testes/gate golden
    passam.
</task>

<rules>
- Seguir `AGENTS.md` e `CLAUDE.md` como contrato raiz; carregar apenas as referências exigidas
  pelos gatilhos da mudança tocada (economia de contexto).
- Não flexibilizar nenhuma regra `[HARD]` por conveniência, ferramenta ou deadline.
- Não presumir sucesso de implementação anterior com base apenas em `execution_report.md`
  autodeclarado — validar contra o código e os testes reais.
- Se algum critério do PRD ou de task file for materialmente ambíguo entre bloqueante e soft,
  aplicar o protocolo de múltipla escolha (`multiple-choice-protocol.md`) em vez de decidir
  silenciosamente.
- Se incerto sobre qualquer achado, dizer isso explicitamente em vez de forçar um veredito.
- Não realizar ações destrutivas de git (push, reset --hard, force) sem pedido explícito.
- Respeitar limite de duas tentativas de remediação por bug na skill `bugfix`; se excedido, marcar
  `failed` e registrar diagnóstico em vez de insistir indefinidamente.
</rules>

<format>
Ao ser executado, o resultado esperado é:
1. Um relatório de revisão por rodada, no formato padrão da skill `review` (`verdict`,
   `files_reviewed`, `refs_loaded`, `findings`, `residual_risks`, `validations_run`).
2. Um `bugfix_report.md` por rodada de correção (quando houver achados), salvo em
   `.specs/prd-lancamento-edicao-receitas/`.
3. Um resumo final em português confirmando veredito `APPROVED`, 0 achados residuais, e link/paths
   para todos os relatórios gerados.
</format>

<check>
Antes de encerrar o ciclo, confirmar:
- [ ] Todos os RFs de `prd.md` (lançamento + edição) confrontados individualmente contra o código.
- [ ] Todos os critérios de aceite das 5 task files confrontados individualmente.
- [ ] Gate golden real-LLM executado e ≥ 0,90 por grupo, 0 falso-sucesso (não apenas citado no
      report — rodar e validar de fato).
- [ ] `5.0_execution_report.md` (ou equivalente) existe e está completo.
- [ ] Nenhuma violação de regra `[HARD]` de plataforma nos arquivos tocados.
- [ ] Veredito final da skill `review` é `APPROVED` puro (sem achados de nenhuma severidade).
- [ ] Build/vet/test race/lint proporcionais ao risco da camada tocada, todos verdes.
</check>

---

## Justificativa das adições (comparação com o prompt original)

| Adição | Por quê |
|---|---|
| Escopo de diff explícito | A skill `review` exige determinar o escopo antes de tudo (Etapa 1.2); sem isso o review pode revisar o repo inteiro ou nada. |
| Lista de RFs e task files nomeados | Fonte real do PRD lida (`prd.md`, `tasks.md`) — evita que a revisão futura precise redescobrir a estrutura da feature. |
| Instrução para não confiar em `execution_report.md` | Alinhado à memória do projeto: "0 falso positivo é inegociável" e "validação real-LLM obrigatória" — reports autodeclarados não são evidência suficiente. |
| Reinterpretação de "0 ressalvas" | O veredito determinístico da skill `review` tem 4 estados; sem esclarecer, um `APPROVED_WITH_REMARKS` poderia ser aceito incorretamente como conforme ao pedido do usuário. |
| Subagentes sugeridos concretamente | O pedido original já autorizava subagentes "quando agregarem qualidade" — aqui ficam nomeados casos de uso reais (exploração, segunda opinião adversarial, validação real-LLM separada). |
| Checklist final (`<check>`) | Critério de aceitação mensurável, exigido pelas práticas oficiais do projeto (`CLAUDE.md` — "fornecer um check pass/fail antes de encerrar uma tarefa"). |
| Regra de não-flexibilização e limite de remediação | Evita loop infinito de bugfix (proibido por `governance.md`) mantendo, ainda assim, o rigor total pedido. |

---

## Prompt original preservado (para referência rápida ao disparar a execução)

```
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente
contra .specs/prd-lancamento-edicao-receitas
Critérios obrigatórios:
* Todos os critérios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* 0 ressalvas
* Todas Regras de negócio atendidos (implementados)
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review
até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
```
