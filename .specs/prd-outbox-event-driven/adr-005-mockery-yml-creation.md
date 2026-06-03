# ADR-005 — Criação de `mockery.yml` na raiz como parte desta entrega

## Metadados

- **Título:** Adoção formal de `mockery.yml` versionado, declarando interfaces do Outbox como primeiros consumidores
- **Data:** 2026-06-02
- **Status:** Aceita
- **Decisores:** Tech lead backend
- **Relacionados:** PRD `prd-outbox-event-driven` v4; techspec; R3 (`go-implementation/references/testing.md`)

## Contexto

Auditoria do codebase revelou conflito de governança:

- `go.mod` pinou `github.com/vektra/mockery/v2 v2.53.6` (entrada direta).
- **Não existe `mockery.yml` na raiz do repositório** (verificado: `cat mockery.yml` → no such file).
- Nenhum pacote atual usa `mockery` para gerar mocks; pacotes existentes (`internal/infrastructure/{events,database,errors,http,observability}`) usam apenas testify/suite com fakes manuais quando necessário.
- R3 da `go-implementation/references/testing.md` (severidade `[HARD]`): "Todo mock de interface usado em testes DEVE ser gerado via mockery com configuração declarada em `mockery.yml` na raiz do módulo".

O Outbox introduz a primeira interface não-trivial que justifica mocks gerados: `outbox.Storage` tem 9 métodos com tipos complexos, mockear à mão geraria ~200 LOC de boilerplate frágil que precisaria ser mantido sincronizado manualmente. A regra R3 obriga `mockery.yml`.

## Decisão

Esta entrega **cria o arquivo `mockery.yml` na raiz** com a seguinte configuração inicial:

```yaml
# mockery.yml — raiz do módulo
with-expecter: true
mockname: "{{.InterfaceName}}"
outpkg: "mocks"
filename: "{{.InterfaceName | snakecase}}.go"
dir: "{{.InterfaceDir}}/mocks"
packages:
  github.com/LimaTeixeiraTecnologia/mecontrola/internal/infrastructure/outbox:
    interfaces:
      Storage:
      Registry:
```

E **adiciona ao `Taskfile.yml`**:

```yaml
tasks:
  mocks:
    desc: Gera mocks das interfaces declaradas em mockery.yml
    cmds:
      - go run github.com/vektra/mockery/v2 --config mockery.yml

  mocks:verify:
    desc: Verifica se os mocks estão sincronizados (gate de CI)
    cmds:
      - go run github.com/vektra/mockery/v2 --config mockery.yml
      - git diff --exit-code -- internal
```

Mocks gerados ficam em `internal/infrastructure/outbox/mocks/` e são commitados (padrão mockery + go).

PRDs futuros que introduzam interfaces (ex.: `notifications.EmailSender`, `finance.PaymentGateway`) **adicionam suas interfaces ao `packages:` deste arquivo** — não criam YAML próprio nem rodam mockery ad-hoc. Padrão consolidado de governança.

`Handler` (tipo função `func(ctx, evt) error`) **não vai para mockery** — fakes manuais em `internal/infrastructure/outbox/fakes/handler.go` cobrem cenários de teste.

## Alternativas Consideradas

- **Não criar `mockery.yml`; mocks ad-hoc por CLI**: viola R3 explicitamente; deixa o próximo PRD repetir a discussão; nenhum guardrail de CI possível.
- **Criar `mockery.yml` mas vazio (placeholder)**: gera arquivo sem propósito; viola "evitar arquivos sem demanda concreta" da governança.
- **Escrever mocks à mão para o Outbox e adiar `mockery.yml`**: 200+ LOC de boilerplate; teste do Dispatcher é crítico (RF-33) e merece mock confiável. R3 violada.
- **Substituir mockery por `go.uber.org/mock` (gomock)**: troca de ferramenta; sem ganho; quebra R3 que cita mockery por nome.

## Consequências

**Benefícios**:
- Cumpre R3 `[HARD]` da governança.
- Estabelece padrão reutilizável para PRDs futuros.
- Gate de CI (`task mocks:verify`) detecta drift entre interface e mock.
- `with-expecter: true` habilita assertions tipadas (`s.storage.EXPECT().ClaimReady(...).Return(...)`).

**Custos**:
- 1 arquivo `mockery.yml` na raiz + 2 tarefas no Taskfile.
- Pequena curva de aprendizado para devs não familiarizados com mockery.

**Riscos / Mitigações**:
- **Dev esquece de rodar `task mocks` após mudar interface**: mitigado por `task mocks:verify` em CI (gate falha o pipeline).
- **Mockery v2 será descontinuado em favor de v3**: monitorar — adoção atual já estabelecida; migração futura simples.

## Plano de Implementação

Fase 1 da techspec: criar `mockery.yml`, adicionar tarefa `task mocks` ao Taskfile, gerar mocks de `Storage` e `Registry`, commitar `mocks/`. Adicionar `task mocks:verify` ao gate de CI.

## Monitoramento e Validação

- CI executa `task mocks:verify` em cada PR.
- Validação local: `task mocks && git diff` mostra mudanças.

## Revisão Futura

Avaliar migração para `mockery v3` quando estável (v3.x atualmente em desenvolvimento ativo). Adicionar interfaces de outros módulos conforme PRDs forem aprovados — esta ADR estabelece o ponto de extensão, não o limite.
