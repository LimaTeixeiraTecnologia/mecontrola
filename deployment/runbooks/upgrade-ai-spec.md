# Runbook: Upgrade do ai-spec Harness

**Referências:** ADR-014 (tool pinning), ADR-015 (coverage report)

## Quando Usar

- Nova versão do `ai-spec-harness` disponível.
- `ai-spec doctor` reporta versão desatualizada.
- PRD ou techspec evoluíram e requerem versão mínima do harness.

## Pré-requisitos

```sh
which ai-spec
ai-spec --version
```

## Passo a Passo

### 1. Verificar versão atual e disponível

```sh
ai-spec upgrade --check
```

Saída esperada:
```
Current:   v1.x.y
Available: v1.x.z
```

### 2. Fazer upgrade

```sh
brew upgrade ai-spec-harness
# ou
go install github.com/ai-spec-harness/ai-spec-harness/cmd/ai_spec_harness@latest
```

### 3. Revalidar saúde do harness

```sh
ai-spec doctor
```

Deve retornar `OK` sem erros.

### 4. Revalidar lint das specs

```sh
ai-spec lint
```

Corrigir eventuais divergências reportadas antes de prosseguir.

### 5. Verificar drift de specs

```sh
ai-spec check-spec-drift .specs/prd-mecontrola-foundation/tasks.md
```

Nenhum RF descoberto sem cobertura deve aparecer.

### 6. Atualizar pin em `tools.go` (se aplicável)

Caso o harness seja instalado via `tools.go`:

```sh
go get github.com/ai-spec-harness/ai-spec-harness@<nova-versao>
go mod tidy
```

Commitar com:
```
chore(deps): upgrade ai-spec-harness to v<nova-versao>
```

### 7. Validar CI

Abrir PR com a atualização e verificar que o job `governance` do CI passa.

## Referências

- Repositório: https://github.com/ai-spec-harness/ai-spec-harness
- Skills disponíveis: `.agents/skills/`
