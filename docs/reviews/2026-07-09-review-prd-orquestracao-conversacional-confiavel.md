# Prompt de Revisao: Orquestracao Conversacional Confiavel

- **Data:** 2026-07-09
- **PRD:** `.specs/prd-orquestracao-conversacional-confiavel/prd.md`
- **Artefatos relacionados:** `.specs/prd-orquestracao-conversacional-confiavel/techspec.md`, ADRs `adr-001` a `adr-005`
- **Observacao:** `tasks.md` nao foi encontrado no diretorio do PRD; se aparecer em outro local, deve ser localizado e lido antes do veredito.
- **Modulo alvo predominante:** `internal/agents` sobre `internal/platform/{agent,llm,memory,workflow,tool,scorer}`
- **Escopo deste arquivo:** prompt enriquecido para execucao posterior. Nenhum review ou bugfix foi executado na criacao deste artefato.

---

## Analise do Prompt Original

| Dimensao | Prompt original | Enriquecimento aplicado |
|---|---|---|
| Objetivo | Revisar estritamente contra `.specs/prd-orquestracao-conversacional-confiavel`. | Define veredito binario operacional: somente `APPROVED` encerra; qualquer outro resultado exige correcao e nova revisao. |
| Fonte da verdade | PRD indicado por caminho de diretorio. | Explicita leitura obrigatoria de `prd.md`, `techspec.md` e cinco ADRs; `tasks.md` deve ser buscado se existir fora do diretorio. |
| Criterios | Todos os criterios, DoD, regras de negocio, zero gaps/lacunas/falsos positivos/ressalvas. | Converte em matriz de rastreabilidade para RF-01..RF-57, criterio de sucesso primario, restricoes tecnicas, DoD inferido dos artefatos e ADRs. |
| Ciclo de correcao | Usar `bugfix` e repetir ate `APPROVED` caso haja problema. | Define entrada canonica para `bugfix`, preservando a regra de que `APPROVED_WITH_REMARKS`, `BLOCKED` e `REJECTED` nao satisfazem este contrato. |
| Subagentes | Disparar quando agregarem qualidade. | Sugere subagentes especializados por area: guards/runtime, cardId/month/workflows, scorers/golden/gates, governanca/testes. |
| Saida | Nao especificada alem do resultado esperado. | Exige relatorio markdown com veredito, matriz, achados, bugs canonicos, validacoes e riscos residuais. |

### Ambiguidades tratadas

- O pedido atual diz para nao implementar nada; portanto este arquivo apenas cria o prompt. O prompt enriquecido abaixo, quando executado em outro turno, pode acionar `bugfix` como parte do ciclo solicitado.
- O diretorio do PRD nao contem `tasks.md`; o executor deve registrar isso como ausencia de artefato, nao inventar tarefas.
- O termo "DoD" deve ser resolvido pelos itens de pronto declarados nos artefatos do PRD, techspec e ADRs, alem dos gates obrigatorios do repositorio.
- "0 falsos positivos" vale nos dois sentidos: nao aprovar sem evidencia e nao apontar defeito sem base concreta em especificacao, codigo, teste ou gate.

---

## Prompt Original

> Use `@.claude/skills/review/` de forma criteriosa e sem flexibilizacao, validando estritamente contra `.specs/prd-orquestracao-conversacional-confiavel`.
>
> Criterios obrigatorios:
> - Todos os criterios de aceite atendidos (implementados).
> - DoD 100% atendido (implementados).
> - 0 gaps.
> - 0 lacunas.
> - 0 falsos positivos.
> - 0 ressalvas.
> - Todas Regras de negocio atendidos (implementados).
>
> Caso encontre qualquer problema, utilize `@.claude/skills/bugfix/` e repita o ciclo review -> bugfix -> review ate obter APPROVED, sem falsos positivos e em conformidade total com a especificacao.
>
> Dispare subagentes especializados quando agregarem qualidade a revisao.
>
> Nao implemente nada. Apenas crie/enriqueca o prompt e salve o arquivo em `docs/reviews/`.

---

## Prompt Enriquecido

<goal>
Executar uma revisao criteriosa, estrita e sem flexibilizacao da implementacao do PRD "Orquestracao Conversacional Confiavel", usando `.claude/skills/review/`, ate obter um veredito final `APPROVED` real.

