# Tarefa 7.0: Gate lint:outbox-user-id + receita Taskfile

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa gate mecânico de CI que falha se construtor de `outbox.EventInput{...}` em código de produção for chamado sem `AggregateUserID:` populado, exceto quando o tipo do evento estiver na allowlist (`isSystemEvent`).

<requirements>
- RF-16: script `deployment/scripts/lint-outbox-user-id.sh` falha CI em construtor sem `AggregateUserID:`
- RF-17: receita `task lint:outbox-user-id` em `taskfiles/lint.yml`
- Allowlist mecânica: script lê `internal/platform/outbox/system_event_allowlist.go` e isenta event types listados (se a verificação for por `Type:` na mesma struct literal)
- Idempotente
- Validado adversarialmente (revert + restore)
</requirements>

## Subtarefas

- [ ] 7.1 Criar `deployment/scripts/lint-outbox-user-id.sh` que:
  - Encontra struct literais `outbox.EventInput{` em `.go` produção (excluindo `_test.go`, `mocks/`).
  - Para cada match, valida que entre `{` e `}` existe linha contendo `AggregateUserID:` OU o campo `Type:` referencia constante presente na allowlist.
  - Falha com lista de violações; sucesso com mensagem clara.
- [ ] 7.2 Adicionar receita `lint:outbox-user-id` em `taskfiles/lint.yml`:
  ```yaml
  outbox-user-id:
    desc: |
      Gate: todo construtor de outbox.EventInput em codigo de producao deve popular
      AggregateUserID, exceto event types na allowlist de sistema (ADR-004).
    cmds:
      - bash deployment/scripts/lint-outbox-user-id.sh
  ```
- [ ] 7.3 Validar adversarialmente: revert temporário de uma alteração de producer, rodar gate, confirmar FAIL; restaurar, confirmar PASS.

## Detalhes de Implementação

Ver PRD RF-16, RF-17. Padrão de implementação igual ao `lint:user-isolation` já existente.

## Critérios de Sucesso

- `task lint:outbox-user-id` PASS no estado pós-3.0/4.0/5.0/6.0.
- Simulação adversarial: gate FALHA com diagnóstico claro quando AggregateUserID removido; volta a PASS quando restaurado.
- Receita documentada em `taskfiles/lint.yml`.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `taskfile-production` — receita `lint:outbox-user-id` em `taskfiles/lint.yml` seguindo padrão idempotente do projeto (RF-16, RF-17)

## Testes da Tarefa

- [ ] Gate PASS no estado pós-callers
- [ ] Simulação adversarial (revert + FAIL + restore + PASS)
- [ ] Integração em `task lint` chamando subreceita

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/scripts/lint-outbox-user-id.sh` (novo)
- `taskfiles/lint.yml` (modificado)
