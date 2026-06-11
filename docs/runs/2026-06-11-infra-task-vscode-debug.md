# Infra task isolado + debug VS Code

## Motivação

Permitir subir apenas a infraestrutura (postgres + otel-lgtm) sem server/worker em container,
para rodar os binários localmente no debugger do VS Code com breakpoints ativos.

## Arquivos alterados

- `taskfiles/local.yml` — nova task `infra`
- `.vscode/launch.json` — configurações `migrate`, `server`, `worker` + compound `server + worker`

## Fluxo de uso

```
task local:infra          # sobe postgres + otel-lgtm
# VS Code Run & Debug → "migrate"         (one-shot, encerra após migrar)
# VS Code Run & Debug → "server + worker" (compound, breakpoints ativos)
```