Done significa: todos os RF-01..RF-57, todos os criterios de aceite, todo o DoD, todas as regras de negocio, techspec e ADRs estao implementados e validados com evidencia concreta; existem 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas.
</goal>

<context>
Voce esta no repositorio MeControla, um monolito modular Go com regras canonicas em `AGENTS.md`.

A fonte da verdade desta revisao e:
- `.specs/prd-orquestracao-conversacional-confiavel/prd.md`;
- `.specs/prd-orquestracao-conversacional-confiavel/techspec.md`;
- `.specs/prd-orquestracao-conversacional-confiavel/adr-001-guard-chain-cor.md`;
- `.specs/prd-orquestracao-conversacional-confiavel/adr-002-runtime-robustez-truncamento.md`;
- `.specs/prd-orquestracao-conversacional-confiavel/adr-003-cardid-provenance.md`;
- `.specs/prd-orquestracao-conversacional-confiavel/adr-004-scorers-comportamentais.md`;
- `.specs/prd-orquestracao-conversacional-confiavel/adr-005-golden-harness-gate.md`.

O diretorio do PRD nao contem `tasks.md` na data de criacao deste prompt. Antes de revisar, procure por task file relacionada em `.specs/`, `docs/` ou evidencias da branch. Se nao existir, registre a ausencia como contexto; nao invente tarefas.

O escopo tecnico predominante envolve `internal/agents` consumindo o substrato `internal/platform/{agent,llm,memory,workflow,tool,scorer}`, com foco em:
- cadeia de guardas conversacionais no padrao Chain of Responsibility;
- roteamento deterministico e anti-alucinacao;
- escritas financeiras com confirmacao e idempotencia;
- proveniencia deterministica de `cardId`;
- competencia de mes e `monthRefKind`;
- robustez operacional do runtime;
- scorers comportamentais e observabilidade;
- golden set e harness de avaliacao;
- gates pre-deploy e pos-deploy;
- fechamento da divida R5.26 de identificadores `_`-prefixados;
- endurecimento de workflows, pendencias e estados fechados.
</context>

<rules>
- Carregue e siga `AGENTS.md` como fonte canonica de governanca.
- Carregue `.claude/skills/review/SKILL.md` e execute o procedimento completo da skill.
- Para qualquer correcao gerada por achado, carregue `.claude/skills/bugfix/SKILL.md`; se a correcao tocar Go, carregue tambem as skills obrigatorias do repositorio para Go.
- Leia integralmente `prd.md`, `techspec.md` e os cinco ADRs antes de emitir qualquer veredito.
- Construa uma matriz de rastreabilidade cobrindo RF-01..RF-57, criterio de sucesso primario composto, restricoes tecnicas, fora de escopo, suposicoes da techspec, ADR-001..ADR-005 e DoD derivado dos artefatos.
- `APPROVED_WITH_REMARKS` nao satisfaz este pedido. Qualquer ressalva, risco residual acionavel, evidencia ausente, validacao nao executada sem justificativa bloqueante ou item nao comprovado deve manter o fluxo em correcao/revisao.
- Nao aceite `TODO`, `FIXME`, comentario, intencao de codigo, teste inexistente, log manual ou suposicao como evidencia de implementacao.
- Nao assuma conformidade por nome de arquivo. Abra o codigo, teste e fixture correspondente.
- Nao gere falso positivo: cada achado deve apontar referencia concreta na especificacao e evidencia concreta no codigo, teste, fixture, gate ou ausencia verificavel.
- Nao gere falso negativo: se um requisito nao tiver evidencia suficiente, trate como falha de verificacao e nao aprove.
- Preserve o working tree do usuario: nao reverta mudancas nao relacionadas.
- Se o diff ou a base de comparacao nao estiver clara, determine o escopo com `git diff --stat`, `git diff --name-only`, branch atual e base remota apropriada. Se ainda assim nao houver escopo revisavel, retorne `BLOCKED`, nao `APPROVED`.
</rules>

