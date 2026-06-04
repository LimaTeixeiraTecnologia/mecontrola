# ADR-007 — Layout físico de `internal/identity/` com sub-pastas por responsabilidade

## Metadados

- **Título:** Adoção do "Layout Obrigatório por Módulo" do AGENTS.md como padrão físico em identity
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + autor do PRD
- **Relacionados:** AGENTS.md §"Layout Obrigatório por Módulo", PRD (visão geral), techspec §Visão Geral dos Componentes

## Contexto

`AGENTS.md` prescreve para novos módulos de negócio o layout:

```
internal/<modulo>/
  application/{dtos/{input,output}, usecases, interfaces}
  domain/{entities, valueobjects, services, interfaces}
  infrastructure/{messaging/<broker>/{producers,consumers}, repositories/{postgres,mssql}, http/{server,client}}
```

`internal/identity/README.md` atual mostra exemplos de scaffold em layout plano (`domain/user.go`, `application/repository.go`). Há divergência: o README é da época do scaffold inicial; AGENTS.md (governança canônica) é mais recente e prevalece (R-GOV-001 + precedência declarada em `governance.md`).

`internal/platform/outbox/` usa layout plano, mas `platform/` é fundação técnica compartilhada (não módulo de negócio) — a regra do AGENTS.md cobre apenas módulos de negócio.

Identity é o primeiro módulo de negócio real a sair do estágio scaffold-apenas (`finance` e `conversation` ainda só têm `doc.go`). A decisão de layout aqui vira referência para billing (E2), onboarding (E3) e refatoração futura de finance/conversation.

## Decisão

Adotar o layout obrigatório do AGENTS.md em `internal/identity/`:

```
internal/identity/
  application/
    interfaces/         # ports (UserRepository, IDGenerator)
    usecases/           # 1 arquivo por use case
  domain/
    entities/           # User aggregate + UserID
    valueobjects/       # WhatsAppNumber, Email, UserStatus
    services/           # EntitlementChecker, Subscription contract
    errors.go           # sentinelas compartilhadas do domínio
  infrastructure/
    repositories/postgres/   # PgxUserRepository
    id/                      # UUID v4 generator
```

Sub-pastas vazias documentadas em AGENTS.md (ex.: `domain/services` quando só há entidades) **não** são criadas — só se materializam quando demandadas. `application/dtos/{input,output}` não é criado neste PRD porque os use cases recebem tipos primitivos (`rawNumber string`) ou VOs diretamente — DTOs entram quando houver handler HTTP/gRPC (fora de escopo).

O README atual é reescrito para alinhar exemplo ao layout final (parte da F-07).

## Alternativas Consideradas

- **Layout plano (`domain/user.go`, `domain/whatsapp_number.go`)** — Vantagens: menos navegação, segue exemplo plano do README atual e do `platform/outbox`. Desvantagens: viola AGENTS.md "DEVEM usar separacao fisica clara por responsabilidade"; divergente entre módulos de negócio. Rejeitada.
- **Layout híbrido (sub-pasta só com ≥ 2 arquivos)** — Vantagens: pragmático. Desvantagens: inconsistente entre módulos e ao longo do tempo (arquivo cresce, demanda mudança de path), confunde import paths em refactor. Rejeitada.

## Consequências

### Benefícios Esperados

- AGENTS.md cumprido à risca; identity vira modelo de referência para billing/onboarding.
- Imports refletem responsabilidade (`identity/domain/entities`, `identity/domain/valueobjects`) facilitando navegação.
- `depguard` pode evoluir para regras mais finas se necessário (`domain/entities` não importa `domain/services`?).

### Trade-offs e Custos

- Paths de import mais longos.
- Mais diretórios para criar inicialmente (cosmético).

### Riscos e Mitigações

- **Risco:** Excesso de fragmentação em módulo que ainda é pequeno.
- **Mitigação:** Só criar sub-pasta quando há arquivo real (regra: nenhum diretório vazio + nenhum `doc.go` sem conteúdo útil).
- **Risco:** Refatoração futura para flatten exige tocar muitos imports.
- **Mitigação:** Layout estável é objetivo do PRD; mudança implicaria PRD próprio de refactor com migração assistida.

## Plano de Implementação

1. Criar diretórios apenas conforme arquivos forem materializados na ordem da §Sequenciamento de Desenvolvimento.
2. Reescrever `internal/identity/README.md` com exemplos no layout final.
3. Validar com `depguard` que `domain/*` não importa `application/*` nem `infrastructure/*` (regras já existentes cobrem isso por glob).

## Monitoramento e Validação

- `golangci-lint run` valida fronteiras.
- Code review valida que nenhum arquivo Go aparece direto em `internal/identity/{domain,application,infrastructure}/` (sempre dentro de subpasta de responsabilidade), exceto `doc.go` e `errors.go` (sentinelas transversais).

## Impacto em Documentação e Operação

- `internal/identity/README.md` reescrito.
- `internal/identity/AGENTS.md` reescrito.
- Futuro PRD de billing/onboarding herda o padrão.

## Revisão Futura

Reavaliar se algum módulo de negócio crescer a ponto de tornar `application/usecases/` opressivo (>30 arquivos), ponto em que sub-divisão por agregado pode ser necessária.
