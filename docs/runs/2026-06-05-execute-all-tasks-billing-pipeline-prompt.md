# Prompt enriquecido — `execute-all-tasks` para `.specs/prd-billing-pipeline`

## Objetivo

Preparar um prompt robusto, econômico e production-ready para executar a skill `execute-all-tasks` sobre `.specs/prd-billing-pipeline`, reduzindo desvios, falso positivo e execução fora de escopo.

## Prompt original

| Original | Enriquecido |
| --- | --- |
| `Eu quero usar de forma robusta, eficiente, economica, sem desvios e falso positivo completamente production-ready a skill execute-all-tasks para implementar tudo de acordo com: .specs/prd-billing-pipeline` | O prompt enriquecido abaixo fixa contexto, escopo, critérios de aceite, pontos de partida obrigatórios, regras de governança, contrato de saída e travas contra alucinação e falso positivo para uma execução completa via `execute-all-tasks`. |

## Ambiguidades tratadas

1. O prompt original não fixava quais arquivos são fonte de verdade; o enriquecido ancora `prd.md`, `techspec.md`, `tasks.md`, ADRs e task files da spec.
2. O prompt original não explicitava o contrato de execução; o enriquecido exige `execute-all-tasks` top-level, halt-first, DAG, paralelismo apenas quando permitido e evidência real por tarefa.
3. O prompt original não delimitava o que fazer diante de drift, lacunas ou itens fora de escopo; o enriquecido manda parar, reportar e não inventar implementação.
4. O prompt original não reforçava os entrypoints mandatórios do repositório; o enriquecido obriga partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, sem usar `internal/platform/runtime` como ponto de partida.

## Prompt enriquecido

```md
Execute a skill `execute-all-tasks` para implementar integralmente a spec `.specs/prd-billing-pipeline` de forma robusta, econômica, sem desvios, sem falso positivo e com governança production-ready.

### Objetivo operacional

Entregar a implementação completa do PRD `billing-pipeline` seguindo estritamente:

- `.specs/prd-billing-pipeline/prd.md`
- `.specs/prd-billing-pipeline/techspec.md`
- `.specs/prd-billing-pipeline/tasks.md`
- todos os arquivos `task-*.md` e ADRs presentes em `.specs/prd-billing-pipeline/`
- `AGENTS.md` como fonte canônica do repositório

### Regras mandatórias e inegociáveis

1. Use `execute-all-tasks` como orquestrador top-level para o PRD inteiro. Não degrade para execução manual inline de todas as tarefas.
2. Assuma sempre o working tree atual como fonte da verdade. Se houver divergência entre docs antigas, prompts históricos e código real, prevalece o estado atual do workspace e a opção mais segura.
3. Não invente packages, handlers, routers, jobs, consumers, adapters, migrations, interfaces, wiring ou contexto de negócio ausente no workspace. Se faltar contexto real, reporte drift ou bloqueio em vez de mascarar com placeholder.
4. O ponto de partida obrigatório para a implementação e para o wiring é `cmd/server/server.go` e/ou `cmd/worker/worker.go`. Não use `internal/platform/runtime` como ponto de partida.
5. Carregue obrigatoriamente `AGENTS.md` e a skill `agent-governance` antes de qualquer execução de tarefa.
6. Em toda tarefa que alterar código Go, carregue obrigatoriamente a skill `go-implementation` e use os exemplos e referências apenas sob demanda, respeitando a economia de contexto do repositório.
7. Em código Go, é obrigatório manter `0` comentários adicionados. Não introduza comentários em arquivos Go.
8. Verifique `go.mod` antes de introduzir APIs ou decisões dependentes de versão de linguagem. Respeite a versão declarada no repositório.
9. Preserve estritamente as fronteiras arquiteturais de `AGENTS.md`: fluxo `infrastructure -> application -> domain`, `domain` puro, interface no consumidor, DI manual explícita e sem abstrações especulativas.
10. Para `internal/billing` e `internal/identity`, siga o padrão obrigatório de módulo descrito em `AGENTS.md`. Não introduza `NewModule(opts...)`, `WithDatabase(...)`, `Routers()` ou `Runners()` como novo padrão.
11. Toda chamada HTTP outbound deve usar `internal/platform/httpclient`; todo side effect resiliente deve respeitar `internal/platform/outbox`; jobs e consumers devem passar pelos adapters de `internal/platform/worker`.
12. Não implemente nada fora do escopo do PRD/techspec/tasks. Itens explicitamente fora de escopo, como RF-19 e RF-21 quando marcados como não implementáveis nesta spec, devem ser apenas reconhecidos e não expandidos.
13. Não produza sucesso aparente sem evidência real. Se uma tarefa não puder ser concluída com evidência, status, report e atualização coerente em `tasks.md`, trate como `blocked`, `failed` ou `needs_input`, conforme o caso.
14. Não faça reexecução silenciosa, fallback mágico, reparo especulativo, “quase pronto”, nem marque tarefa como concluída sem cumprir integralmente o contrato da skill.

### Escopo de leitura obrigatório antes de executar

Leia e use como base:

- `AGENTS.md`
- `.agents/skills/execute-all-tasks/SKILL.md`
- `.agents/skills/agent-governance/SKILL.md`
- `.specs/prd-billing-pipeline/prd.md`
- `.specs/prd-billing-pipeline/techspec.md`
- `.specs/prd-billing-pipeline/tasks.md`
- todos os `task-*.md` do diretório da spec
- ADRs da própria spec quando citados por `tasks.md` ou `techspec.md`
- `go.mod`
- `cmd/server/server.go`
- `cmd/worker/worker.go`

### Requisitos de execução

1. Valide o pré-voo completo da skill `execute-all-tasks` sem pular hooks, depth checks, `ai-spec`, drift e cobertura de requisitos.
2. Respeite o DAG de `tasks.md`, o paralelismo permitido e a regra halt-first da skill.
3. Cada tarefa deve rodar em subagent fresh, com contexto mínimo necessário, sem contaminar a orquestração com contexto excessivo.
4. Cada subagent deve ficar estritamente no escopo do seu `task-*.md`.
5. Em tarefas Go, além de `agent-governance`, carregue `go-implementation` e selecione exemplos/referências apenas quando o diff exigir.
6. Não copie exemplos cegamente; adapte-os ao contexto real do repositório.
7. Registre explicitamente qualquer drift, bloqueio, dependência quebrada, lacuna de ambiente ou inconsistência entre spec e workspace.
8. Se houver conflito entre o texto da spec e o código real, prefira a opção mais segura, registre a decisão e não invente compensação estrutural.
9. Mantenha as alterações cirúrgicas, completas e conectadas ponta a ponta; não pare em implementação parcial disfarçada de progresso.

### Critérios de aceite inegociáveis

Considere a execução bem-sucedida apenas se:

1. Todas as tarefas aplicáveis de `.specs/prd-billing-pipeline/tasks.md` terminarem em `done`, ou a execução parar corretamente com `partial`, `failed` ou `needs_input` ao primeiro problema material, sem falso positivo.
2. Cada tarefa concluída tiver evidência física real no `report_path` esperado e coerência com o status final de `tasks.md`.
3. A implementação final respeitar `AGENTS.md`, o padrão modular de `identity` e `billing`, o fluxo arquitetural e as restrições de plataforma compartilhada.
4. Toda alteração Go tiver sido conduzida com `go-implementation`, exemplos sob demanda, sem comentários em código Go e sem atalhos que burlem as regras R0-R7.
5. Nenhum item fora de escopo for implementado por impulso.
6. O resultado final incluir o relatório de orquestração da skill e um sumário honesto do que foi concluído, do que bloqueou e dos próximos passos estritamente necessários.

### Formato de saída esperado ao concluir a execução

Responda em PT-BR, de forma objetiva, contendo:

1. `status_final`: `done`, `partial`, `failed` ou `needs_input`
2. `spec_alvo`: `.specs/prd-billing-pipeline`
3. `waves_executadas`
4. `tarefas_done`
5. `tarefas_nao_done`
6. `relatorio_orquestracao`
7. `bloqueios_reais`
8. `drifts_registrados`
9. `proximos_passos` apenas se houver bloqueio ou dependência externa real

Não inclua código na resposta final. Não omita falhas. Não resuma como concluído algo que ainda depende de correção manual, rerun ou decisão do usuário.
```