<review_scope>
Valide obrigatoriamente os grupos do PRD:
- Grupo A, RF-01..RF-06: cadeia de guardas conversacionais, absorcao do `MultiItemGuard`, multi-item pre-LLM, formato brasileiro de valor e observabilidade por handler.
- Grupo B, RF-07..RF-10: roteamento deterministico, reinvocacao de tools em follow-up, proibicao de sucesso sem tool real e resposta WhatsApp sem termos internos.
- Grupo C, RF-11..RF-15: tool fina, confirmacao verbatim, retomada de workflow pendente, idempotencia e `Snapshot` persistido antes de clarificar.
- Grupo D, RF-16..RF-18: `resolve_card`/`list_cards`, proibicao de `cardId` fabricado e fallback quando cartao nao for encontrado.
- Grupo E, RF-19..RF-21: `monthRefKind`, proibicao de inferir ano indevidamente e mes por extenso.
- Grupo F, RF-22..RF-28: falha-segura de tool/LLM, truncamento por length, `MaxTokens`, metricas/logs para `MessageStore.Append` e `RunStore.Update`, agregacao de falhas e runs sem scorer/mensagem.
- Grupo G, RF-29..RF-34: manutencao dos 3 scorers atuais, novos 9 scorers comportamentais, sinais de promocao/rollback e observabilidade sem alta cardinalidade.
- Grupo H, RF-35..RF-38: golden set versionado, casos com tool/args/outcome/resposta esperados, dados sinteticos/anonimizados e metricas por versao.
- Grupo I, RF-39..RF-43: gate pre-deploy bloqueante, real-LLM manual/nightly/pre-deploy, thresholds, redefinicao de `tool-call-accuracy` e gate pos-deploy.
- Grupo J, RF-44: ausencia de identificadores `_`-prefixados em Go de producao no escopo afetado.
- Grupo K, RF-45..RF-46: workflows e pendencias com efeito unico, idempotencia, concorrencia, replay e conclusao sem `Suspended` orfao.
- Grupo L, RF-47..RF-48: estados fechados e economia de LLM quando guard deterministico resolve.
- Grupo M, RF-49..RF-57: baseline produtiva, melhora objetiva, amostra minima, decisao rastreavel, nenhum alerta critico novo e preservacao dos contratos/fluxos existentes.
</review_scope>

<architecture_checks>
- `internal/platform/workflow` deve permanecer generico, sem import de dominio, agente, memoria ou tipo semantico de consumidor.
- `internal/platform/agent` so pode receber evolucoes aditivas compatíveis com contratos publicos existentes.
- Tools e adapters devem ser finos: sem SQL direto, sem regra de negocio, sem branching de dominio e delegando para use cases/clients.
- Novos comportamentos agentivos devem entrar por registry, tool, agente, workflow ou scorer, nao por `switch case intent.Kind`.
- Estados de fronteira devem ser tipos fechados: `agent.ToolOutcome`, `agent.RunStatus`, `agent.AwaitingKind`, `workflow.RunStatus`, `workflow.StepStatus`, `workflow.SuspendReason`, `scorer.ScorerKind`, `memory.MessageRole` e outcome de truncamento.
- O fluxo canonico de inbound deve permanecer auditavel: `InboundRequest -> AgentRuntime.Execute -> ThreadGateway.GetOrCreate -> RunStore.Insert -> AgentRegistry.Resolve -> Agent.Execute -> MessageStore.Append -> closeRun`.
- Nenhuma metrica pode usar label de alta cardinalidade como `user_id`, `thread_id`, `resource_id`, `correlation_key` ou conteudo de mensagem.
- Codigo Go de producao nao pode conter comentario fora das excecoes permitidas por `AGENTS.md`.
- O gate R5.26 deve passar: nenhum identificador Go de producao pode comecar com `_`, exceto blank identifier.
</architecture_checks>

<validation_requirements>
Execute validacoes proporcionais ao escopo revisado e registre comandos, saida relevante e resultado:
- `go build` no escopo alterado ou `go build ./...` quando houver mudanca transversal;
- `go vet` no escopo alterado ou `go vet ./...` quando houver mudanca transversal;
- `go test -race -count=1` nos pacotes alterados e nos pacotes que cobrem agent/runtime/workflow/scorers/golden;
- `golangci-lint run` no escopo alterado quando disponivel;
- testes de guardas, runtime, tools de cartao, scorers comportamentais, workflows pendentes, idempotencia e golden deterministico;
- harness real-LLM apenas quando credenciais/ambiente permitirem, registrando comando, tag/env usada e resultado. Se nao for possivel executar, isso impede `APPROVED` salvo se houver evidencia formal equivalente exigida pelo PRD/techspec.

