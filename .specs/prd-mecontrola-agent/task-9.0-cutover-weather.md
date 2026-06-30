# Tarefa 9.0: Cutover — remoção total do weather sem resíduo + e2e + gates verdes

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Etapa final: remover 100% do weather-agent sem resíduo, trocando o registro pelo `MeControlaAgent`, e validar a entrega ponta a ponta. Preservar `internal/onboarding` (ativação de conta por magic token) intacto.

<requirements>
- ADR-001: substituição integral; remoção total sem resíduo/wiring órfão; `internal/onboarding` intacto.
- Cobre: RF-02, RF-03, RF-04.
</requirements>

## Subtarefas

- [ ] 9.1 Remover `internal/agents/application/agents/agent.go` (weather), tools/workflows/scorers/domain weather, `interfaces.WeatherClient`, `internal/agents/infrastructure/weather`.
- [ ] 9.2 Remover o campo `WeatherClient` de `agents.Deps` e do wiring (`cmd/server`); remover config weather.
- [ ] 9.3 Trocar o registro do `AgentRegistry` para o `MeControlaAgent` (já feito em 7.0/8.0; confirmar que weather não é mais registrado).
- [ ] 9.4 Ajustar/atualizar testes e2e que importavam o weather; manter `internal/onboarding` (ativação) intacto e funcional.
- [ ] 9.5 Gate de ausência de resíduo: `grep -rn "weather\|WeatherClient" internal/agents cmd/` (exceto histórico) retorna vazio.

## Detalhes de Implementação

Ver techspec.md → "Sequenciamento" passo 9 e ADR-001. Cutover é a última etapa; depende de 1.0–8.0 prontos. Rollback: reverter o commit do cutover restaura o weather sem afetar os módulos novos.

## Critérios de Sucesso

- Zero referência a weather/`WeatherClient` em produção (gate `grep` vazio).
- `internal/onboarding` (ativação) preservado e funcional (não tocado).
- Build/gofmt verdes; **todos os gates de governança verdes** (R-ADAPTER-001, R-AGENT-WF-001, R-WF-KERNEL-001, R-DTO-VALIDATE-001, R-TESTING-001).
- E2E: uma mensagem financeira percorre dispatcher→consumer→runtime→tool→gateway e responde; jornada de onboarding e um registro de despesa validados (variante real atrás de `RUN_REAL_LLM`).
- Cobertura de testes determinística no CI.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — substituição do consumidor de referência do substrato (weather → MeControlaAgent) sem quebrar o ciclo Thread→Run.

## Testes da Tarefa

- [ ] Testes unitários/integração: suíte verde após remoção; nenhum import órfão.
- [ ] Testes E2E: jornada de onboarding + registro de despesa; gate de ausência de resíduo.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/agent.go`, `internal/agents/infrastructure/weather/`, `internal/agents/domain/` (weather) — remoção
- `cmd/server/server.go`, `cmd/worker/worker.go` (limpeza de wiring/config weather)
- `internal/onboarding/` (preservar intacto)
- techspec.md (Sequenciamento passo 9), ADR-001
