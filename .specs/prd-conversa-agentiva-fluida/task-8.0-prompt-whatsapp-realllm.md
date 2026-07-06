# Tarefa 8.0: Prompt, Resposta Final WhatsApp e Validação RUN_REAL_LLM

<critical>Ler prd.md, techspec.md e scenarios.md desta pasta — tarefa invalidada se pulado</critical>

## Visão Geral

Atualizar as instruções do agente para refletir o novo contrato de pendência conversacional, garantir que respostas finais sejam compatíveis com WhatsApp (PT-BR, texto curto, sem markdown incompatível, sem menção à infraestrutura) e validar com LLM real (`RUN_REAL_LLM=1`) que o modelo segue as instruções sem declarar sucesso simulado. O harness determinístico (7.0) é o gate primário; RUN_REAL_LLM é o gate de produção obrigatório antes de `done`.

<requirements>
- Prompt/instruções do agente refletem: uma pergunta por vez (RF-04), não re-perguntar dados já informados (RF-05), aceitar respostas curtas e elípticas (RF-06), confirmar sucesso apenas após write real (RF-21), linguagem WhatsApp (RF-24)
- Respostas finais de sucesso: "Despesa de R$ X registrada em *<Raiz> > <Folha>* para <data> no <pagamento> ✅" — curtas, em PT-BR, sem markdown incompatível, sem "registrei no banco", sem "sistema interno"
- Respostas de clarificação: uma pergunta por mensagem; sem menu enumerado longo; sem linguagem técnica
- Respostas de erro: sem afirmar sucesso; sem mencionar stack trace, workflow_id ou correlation_key
- Resposta de cancelamento: "Tudo certo, o registro foi cancelado." — sem valor nem categoria
- Resposta de expiração: "O registro expirou. Para registrar, envie a informação completa novamente."
- Resposta de confirmação: "Confirma? R$ X em *<Raiz> > <Folha>* para <data> no <pagamento>?" — antes de toda escrita (spec-version 3)
- Resposta de múltiplos candidatos: lista legível com nomes de categoria (não IDs); sem escolha automática
- RUN_REAL_LLM=1 obrigatório: validar com OPENROUTER_* do .env que agente real produz respostas compatíveis em todos os fluxos acima
- Zero comentários Go de produção em arquivos de instruções/prompt gerados como .go
</requirements>

## Subtarefas

- [ ] 8.1 Atualizar `instructions.go` (ou equivalente) do agente: adicionar seção sobre pendência ativa, uma pergunta por vez, não re-solicitar dados preservados, confirmar só após write, linguagem WhatsApp
- [ ] 8.2 Revisar e atualizar templates de resposta: sucesso (com valor+categoria+data+pagamento), clarificação (slot por slot), erro (sem sucesso), cancelamento, expiração, confirmação, múltiplos candidatos
- [ ] 8.3 Verificar que prompt não menciona infraestrutura: grep para "workflow_id", "pending-entry", "correlation_key", "platform", "resource_id", "thread_id" em strings de resposta do agente — deve retornar vazio
- [ ] 8.4 Executar `RUN_REAL_LLM=1` nos cenários CA-01..CA-17 e nos fluxos G7-20 (completo), G7-04 (cancelamento), G7-08 (expiração), G7-15 (erro de ledger), G7-03 (raiz sem folha): verificar que o agente real não declara sucesso sem write, não menciona infraestrutura, faz uma pergunta por vez e preserva dados dos turnos anteriores
- [ ] 8.5 Documentar evidências do RUN_REAL_LLM em `docs/runs/<YYYY-MM-DD>-evidence-pending-entry-realllm.md` com logs de cada cenário testado

## Detalhes de Implementação

Ver `techspec.md` seção **"Conformidade com Padrões"** e `prd.md` **RF-24** e **Experiência do Usuário**.

Exemplos de resposta esperada (WhatsApp-compatible):