Inclua tambem verificacoes especificas:
- busca por `_`-prefixados em Go de producao no escopo RF-44;
- busca por comentarios proibidos em Go de producao no diff;
- busca por labels metricas proibidas;
- busca por alteracao indevida de schemas strict das tools;
- verificacao de que `tasks.md` realmente nao existe ou leitura dele se encontrado.
</validation_requirements>

<subagents>
Dispare subagentes especializados quando isso aumentar a qualidade ou reduzir risco de falso positivo:
- `review-guards-runtime`: revisar guard chain, multi-item, verbatim relay, empty answer, internal terms, success without tool, truncamento, `RunStore.Update` e `MessageStore.Append`.
- `review-financial-tools`: revisar tools de escrita, confirmacao, idempotencia, `cardId` provenance, `monthRefKind` e workflows pendentes.
- `review-scorers-golden`: revisar scorers atuais e comportamentais, fixtures golden, harness deterministico, real-LLM e redefinicao de `tool-call-accuracy`.
- `review-governance-tests`: revisar R5.26, comentarios proibidos, arquitetura `internal/platform`, testes, lint, build, vet e gates.

Cada subagente deve retornar somente achados com evidencia, arquivos lidos, comandos executados e riscos. A decisao final permanece no agente principal.
</subagents>

<bugfix_cycle>
Se qualquer item resultar em `REJECTED`, `APPROVED_WITH_REMARKS` ou `BLOCKED` por problema corrigivel:
1. Converta cada achado acionavel para o formato canonico da skill `bugfix`: `{ id, severity, file, line, reproduction, expected, actual }`.
2. Inclua `Origem` com RF, criterio, DoD, ADR ou secao da techspec.
3. Execute `.claude/skills/bugfix/` para corrigir causa raiz com teste de regressao obrigatorio.
4. Defina `AI_REVIEW_PRIOR_SHA` para revisar o delta da remediacao quando aplicavel.
5. Repita o ciclo `review -> bugfix -> review` ate obter `APPROVED`.

Nao encerre com `APPROVED` se houver qualquer bug `blocked`, `skipped`, `failed`, validacao ausente, risco residual acionavel ou ressalva.
</bugfix_cycle>

<output_format>
Produza um relatorio markdown em pt-BR com estas secoes obrigatorias:

1. **Veredito Final**
   - Um unico valor: `APPROVED`, `REJECTED` ou `BLOCKED`.
   - Declare explicitamente se o contrato de 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas foi cumprido.

2. **Arquivos e Referencias Lidos**
   - Liste PRD, techspec, ADRs, task file se existir, arquivos de codigo, testes, fixtures e referencias de governanca carregadas.

3. **Matriz de Rastreabilidade**
   - Tabela com colunas: `Item`, `Fonte`, `Status`, `Evidencia`, `Validacao`, `Observacao`.
   - Cubra RF-01..RF-57, criterio de sucesso primario, DoD, ADR-001..ADR-005 e regras de negocio.
   - Use status `atendido`, `nao atendido` ou `nao verificavel`; `nao verificavel` impede `APPROVED`.

4. **Achados**
   - Liste achados por severidade canonica da skill `review`: `critical`, `high`, `medium`, `low`.
   - Cada achado deve conter arquivo:linha, impacto, referencia ao PRD/techspec/ADR e dica de correcao.
   - Se nao houver achados, escreva exatamente: `Nenhum achado encontrado com base nas evidencias revisadas.`

5. **Bugs Canonicos para Bugfix**
   - Quando houver achados acionaveis, forneca a lista no formato canonico exigido por `.claude/skills/bugfix/`.
   - Quando nao houver, escreva: `Nenhum bug canonico gerado.`

6. **Validacoes Executadas**
   - Liste comandos, escopo, resultado e trecho relevante da saida.
   - Separe falhas preexistentes de falhas introduzidas pela implementacao.

