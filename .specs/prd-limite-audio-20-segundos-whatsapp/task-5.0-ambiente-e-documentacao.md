# Tarefa 5.0: Ambiente e documentação: .env.example, prod.env e runbooks de áudio

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Alinhar os artefatos de ambiente e a documentação operacional ao novo comportamento: limite de 20s, nova chave de mensagem e impacto nos procedimentos de pós-deploy, evitando drift silencioso entre documentação e runtime.

<requirements>
- RF-08: documentação operacional e exemplos de configuração atualizados no mesmo ciclo da mudança.
</requirements>

## Subtarefas

- [ ] 5.1 `.env.example:240`: `AGENT_AUDIO_MAX_DURATION=20s`; adicionar `WA_MSG_AUDIO_DURATION_EXCEEDED` com o texto default após `:245`, mantendo o bloco de comentários coerente.
- [ ] 5.2 `deployment/config/prod.env:218`: `AGENT_AUDIO_MAX_DURATION=20s`; adicionar a nova chave após `:223`.
- [ ] 5.3 `deployment/runbooks/audio-whatsapp-stt.md`: tabela de configs (`:80`) com default 20s; nova linha para a chave de mensagem após `:85`; nota na tabela de reasons (`:63`) indicando a resposta dedicada para `duration_exceeded`; lista de pré-requisitos de canary (`:340-341`) com a nova chave; nota de acoplamento entre o número da mensagem e o limite configurado (ADR-002).
- [ ] 5.4 `deployment/runbooks/audio-whatsapp-pos-deploy.md:185-188`: reescrever o cenário 3.4, que hoje usa áudio de 55-60s esperando sucesso; o caminho feliz passa a usar áudio de até 20s e o áudio de 55-60s vira cenário de rejeição esperada com a mensagem dedicada.
- [ ] 5.5 Nenhuma mudança em compose, dashboards ou outros arquivos de deployment.

## Detalhes de Implementação

Ver `techspec.md` seções `Sequenciamento de Desenvolvimento` (passo 4), `Monitoramento e Observabilidade` (Runbook) e `Riscos Conhecidos` (falso positivo de regressão documental). Não existe gate de CI de paridade entre `.env.example`, `config.go` e `prod.env`; a conferência manual dos três artefatos é parte desta tarefa.

## Critérios de Sucesso

- Grep por `AGENT_AUDIO_MAX_DURATION` mostra `20s` em `.env.example`, `prod.env` e runbook, e `60s` apenas na faixa de validação documentada `[1s..60s]`.
- Grep por `WA_MSG_AUDIO_DURATION_EXCEEDED` mostra a chave presente nos três artefatos de configuração e no runbook.
- Cenário 3.4 do runbook de pós-deploy consistente com o comportamento novo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `.env.example`
- `deployment/config/prod.env`
- `deployment/runbooks/audio-whatsapp-stt.md`
- `deployment/runbooks/audio-whatsapp-pos-deploy.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-001-limite-20s-via-config-com-default-sem-endurecer-faixa.md`
- `.specs/prd-limite-audio-20-segundos-whatsapp/adr-002-selecao-de-mensagem-por-reason-no-usecase.md`
