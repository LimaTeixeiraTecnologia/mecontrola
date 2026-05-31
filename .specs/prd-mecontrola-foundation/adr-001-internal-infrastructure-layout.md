# ADR-001 — Layout `internal/infrastructure/` substitui `internal/platform/`

## Metadados

- **Título:** Layout `internal/infrastructure/` substitui `internal/platform/`
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94 (tech lead)
- **Relacionados:** [PRD v7 §RF-09, §RF-10, §D-08](./prd.md), [techspec §Arquitetura](./techspec.md), [ADR-010 (cobra subcomandos em `cmd/`)](./adr-010-cobra-subcommands.md)

> **Atualização v7:** com a adoção de cobra (ADR-010), `cmd/` deixou de ser pasta única (`cmd/server/main.go`) e passou a conter `cmd/main.go` + `cmd/{server,worker,migrate}/cmd.go`. O princípio do `internal/infrastructure/` permanece intacto; a mudança afeta só o entrypoint.

## Contexto

O discovery `technical-arquitetura-backend-mvp-mecontrola` propunha `internal/platform/{config,observability,db,http,ratelimit,events,clock}` como pacote cross-cutting. O PRD v1–v4 adotou essa nomenclatura. Durante a fase de techspec o tech lead apontou que `platform` é um nome ambíguo no contexto Go (confunde com camada de PaaS / Fly.io) e que `infrastructure` é semanticamente mais preciso (representa "tudo que é IO concreto / adapters cross-cutting", em oposição ao `domain` puro).

A correção precisa nascer **no primeiro commit**: alterar depois do código pronto força refactor massivo e poluiria git history.

## Decisão

Todo cross-cutting de infraestrutura vive em **`internal/infrastructure/`** com sub-pacotes mínimos: `config`, `observability`, `database`, `http`, `events`, `clock`, `errors`, `runtime`. O nome histórico `platform` está descartado para sempre neste repositório.

Os 7 módulos do discovery viram **6 módulos de domínio** (`identity`, `conversation`, `agent`, `finance`, `notifications`, `telemetry`) — `platform` deixa de ser um "módulo de domínio" e se torna o pacote de infraestrutura compartilhada.

## Alternativas Consideradas

1. **Manter `internal/platform/`** (proposta original do discovery).
   - Vantagens: zero divergência com o documento de discovery; menos retrabalho de PRD.
   - Desvantagens: nome ambíguo; "platform" em Go raramente significa "infra"; risco de virar lixeira semântica.
2. **Pasta `pkg/` para o devkit-go wrapping** (estilo libs externas).
   - Vantagens: alinha com convenção de pacotes públicos reutilizáveis.
   - Desvantagens: viola Go style guide (não usar `pkg/` quando não há export externo); incentiva acoplamento indevido entre projetos.
3. **`internal/shared/`**.
   - Vantagens: nome curto.
   - Desvantagens: "shared" não comunica que é infra; histórico de "shared" em codebases costuma virar god-package.

## Consequências

### Benefícios Esperados

- Nomenclatura precisa: `internal/infrastructure/` declara intenção arquitetural sem ambiguidade.
- Reduz risco de PR virar "atualiza platform" sem dono claro.
- Facilita onboarding (dev novo entende fronteiras pelo nome do diretório).
- Aderente a `shared-architecture.md` §Diretrizes ("nomear pelo papel de infraestrutura real").

### Trade-offs e Custos

- Divergência permanente vs documento de discovery (mitigada por nota explícita no PRD §RF-09 e nesta ADR).
- 1 página adicional de documentação para explicar a divergência.

### Riscos e Mitigações

- **Risco:** `internal/infrastructure/` virar lixeira de utilitários genéricos sem dono.
  - **Mitigação:** Regra `depguard` em `.golangci.yml` restringindo imports + skill `object-calisthenics-go` em review automático + critério de aceite de PR: cada novo sub-pacote em `internal/infrastructure/` precisa de doc.go explicando responsabilidade.

## Plano de Implementação

1. Atualizar PRD para v5 (já feito) substituindo todas as ocorrências de `internal/platform/` por `internal/infrastructure/`.
2. Criar a estrutura `internal/infrastructure/{config,observability,database,http,events,clock,errors,runtime}/doc.go` no primeiro commit de código.
3. Configurar `depguard` para impedir que `internal/<modulo>/domain/*` importe `internal/infrastructure/*`.
4. Adicionar verificação no `task lint` que confirma ausência de `internal/platform/`.

## Monitoramento e Validação

- `task lint` falha se aparecer `internal/platform/`.
- `golangci-lint depguard` falha em violação de fronteira.
- Review code de PR confere `doc.go` em qualquer novo sub-pacote sob `internal/infrastructure/`.

## Impacto em Documentação e Operação

- PRD v5 atualizado.
- README.md (a criar) deve mostrar layout final.
- Runbooks: nenhum impacto direto.
- Onboarding: incluir esta ADR como leitura obrigatória.

## Revisão Futura

- Revisitar se o número de sub-pacotes ultrapassar 12 (sinal de erosão).
- Marco: 6 meses após primeiro deploy de produção (≈ 2026-12).
