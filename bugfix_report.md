# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 0 arquivos; validacao shell direcionada
- Pendentes: nenhum
- Estado final: done

## Bugs
- ID: pretooluse-preload-false-positive
- Severidade: major
- Origem: issue do usuario
- Estado: fixed
- Causa raiz: os hooks `validate-preload.sh` bloqueavam por padrao em uma variavel de ambiente que nao comprova se `AGENTS.md` e as skills foram carregados no contexto do agente. O mirror do Codex ainda bloqueava chamadas sem `file_path`, incluindo comandos sem alvo de arquivo.
- Arquivos alterados: `.claude/hooks/validate-preload.sh`, `.github/hooks/validate-preload.sh`, `.codex/hooks/validate-preload.sh`
- Teste de regressao: execucao direta dos hooks com payload de arquivo Go, modo estrito, alvo Node sem skill e chamada sem `file_path`
- Validacao: todos os cenarios esperados passaram

## Comandos Executados
- `bash -n .claude/hooks/validate-preload.sh .github/hooks/validate-preload.sh .codex/hooks/validate-preload.sh` -> passou
- `printf '%s' '{"tool_input":{"file_path":"internal/platform/outbox/event.go"}}' | bash .claude/hooks/validate-preload.sh` -> exit 0
- `printf '%s' '{"tool_input":{"file_path":"internal/platform/outbox/event.go"}}' | bash .github/hooks/validate-preload.sh` -> exit 0
- `bash .codex/hooks/validate-preload.sh` -> exit 0 sem `file_path`
- `printf '%s' '{"tool_input":{"file_path":"internal/platform/outbox/event.go"}}' | GOVERNANCE_PRELOAD_MODE=fail bash .claude/hooks/validate-preload.sh` -> exit 1 esperado
- `printf '%s' '{"tool_input":{"file_path":"web/app.ts"}}' | bash .claude/hooks/validate-preload.sh` -> exit 1 esperado por skill `node-implementation` ausente
- `printf '%s' '{"tool_input":{"file_path":"internal/platform/outbox/event.go"}}' | bash .codex/hooks/validate-preload.sh` -> exit 0
- `cmp -s .claude/hooks/validate-preload.sh .github/hooks/validate-preload.sh` -> passou
- `cmp -s .claude/hooks/validate-preload.sh .codex/hooks/validate-preload.sh` -> passou

## Riscos Residuais
- O hook continua sem conseguir provar leitura real de contexto por modelo; essa parte permanece procedimental. O gate programatico preservado valida prerequisitos observaveis de skill e emite guidance cirurgico.
