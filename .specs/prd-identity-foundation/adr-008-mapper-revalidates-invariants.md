# ADR-008 — Mapper Postgres re-valida invariantes na reidratação do agregado `User`

## Metadados

- **Título:** Defesa em profundidade: `rowMapper.HydrateUser` chama construtores de VOs ao reconstruir `*entities.User`
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + autor do PRD
- **Relacionados:** PRD (RT-08), techspec §Modelos de Dados §Mapper, ADR-006 (constraints DB), R-ERR-001

## Contexto

Quando uma row Postgres é convertida em `*entities.User`, há duas filosofias possíveis:

1. **Trust DB** — confiar que CK constraint (`status IN ('ACTIVE','BLOCKED','DELETED')`), UNIQUE parcial (`whatsapp_number`), formato E.164 no INSERT (validado pelo VO) e tipo `UUID` da coluna garantem integridade. Mapper popula `entities.User` diretamente via construtor interno sem revalidar. Mais rápido (~nanossegundos a menos).
2. **Defense in depth** — mapper chama os mesmos construtores que validariam input externo (`NewUserID`, `NewWhatsAppNumber`, `NewEmail`). Custo: ~microssegundos por row. Benefício: row corrompida (migração manual via psql, restore parcial de backup, ferramenta externa que ignora constraint) falha imediato em vez de propagar dado inválido para domínio.

O PRD não decide explicitamente. A decisão foi confirmada via questionário: defesa em profundidade.

## Decisão

`rowMapper.HydrateUser` (em `internal/identity/infrastructure/repositories/postgres/mapper.go`) reconstrói `*entities.User` chamando `entities.NewUserID(row.id)`, `valueobjects.NewWhatsAppNumber(row.whatsapp_number)` e `valueobjects.NewEmail(row.email)` (se não nulo). Falha em qualquer construtor é wrappada com prefixo `"postgres user mapper: <campo> corrompido: %w"` e propagada como erro de leitura.

Para `status` e `deleted_at`, que já foram filtrados em SELECT (`WHERE deleted_at IS NULL`) ou cobertos por CK constraint, o mapper aceita os valores sem revalidar — esses campos são reconstruídos via construtor exclusivo `entities.RehydrateUser(RehydrateUserParams)`, distinto de `entities.NewUser` (não dispara `ErrUserRequiresTimestamps`). A função `entities.RehydrateUser` é documentada como "uso restrito ao mapper de infrastructure" no godoc.

## Alternativas Consideradas

- **Trust DB completo** (construtor interno `entities.hydrateFromRow` sem validação) — Vantagens: mais rápido. Desvantagens: dado corrompido (ex.: alguém renomeou o número via SQL sem normalizar) flui silenciosamente para serviços downstream. Rejeitada — custo de validação é desprezível frente a I/O de banco; benefício de fail-fast é real.
- **Revalidação parcial (só PK)** — Vantagens: compromisso. Desvantagens: cria assimetria que confunde leitor ("por que UUID sim e WhatsApp não?"). Rejeitada.
- **Validação em camada superior (use case re-valida após mapeamento)** — Vantagens: separa concerns. Desvantagens: duplica trabalho em todo consumidor; mapper continua produzindo agregado inválido. Rejeitada.

## Consequências

### Benefícios Esperados

- Corrupção de dado vira erro de leitura visível em log (`postgres user mapper: whatsapp corrompido: ...`).
- Construtores de VOs são a única porta de validação — mantém invariantes consistentes entre INSERT e SELECT.
- Testes de integração detectam drift se constraint do banco for relaxada sem atualizar VO.

### Trade-offs e Custos

- Custo de CPU adicional (~microssegundos por user lido) — irrelevante para volumes do MVP (estimativa: < 5k subs).
- Pequena duplicação aparente (VO valida no INSERT e novamente no SELECT) — justificada como defesa em camadas.
- Necessidade de `RehydrateUser` como construtor adicional, com risco de uso indevido em `application`.

### Riscos e Mitigações

- **Risco:** `entities.RehydrateUser` é usada em `application/usecases` por engano, contornando invariantes de `NewUser`.
- **Mitigação:** Godoc explícito `// Uso restrito ao mapper de infrastructure`; PR template inclui checklist; futuro lint custom poderia bloquear o uso fora de `internal/identity/infrastructure/`.
- **Risco:** Validação no SELECT mascara constraint quebrada do banco (dev assume que constraint existe porque mapper não falha).
- **Mitigação:** Integration test valida que constraints existem em `pg_constraint` após `RunMigrations`.

## Plano de Implementação

1. Implementar `entities.RehydrateUser(RehydrateUserParams) *User` em `internal/identity/domain/entities/user.go`.
2. Implementar `rowMapper.HydrateUser(userRow) (*entities.User, error)` em `infrastructure/repositories/postgres/mapper.go` chamando os 3 construtores de VOs.
3. Testes de integração `TestMapperRejeitaUUIDCorrompido` e `TestMapperRejeitaWhatsAppCorrompido` inserem row via SQL direto (bypass de VO) e validam que SELECT falha com erro tipado.

## Monitoramento e Validação

- Log com chave `postgres.user_mapper.error` em qualquer falha de hidratação.
- Métrica futura: counter `identity_mapper_corruption_total` por campo — pico indica drift de schema ou intervenção manual.

## Impacto em Documentação e Operação

- `internal/identity/AGENTS.md` documenta a regra "infrastructure usa `RehydrateUser`; application e domain nunca".

## Revisão Futura

Revisitar se profiling mostrar `mapper.HydrateUser` em hot path crítico de leitura (>10k user reads/s) — improvável no MVP.
