# Prompt de Revisão: Orçamento Retroativo Conversacional e Mês por Extenso

- **Data:** 2026-07-09
- **PRD:** `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/prd.md`
- **Artefatos relacionados:** `techspec.md`, `tasks.md`, ADRs no mesmo diretório
- **Módulo alvo:** `internal/agents` sobre o substrato `internal/platform`

---

## Prompt Original

> Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso`. Critérios obrigatórios: todos os critérios de aceite atendidos (implementados); DoD 100% atendido (implementados); 0 gaps; 0 lacunas; 0 falsos positivos; 0 ressalvas; todas as regras de negócio atendidas (implementadas). Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação. Dispare subagentes especializados quando agregarem qualidade à revisão. Não implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em `docs/reviews/`.

---

## Prompt Enriquecido (Prompt de Execução)

<context>
Você é um revisor técnico sênior atuando no monolito modular Go do MeControla. O bounded context alvo é `internal/agents`, construído sobre o substrato reutilizável `internal/platform` (`internal/platform/agent`, `internal/platform/workflow`, `internal/platform/memory`, `internal/platform/tool`, `internal/platform/outbox`, `internal/platform/worker`).

A fonte da verdade para esta revisão é o diretório `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/`, contendo:
- `prd.md` — requisitos funcionais (RF-01 a RF-30), critérios de aceite, regras de negócio, fluxos e decisões (D1–D10);
- `techspec.md` — especificação técnica e contratos;
- `tasks.md` — tarefas de implementação;
- ADRs (`adr-001` a `adr-005`) — decisões arquiteturais vinculantes.
</context>

<goal>
Execute a skill `.claude/skills/review/` de forma criteriosa, inflexível e sem falsos positivos para validar se a implementação do PRD **Orçamento Retroativo Conversacional e Mês por Extenso** está 100% conforme com a especificação.

O resultado final deve ser um veredito explícito:
- `APPROVED` — somente se todos os critérios de aceite, todo o DoD, todas as regras de negócio e todos os RFs estiverem implementados e validados, com 0 gaps, 0 lacunas, 0 ressalvas e 0 falsos positivos;
- `REJECTED` — se qualquer não conformidade for encontrada, acompanhada de evidência precisa e plano de correção.
</goal>

<rules>
- Leia integralmente `prd.md`, `techspec.md`, `tasks.md` e todos os ADRs do diretório `.specs/prd-orcamento-retroativo-conversacional-e-mes-por-extenso/`.
- Valide cada requisito funcional (RF-01 a RF-30), cada critério de aceite, cada decisão de design (D1–D10) e cada item do DoD contra o working tree real.
- Verifique se **todos** os critérios de aceite do PRD foram implementados.
- Verifique se o **DoD 100%** foi atendido (implementado).
- Exija **0 gaps**, **0 lacunas**, **0 falsos positivos**, **0 ressalvas**.
- Verifique se **todas** as regras de negócio foram implementadas.
- Valide conformidade arquitetural:
  - Tool fina (`create_budget`) sem regra de negócio, SQL direto ou branching de domínio;
  - Funções `Decide*` puras, recebendo `now time.Time`, sem relógio interno;
  - Estados de espera do diálogo modelados como tipos fechados e persistidos no `Snapshot` do kernel;
  - Kernel `internal/platform/workflow` permanece genérico (sem import de domínio ou tipo semântico);
  - `internal/platform/agent` usado conforme fluxo canônico (`InboundRequest → AgentRuntime.Execute → ThreadGateway.GetOrCreate → RunStore.Insert → AgentRegistry.Resolve → Agent.Execute → MessageStore.Append → closeRun`).
- Valide cobertura de testes:
  - Testes unitários com testify/suite e mocks via `.mockery.yml`;
  - Testes de integração nos workflows e tools;
  - Avaliação real-LLM (`RUN_REAL_LLM=1`) nos cenários-chave (criação com distribuição, retroativo, resolução de mês, retrospectiva), com threshold ≥ 0.90.
- Valide observabilidade:
  - Métrica `agent_tool_invocations_total{tool="create_budget"}` deve existir;
  - Runs falhos devem ser auditáveis com `error` preenchido;
  - Cardinalidade controlada: proibido `user_id` ou `competence` como label; labels permitidos são enums fechados (`agent_id`, `channel`, `workflow`, `tool`, `status`, `outcome`).
- Se encontrar qualquer problema, marque `REJECTED`, detalhe a não conformidade com referência precisa (arquivo:linha ou seção do PRD) e inicie o ciclo de correção.
</rules>

<workflow>
1. **Mapeamento**: construa uma matriz de rastreabilidade cobrindo RF-01 a RF-30, critérios de aceite, regras de negócio e itens do DoD.
2. **Evidência**: para cada item, aponte o arquivo e a linha de implementação correspondente, ou justifique por que está ausente.
3. **Subagentes**: dispare subagentes especializados (`explore`, `coder`, `review`) quando agregarem qualidade — por exemplo:
   - um subagente para varredura de `internal/agents` (tools, workflows, handlers, producers);
   - um subagente para `internal/budgets` (use cases de criação/ativação, constraints, repositórios);
   - um subagente para testes, observabilidade e gates de governança.
4. **Veredito**: emita `APPROVED` somente se 100% dos itens estiverem implementados, testados e validados. Caso contrário, emita `REJECTED`.
5. **Ciclo de correção**: se `REJECTED`, use `.claude/skills/bugfix/` para corrigir a causa raiz com teste de regressão obrigatório. Após a correção, repita este prompt de revisão do início até obter `APPROVED`.
</workflow>

<output_format>
Produza um relatório em markdown em português do Brasil com as seções obrigatórias:

1. **Resumo Executivo**: veredito (`APPROVED`/`REJECTED`) em destaque, com uma frase de justificativa.
2. **Matriz de Rastreabilidade**: tabela com colunas `Item` (RF/critério/DoD/regra), `Status` (✅/❌/⚠️), `Evidência` (caminho:linha), `Observação`.
3. **Não Conformidades Encontradas**: lista numerada com severidade (`bloqueante`/`alta`/`média`/`baixa`), descrição, referência ao PRD/seção e sugestão de correção.
4. **Ações de Correção**: se houver, itens gerados para `.claude/skills/bugfix/`, com causa raiz e teste de regressão esperado.
5. **Evidências de Validação**: comandos executados (`go build`, `go vet`, `go test`, `golangci-lint run`, `RUN_REAL_LLM=1 ...`), outputs relevantes e resultados de testes.
6. **Considerações Finais**: declare explicitamente se há riscos residuais; se zero, afirme: "Nenhuma ressalva, gap ou lacuna identificada."

O veredito `APPROVED` só pode aparecer se a seção **Não Conformidades Encontradas** estiver vazia e você puder afirmar, com evidência, que:
- todos os critérios de aceite estão atendidos;
- o DoD está 100% atendido;
- há 0 gaps, 0 lacunas, 0 falsos positivos, 0 ressalvas;
- todas as regras de negócio estão implementadas.
</output_format>

<constraints>
- Não implemente código neste turno; apenas revise e, se necessário, delegue ao bugfix.
- Não aceite soluções parciais, placeholders, `TODO`, `FIXME` ou comentários como evidência de implementação.
- Não assuma que algo funciona sem teste, build ou lint passando.
- Sempre que possível, execute `go build`, `go vet`, `go test -race -count=1` e `golangci-lint run` nos pacotes afetados e registre os resultados.
- Só declare `APPROVED` após evidência concreta e reproducível.
</constraints>

---

## Justificativas das Adições

| Adição | Motivo |
|--------|--------|
| `<context>` com bounded context e substrato | Evita que o revisor ignore as fronteiras arquiteturais (`internal/platform` genérico vs. `internal/agents` consumidor). |
| `<goal>` com veredito binário | Torna o critério de parada objetivo: só `APPROVED` se 100% atendido. |
| `<rules>` detalhadas | Converte os critérios obrigatórios do usuário em checklist acionável (RFs, D1–D10, DoD, arquitetura, testes, observabilidade). |
| `<workflow>` com subagentes | Garante revisão profunda e paralelizável, sem perder o foco no resultado final. |
| `<output_format>` estruturado | Força evidência documentada e evita respostas vagas ou vereditos sem fundamentação. |
| `<constraints>` anti-falso-positivo | Impede que o revisor aprove parcialmente ou baseado em suposições. |
| Instrução de ciclo `review → bugfix → review` | Materializa a exigência do usuário de iteração até `APPROVED` real. |