7. **Subagentes**
   - Informe quais subagentes foram usados, arquivos analisados e conclusoes.
   - Se nenhum subagente foi usado, justifique tecnicamente.

8. **Riscos Residuais e Ressalvas**
   - Para `APPROVED`, esta secao deve conter exatamente: `Nenhuma ressalva, gap ou lacuna identificada.`
   - Qualquer outro conteudo nesta secao impede `APPROVED`.

9. **Proxima Acao**
   - Se `APPROVED`: declarar que a implementacao esta em conformidade total com o PRD.
   - Se `REJECTED`: declarar que o proximo passo obrigatorio e executar `bugfix` sobre os bugs canonicos e repetir a revisao.
   - Se `BLOCKED`: declarar a evidencia faltante e por que nao e possivel aprovar.
</output_format>

<approval_gate>
So emita `APPROVED` se todas as afirmacoes abaixo forem verdadeiras e sustentadas por evidencia:
- RF-01..RF-57 estao implementados.
- Todos os criterios de aceite e o criterio de sucesso primario composto estao atendidos.
- DoD esta 100% atendido.
- Todas as regras de negocio estao implementadas.
- Techspec e ADR-001..ADR-005 estao respeitados.
- Validacoes proporcionais foram executadas e passaram, ou a propria especificacao aceita formalmente evidencia equivalente.
- Nao existem achados, bugs canonicos, lacunas de teste, gaps de implementacao, ressalvas ou riscos residuais acionaveis.
- Nao ha falso positivo nos achados nem falso negativo por ausencia de verificacao.

Se qualquer afirmacao for falsa, desconhecida ou nao verificavel, o veredito deve ser `REJECTED` ou `BLOCKED`, nunca `APPROVED`.
</approval_gate>

---

## Justificativas das Adicoes

| Adicao | Justificativa |
|---|---|
| Tags XML (`<goal>`, `<context>`, `<rules>`, etc.) | Atende a governanca local para prompts multi-parte e reduz ambiguidade operacional. |
| RF-01..RF-57 explicitados por grupos | Evita revisao parcial e obriga rastreabilidade completa contra o PRD real. |
| Lista nominal de ADRs | Garante que decisoes arquiteturais vinculantes sejam tratadas como criterios de aceite. |
| Tratamento de `tasks.md` ausente | Evita alucinacao de artefato e exige busca/verificacao antes do veredito. |
| `APPROVED_WITH_REMARKS` como nao aceitavel | Alinha a skill `review` ao contrato do usuario de 0 ressalvas. |
| Validacoes obrigatorias | Transforma "implementado" em evidencia reproduzivel por build, vet, testes, lint e harness. |
| Subagentes por dominio tecnico | Aumenta profundidade da revisao sem misturar responsabilidades nem inflar conclusoes sem evidencia. |
| Ciclo canonico `review -> bugfix -> review` | Materializa a exigencia de corrigir causa raiz e repetir ate `APPROVED`. |
| Gate final de aprovacao | Impede aprovacao por suposicao, validacao ausente ou achado de baixa severidade ignorado. |

## Variante Curta para Disparo

<goal>
Revise a implementacao do PRD `.specs/prd-orquestracao-conversacional-confiavel` com `.claude/skills/review/` ate obter `APPROVED` real, sem gaps, lacunas, falsos positivos ou ressalvas.
</goal>

<rules>
- Leia `prd.md`, `techspec.md` e ADR-001..ADR-005 integralmente.
- Matriz obrigatoria para RF-01..RF-57, criterio de sucesso primario, DoD, regras de negocio, techspec e ADRs.
- `APPROVED_WITH_REMARKS`, `REJECTED` e `BLOCKED` nao atendem ao pedido; achados corrigiveis devem virar bugs canonicos e acionar `.claude/skills/bugfix/`.
- Repita `review -> bugfix -> review` ate `APPROVED`.
- So aprove com evidencia concreta de codigo, testes, fixtures, gates e validacoes passando.
</rules>

<output>
Relatorio markdown em pt-BR com veredito, arquivos lidos, matriz de rastreabilidade, achados, bugs canonicos, validacoes, subagentes, riscos residuais e proxima acao.
</output>
