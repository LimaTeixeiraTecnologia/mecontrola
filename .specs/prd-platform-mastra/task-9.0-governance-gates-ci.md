# Tarefa 9.0: Reemissão dos gates de governança no CI/Taskfile

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reemitir, no Taskfile/CI, os gates de verificação (grep) das regras `R-WF-KERNEL-001` e `R-AGENT-WF-001` (alteradas 2026-06-29) apontando para os caminhos finais de `internal/platform/{agent,memory,llm,scorer,tool}` e o kernel. Endurecimento anti-desvio: o pipeline falha se layering, LLM no kernel, comentários em Go de produção, alta cardinalidade ou tipos abertos forem introduzidos.

<requirements>
- RF-31: enforcement de fronteira — `internal/platform` sem regra/semântica de domínio.
- RF-32: enforcement de consumibilidade/layering unidirecional.
- RF-33: enforcement de tipos fechados na fronteira (nenhuma `string` solta).
- RF-34: enforcement de kernel sem LLM; LLM só na camada agent.
- Gates determinísticos (grep), executáveis localmente e no CI; retornam vazio quando conformes.
</requirements>

## Subtarefas

- [ ] 9.1 Adicionar task `gates:platform` ao Taskfile com os greps da techspec "Conformidade com Padrões > Gates de verificação reemitidos".
- [ ] 9.2 Gate de import do kernel (proíbe `internal/platform/{agent,memory,llm,scorer,tool}` e domínio em `internal/platform/workflow`).
- [ ] 9.3 Gate de LLM no kernel; gate de zero comentários nas 5 camadas novas; gate de cardinalidade (sem `resource_id`/`thread_id`/`correlation_key`).
- [ ] 9.4 Integrar `gates:platform` ao pipeline de CI como passo bloqueante (gate de merge).

## Detalhes de Implementação

Ver techspec.md "Conformidade com Padrões" (bloco de gates), `.claude/rules/workflow-kernel.md` e `.claude/rules/agent-workflows-tools.md` (gates reemitidos para os caminhos finais). Os greps devem usar `--exclude-dir=mocks --exclude="*_test.go"` conforme as regras.

## Critérios de Sucesso

- `task gates:platform` executa todos os greps e falha (exit≠0) ao detectar violação; retorna 0 quando conforme.
- CI roda `gates:platform` como passo bloqueante no gate de merge.
- Gates apontam para os caminhos finais reais (não para `internal/agent`, removido).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `taskfile-production` — configuração robusta de Taskfile e integração de gates em CI/CD é o gatilho direto desta tarefa.

## Testes da Tarefa

- [ ] Teste de gate: introduzir violação temporária (ex.: import proibido em fixture) e confirmar que `gates:platform` falha; remover e confirmar que passa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `Taskfile.yml` (ou equivalente) — task `gates:platform`.
- Configuração de CI do projeto — passo bloqueante.
- `.claude/rules/workflow-kernel.md`, `.claude/rules/agent-workflows-tools.md` — fonte dos gates.
