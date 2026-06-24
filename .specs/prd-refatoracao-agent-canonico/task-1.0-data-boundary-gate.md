# Tarefa 1.0: Gate de fronteira de dados + gates de governança

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Blindar a fronteira de dados do `internal/agent` (acesso só a tabelas próprias; consumo de outro BC só
por porta de entrada) com um gate de CI que falha o build em violação, e ancorar os gates de governança
transversais (R-*, zero-comentário, cardinalidade controlada de métricas). É a primeira tarefa: o gate
deve estar verde no estado atual antes de qualquer refatoração.

<requirements>
- RF-20: agent acessa apenas tabelas próprias; proibido SQL direto a outro BC e import de repo/infra de outro contexto; gate de CI falha o build em violação.
- RF-43: toda alteração Go passa nos gates R-ADAPTER-001, R-AGENT-WF-001, R-WF-KERNEL-001, R-TESTING-001, R-DTO-VALIDATE-001, R-TXN-WORKFLOWS-001 + checklist R0–R7.
- RF-44: zero comentários em Go de produção (R-ADAPTER-001.1); sem init() (R0); sem panic em produção (R5.12); context.Context em IO (R6); errors.Join/%w (R7).
- RF-45: métricas novas com cardinalidade controlada (labels só de enums fechados; proibido user_id/category_id/correlation_key/message_id).
</requirements>

## Subtarefas

- [ ] 1.1 Criar `scripts/ci/agent-data-boundary.sh` conforme techspec §"Gate de fronteira de dados" (SQL direto + import de repo/infra de outro BC em `internal/agent`).
- [ ] 1.2 Adicionar receita no Taskfile (`task ci:agent-boundary`) e passo no pipeline de CI.
- [ ] 1.3 Garantir que os gates existentes (`go-adapters.md`, `agent-workflows-tools.md`, `workflow-kernel.md`, `transactions-workflows.md`, `input-dto-validate.md`) estejam wired no CI; consolidar greps de R0/R5/R7 e cardinalidade no pipeline.
- [ ] 1.4 Rodar o gate no estado atual e comprovar verde (a fronteira já é satisfeita — ADR-001).
- [ ] 1.5 PR de teste negativo (injeção temporária de SQL direto no agent) deve falhar o gate; reverter.

## Detalhes de Implementação

Ver techspec.md §"Gate de fronteira de dados (ADR-001)" e `.specs/prd-refatoracao-agent-canonico/adr-001-agent-data-boundary-gate.md`. Não duplicar conteúdo.

## Critérios de Sucesso

- Gate verde no estado atual; vermelho no PR de teste negativo.
- CI executa todos os gates R-* + zero-comentário + cardinalidade.
- Runbook do agent atualizado com o gate.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — o gate codifica a fronteira `Workflow→Tool→binding→usecase` e a regra de dados exclusiva do `internal/agent` (R-AGENT-WF-001).

## Testes da Tarefa

- [ ] Testes unitários (script: caso positivo e negativo do gate, se houver harness shell).
- [ ] Testes de integração (execução do gate no CI local via Taskfile).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `scripts/ci/agent-data-boundary.sh` (novo)
- `Taskfile.yml`
- pipeline de CI (`.github/workflows/*` ou equivalente)
- `docs/runbooks/agent-*.md`
