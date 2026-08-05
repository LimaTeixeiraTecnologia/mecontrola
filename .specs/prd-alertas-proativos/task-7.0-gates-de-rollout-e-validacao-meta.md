# Tarefa 7.0: Gates de rollout e validação Meta

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Transformar aprovação Meta, dry-run e opt-in em gates verificáveis para habilitar envio real por kind.

<requirements>
- Cobrir RF-05, RF-10, RF-11, RF-18, REQ-21 e REQ-25.
- Envio real por kind exige template configurado e `APPROVED`.
- Template `PENDING` não pode ser tratado como sucesso.
- Templates `MARKETING` exigem opt-in explícito.
</requirements>

## Subtarefas

- [ ] 7.1 Definir fonte de verdade local para status de template por kind.
- [ ] 7.2 Adicionar validação de config para envio real.
- [ ] 7.3 Criar comando/check operacional para listar readiness por kind.
- [ ] 7.4 Documentar procedimento de reconsulta Meta sem expor segredo.
- [ ] 7.5 Cobrir gates com testes.

## Detalhes de Implementação

Referenciar `techspec.md` seções `Configuração`, `Pontos de Integração` e `Monitoramento e Observabilidade`.

## Critérios de Sucesso

- Não há caminho de envio real sem template aprovado.
- Readiness por kind é auditável.
- MARKETING sem opt-in é bloqueado.
- Procedimento operacional não registra credenciais.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `design-patterns-mandatory` — valida que gates sejam diretos e auditáveis, sem abstração desnecessária.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./configs/...`
- [ ] `go test -race -count=1 ./internal/budgets/...`
- [ ] `go vet ./configs/... ./internal/budgets/...`

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `configs/config.go`
- `configs/config_test.go`
- `deployment/config/prod.env`
- `docs/refin/2026-08-05-meta-templates-alertas-proativos.md`
