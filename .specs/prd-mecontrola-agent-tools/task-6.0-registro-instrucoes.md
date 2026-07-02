# Tarefa 6.0: Registro no agente + instruções determinísticas e anti-simulação

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Registrar as 15 tools novas no `mecontrola-agent` e atualizar as instruções do agente para
seleção determinística de tool, anti-simulação e limites de domínio financeiro pessoal. Depende
das tarefas 3.0, 4.0 e 5.0 (tools de leitura, `create_recurrence` e tools destrutivas já
construídas). Ver techspec.md, seção "Sequenciamento de Desenvolvimento" (passo 6).

<requirements>
- RF-20, RF-21, RF-24, RF-25, RF-31, RF-32.
- Dependência: 3.0, 4.0, 5.0.
</requirements>

## Subtarefas

- [ ] 6.1 Em `internal/agents/module.go`, estender `buildFinancialTools` para registrar todas as 15
  tools novas (passando `RecurrenceManager`, writer, `confirmEngine`/def onde aplicável),
  totalizando 24 tools.
- [ ] 6.2 Em `internal/agents/application/agents/mecontrola_agent.go`, reescrever as instruções com o
  catálogo de tools e regras determinísticas, anti-simulação e de domínio: declarar cada tool e
  quando usá-la (seleção determinística, RF-21); pedir apenas o dado faltante; NUNCA simular sucesso
  sem retorno real do use case (RF-24/RF-25); permanecer no domínio financeiro pessoal e recusar
  off-topic (RF-31); não expor capacidades do bucket 3 (jobs/consumers/infra, RF-32).

## Detalhes de Implementação

Ver techspec.md, seções "Arquitetura do Sistema", "Sequenciamento de Desenvolvimento" (passo 6) e
"Conformidade com Padrões". As 9 tools atuais estão em `internal/agents/module.go:254-262`; as 15
novas seguem o mapeamento tool → capacidade da tabela em techspec.md. As instruções seguem o molde
`internal/agents` sobre `internal/platform`, sem branching de domínio por kind (resolução por
registry, R-AGENT-WF-001.1).

## Critérios de Sucesso

- O agente registra 24 tools (9 atuais + 15 novas); build passa.
- As instruções cobrem seleção determinística de tool, anti-simulação (sem sucesso alucinado) e
  limites de domínio financeiro pessoal.
- Nenhuma capacidade de bucket 3 (jobs/consumers/infra) é exposta como tool.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — registro de tools, instruções do agente, scorers e verificação da superfície seguem o molde internal/agents sobre internal/platform.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Teste unitário verificando que o agente expõe as 24 tools esperadas. Integração N/A (a validação de
comportamento com LLM real é escopo da tarefa 7.0).

## Arquivos Relevantes
- `internal/agents/module.go`
- `internal/agents/application/agents/mecontrola_agent.go`