## Justificativa das adições

| Adição | Justificativa curta |
| --- | --- |
| Fonte de verdade explícita | Evita execução baseada em contexto velho ou doc desatualizada. |
| Entry points obrigatórios | Alinha o prompt com o padrão operacional já exigido neste repositório. |
| Carga mandatória de `go-implementation` | Garante que toda alteração Go siga o fluxo obrigatório do repositório. |
| `0` comentários em Go | Elimina desvio de estilo já tratado como regra inegociável pelo usuário. |
| Contrato de saída e critérios de aceite | Reduz falso positivo e “done” sem evidência real. |
| Bloqueio explícito de escopo e alucinação | Evita implementação inventada, placeholders e expansão indevida do PRD. |

## Variante curta

Se quiser uma versão mais enxuta para colar diretamente sem o contexto explicativo do runbook:

```md
Execute `execute-all-tasks` para implementar integralmente `.specs/prd-billing-pipeline` com governança production-ready. Use `AGENTS.md` como fonte canônica, working tree atual como fonte da verdade e parta obrigatoriamente de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nunca de `internal/platform/runtime`. Carregue `agent-governance` em toda tarefa e `go-implementation` em toda alteração Go, usando exemplos apenas sob demanda. Em Go, adicione zero comentários. Não invente wiring, handlers, jobs, consumers, adapters ou migrations ausentes; reporte drift ou bloqueio em vez disso. Respeite integralmente `prd.md`, `techspec.md`, `tasks.md`, `task-*.md`, ADRs da spec, DAG, paralelismo permitido e halt-first. Não implemente itens fora de escopo. Não marque sucesso sem evidência real por tarefa, `report_path` válido e coerência com `tasks.md`. Ao final, responda em PT-BR com `status_final`, waves executadas, tarefas `done` e não `done`, relatório de orquestração, bloqueios reais, drifts registrados e próximos passos apenas se estritamente necessários.
```