- Sucesso: `Despesa de R$ 150,00 registrada em *Custo Fixo > Supermercado* para hoje no pix ✅`
- Confirmação: `Confirma? R$ 150,00 em *Custo Fixo > Supermercado* para hoje no pix?`
- Clarificação de categoria (único slot): `Qual categoria para essa compra no mercado?`
- Múltiplos candidatos: `Qual se encaixa melhor? 1. Plano de Saúde 2. Consultas e Exames 3. Terapia e Saúde Mental`
- Cancelamento: `Tudo certo, o registro foi cancelado.`
- Expiração: `O registro expirou. Para registrar, envie a informação completa novamente.`
- Erro de ledger: `Não consegui registrar. Tente novamente em breve.`

Regras inegociáveis de resposta (RF-24, M-03=0):
- Nunca usar "registrei", "anotei", "salvo", "gravado" sem `CreateTransaction` ter retornado sucesso com ID
- Nunca mencionar "workflow", "pending-entry", "correlation", "platform" em texto ao usuário
- Nunca fazer duas perguntas na mesma mensagem
- Nunca reutilizar dados de pendência expirada ou substituída

`RUN_REAL_LLM=1`: executar `go test -run TestRealLLM ./internal/agents/... -v` com `OPENROUTER_API_KEY` e `OPENROUTER_BASE_URL` do `.env`. Cada cenário deve registrar: prompt enviado, resposta recebida, assertiva pass/fail. Evidência é obrigatória antes de marcar tarefa como `done`.

## Critérios de Sucesso

- `go build ./internal/agents/...` passa (instruções atualizadas não quebram compilação)
- `go test -race -count=1 ./internal/agents/...` verde (testes de prompt/resposta unitários)
- CA-01..CA-17 com RUN_REAL_LLM=1: todos passam com agente real
- M-03=0: nenhuma resposta de sucesso sem write real nos cenários RUN_REAL_LLM
- Grep de infraestrutura em strings de resposta retorna vazio (8.3)
- Evidência em `docs/runs/<data>-evidence-pending-entry-realllm.md` com todos os cenários documentados
- RF-24: respostas em PT-BR, curtas, sem markdown incompatível, sem infraestrutura interna

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários de resposta: verificar formato das strings de resposta (sucesso, cancelamento, expiração, erro, confirmação, múltiplos candidatos) sem LLM real
- [ ] `RUN_REAL_LLM=1` — cenários obrigatórios: CA-01 (retomada correta), CA-02 (substituição), CA-03 (sim e pix), CA-04 (múltiplos candidatos), CA-05 (cancelamento), CA-06 (erro de ledger), CA-07 (replay), CA-08 (expiração), CA-09 (raiz sem folha), CA-10 (cartão), CA-11 (pendência substituída), CA-12 (Run auditável)
- [ ] Grep de infraestrutura em strings ao usuário: `grep -r "workflow_id\|pending-entry\|correlation_key\|resource_id\|thread_id" internal/agents/` deve retornar vazio em strings retornadas ao usuário

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>
<critical>RUN_REAL_LLM=1 É GATE OBRIGATÓRIO — não marcar como `done` sem evidência documentada em docs/runs/</critical>

## Arquivos Relevantes

- `internal/agents/application/agents/mecontrola_agent.go` (instruções do agente — atualizar)
- Arquivo de instructions/system prompt do agente (verificar path real no código)
- `docs/runs/<YYYY-MM-DD>-evidence-pending-entry-realllm.md` (novo — evidências RUN_REAL_LLM)
- `.specs/prd-conversa-agentiva-fluida/prd.md` (RF-24, Experiência do Usuário, CA-01..CA-17)
- `.specs/prd-conversa-agentiva-fluida/techspec.md` (seção "Conformidade com Padrões")
- `.specs/prd-conversa-agentiva-fluida/scenarios.md` (Convenção Global de Confirmação spec-version 3, G7-20, G7-04, G7-08, G7-15)
