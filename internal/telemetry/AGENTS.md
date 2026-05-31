# Agentes de IA — Módulo `telemetry`

Consulte `README.md` nesta pasta para o scaffold pattern completo e comandos `ai-spec` recomendados ao criar agregados neste módulo nos PRDs subsequentes.

## Comandos `ai-spec` rápidos

```bash
# Criar PRD de nova feature neste módulo
ai-spec create-prd

# Derivar techspec do PRD aprovado
ai-spec create-technical-specification

# Decompor em tarefas
ai-spec create-tasks

# Executar tarefa
ai-spec execute-task

# Verificar drift de spec
ai-spec check-spec-drift .specs/prd-telemetry-<feature>/tasks.md
```

Fronteiras de import enforçadas por `depguard` em `.golangci.yml` — ver `README.md` para tabela completa.
