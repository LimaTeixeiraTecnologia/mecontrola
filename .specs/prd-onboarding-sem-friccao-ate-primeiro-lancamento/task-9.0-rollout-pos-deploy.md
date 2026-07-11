# Tarefa 9.0: Checklist de rollout sem feature flag e validação pós-deploy

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Documentar o rollout sem feature flag/allowlist/canary e conduzir a validação pós-deploy com o usuário de teste do requester, garantindo que nenhuma resposta afirme sucesso sem retorno real da ferramenta/use case.

<requirements>
- RF-24: sistema não afirma registro de gastos/receitas na conclusão do onboarding enquanto o caminho não estiver funcional e testado.
- RF-25: nenhuma resposta afirma sucesso de cadastro de 💳 ou lançamento sem retorno real da ferramenta/use case.
</requirements>

## Subtarefas

- [ ] 9.1 Criar/atualizar checklist de rollout sem feature flag.
- [ ] 9.2 Validar configs obrigatórias: `TRANSACTIONS_ENABLED=true`, `OUTBOX_DISPATCHER_ENABLED=true`, OpenRouter funcional.
- [ ] 9.3 Executar jornada manual com usuário de teste do requester.
- [ ] 9.4 Verificar `workflow_runs`, `workflow_steps`, `platform_messages`, `outbox_events`, `platform_runs` e linha ativa em `transactions`.
- [ ] 9.5 Confirmar SLO: 95% das ativações até primeira transação ativa em até 5 minutos (excluindo espera do usuário).
- [ ] 9.6 Documentar procedimento de rollback por reversão de deploy.

## Detalhes de Implementação

Ver `techspec.md` — seções **Sequenciamento de Desenvolvimento / Dependências Técnicas**, **Testes E2E, Golden e Pós-Deploy** e ADRs `adr-003-rollout-sem-feature-flag.md` e `adr-004-slo-observabilidade-falso-sucesso.md`.

## Critérios de Sucesso

- Checklist de rollout documentado e revisado.
- Jornada manual cria transação ativa rastreável para pix e receita.
- Nenhuma pergunta de 💳 aparece para pix, dinheiro, boleto, débito, TED, vale ou receita.
- Rollback por reversão de deploy está documentado.
- Todos os gates anteriores (1.0 a 8.0) estão `done`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Jornada manual pós-deploy.
- [ ] Revisão de checklist e runbook.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/adr-003-rollout-sem-feature-flag.md`
- `.specs/prd-onboarding-sem-friccao-ate-primeiro-lancamento/adr-004-slo-observabilidade-falso-sucesso.md`
- `docs/runbooks/` (ou equivalente)
