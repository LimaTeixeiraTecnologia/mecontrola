# Tarefa 2.0: `internal/agents` domínio + tool `get-weather` + cliente open-meteo

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar a base de domínio de `internal/agents` (value objects, tipos fechados DMMF, mapeamento `weather_code`→condição), a tool `get-weather` tipada (paridade Mastra) e o cliente open-meteo (geocoding + forecast) como IO externo do consumidor.

<requirements>
- RF-09: tool `get-weather` com input (`location`) e output (`temperature, feelsLike, humidity, windSpeed, windGust, conditions, location`) tipados/validados; consumível por agent e steps.
- RF-10: dados de open-meteo (geocoding→forecast), mapeamento `weather_code`, erro explícito (cidade não encontrada / upstream).
- RF-04: tipos fechados/state-as-type, smart constructors; sem `Result[T,E]`/currying/DSL/monads.
- ADR-001 (consumidor sobre plataforma; proibido importar `internal/agent`).
</requirements>

## Subtarefas

- [ ] 2.1 `internal/agents/domain`: `Forecast` (value object + smart constructor), `WeatherConditions`, mapeamento `weatherCode` como tipo fechado.
- [ ] 2.2 `internal/agents/infrastructure/weather`: `WeatherClient` (Geocode + Forecast) via `httpclient` com timeout e erros tipados (`ErrLocationNotFound`).
- [ ] 2.3 `internal/agents/tool.go`: `buildWeatherTool` via `tool.NewTool[WeatherInput, WeatherOutput]` com schemas I/O.
- [ ] 2.4 DTOs de input com `Validate()` quando houver fronteira de entrada.

## Detalhes de Implementação

Ver techspec.md §"Interfaces Chave" (WeatherInput/Output, WeatherClient) e §"Pontos de Integração" (open-meteo URLs). Reaproveitar `test/conformance/weather/weather_tool.go` como base, promovendo a produção.

## Critérios de Sucesso

- Tool retorna output válido para cidade existente; erro explícito para inexistente/upstream.
- Mapeamento de `weather_code` coberto; smart constructors rejeitam inválidos.
- Zero comentários em produção; gofmt limpo; sem import de `internal/agent`.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `mastra` — a Tool segue o contrato `@mastra/core/tools` mapeado ao `internal/platform/tool`.

## Testes da Tarefa

- [ ] Testes unitários (tool via httptest: sucesso, not-found, 5xx, mapeamento de código; smart constructors)
- [ ] Testes de integração (não obrigatório nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/domain/*`, `internal/agents/infrastructure/weather/*`, `internal/agents/tool.go`; base: `test/conformance/weather/weather_tool.go`.
