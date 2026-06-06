# Prompt canônico — `create-technical-specification`

Use a skill `create-technical-specification` para produzir uma especificação técnica mandatória, robusta, production-ready, eficiente e econômica, sem alucinação arquitetural e sem desviar do PRD aprovado.

## Gate obrigatório de entrada

Antes de explorar o repositório, valide se eu informei o diretório da spec.

- Se eu **não** informar o path, responda obrigatoriamente com `needs_input` pedindo exatamente o diretório `.specs/prd-<slug>/`.
- Se o path informado não contiver `prd.md`, responda `needs_input` pedindo um PRD válido no mesmo diretório antes de continuar.

## Escopo e fonte de verdade

Use como base obrigatória:

- `AGENTS.md`
- `.agents/skills/create-technical-specification/SKILL.md`
- `.agents/skills/agent-governance/SKILL.md`
- `.specs/prd-<slug>/prd.md`
- o working tree atual como fonte da verdade

Se docs históricas divergirem do código atual, prevalece o estado real do workspace e a opção mais segura.

## Regras mandatórias

1. Explore apenas os módulos, integrações, entrypoints, interfaces e caminhos de código realmente relevantes ao PRD.
2. Não invente packages, handlers, routers, repositories, adapters, migrations, jobs, consumers, contratos, fluxos ou providers inexistentes.
3. Registre drift ou bloqueio quando o PRD pedir algo incompatível com o workspace atual.
4. Faça no máximo duas rodadas de esclarecimento técnico.
5. Se decisões materiais permanecerem ambíguas após duas rodadas, retorne `needs_input` com as decisões faltantes.
6. Foque no `como implementar`, não em reexplicar o PRD.
7. Toda decisão material deve trazer trade-off, alternativa rejeitada, risco e estratégia de teste.
8. Toda techspec desta base deve declarar explicitamente que qualquer implementação Go derivada:
   - deve carregar obrigatoriamente `go-implementation`
   - deve carregar exemplos apenas sob demanda
   - deve verificar `go.mod` antes de usar recursos da linguagem
   - deve partir de `cmd/server/server.go` e/ou `cmd/worker/worker.go`, nunca de `internal/platform/runtime`
9. Não implemente código durante a elaboração da techspec.

## Regras específicas do repositório a refletir na spec

1. Preserve as fronteiras `infrastructure -> application -> domain`.
2. `domain` permanece puro.
3. Para `internal/identity` e `internal/billing`, respeite o padrão obrigatório de módulo com DI manual explícita.
4. Toda chamada HTTP outbound deve passar por `internal/platform/httpclient`.
5. Outbox, workers, jobs e consumers devem respeitar os contratos já definidos em `AGENTS.md`.
6. Em futuras implementações Go, comentários em arquivos Go são proibidos.

## Artefatos obrigatórios

1. Ler o template oficial da techspec antes de redigir.
2. Gerar `.specs/prd-<slug>/techspec.md`.
3. Calcular e injetar o `spec-hash` do PRD no topo da techspec.
4. Criar ADR separada para cada decisão material introduzida, no mesmo diretório da spec.
5. Vincular as ADRs a partir da techspec.

## Critérios de aceite inegociáveis

Considere o trabalho concluído apenas se:

1. `prd.md` tiver sido consumido como fonte primária
2. a techspec estiver salva em `.specs/prd-<slug>/techspec.md`
3. toda decisão material estiver rastreada, com trade-offs e testes
4. o `spec-hash` do PRD estiver presente
5. as ADRs materiais estiverem criadas e linkadas
6. a spec não contiver implementação inventada nem abstrações sem lastro no workspace

## Formato de saída esperado

Responda em PT-BR e retorne apenas:

1. `status_final`: `done` ou `needs_input`
2. `spec_alvo`
3. `techspec_path`
4. `adrs_criadas`
5. `decisoes_materiais`
6. `riscos_abertos`
7. `drifts_registrados`
8. `proximos_passos` apenas se houver bloqueio real
