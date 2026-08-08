# Relatório de Orquestração — indicador-digitando-whatsapp

Iniciado: 2026-08-08T09:04:32Z
Finalizado: 2026-08-08T13:20:00Z
Status final: **partial**

## Snapshot Inicial vs Final

| Métrica | Inicial | Final |
|---|---|---|
| Total de tarefas | 6 | 6 |
| done | 0 | 5 (1.0, 2.0, 3.0, 4.0, 5.0) |
| blocked | 0 | 1 (6.0) |
| pending | 6 | 0 |

## Tarefas Executadas

| # | Título | Status | Report |
|---|---|---|---|
| 1.0 | Feature flag `AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED` na configuração | done | `1.0_execution_report.md` |
| 2.0 | Método `SendTypingIndicator` no client Meta com payload oficial | done | `2.0_execution_report.md` |
| 3.0 | Método `SendTypingIndicator` no gateway de onboarding | done | `3.0_execution_report.md` |
| 4.0 | Emissão best-effort no `WhatsAppInboundConsumer` com métrica | done | `4.0_execution_report.md` |
| 5.0 | Wiring do módulo agents e do worker (ajuste do stub de boot) | done | `5.0_execution_report.md` |
| 6.0 | Gate de versão RF-07 e validação completa de zero regressão | **blocked** | `6.0_execution_report.md` |

## Waves

- **Wave 1** (paralela): 1.0 + 2.0 — done
- **Wave 2** (paralela): 3.0 + 4.0 — done
- **Wave 3** (sequencial): 5.0 — done
- **Wave 4** (sequencial): 6.0 — **blocked**

## Motivo do Bloqueio (6.0)

A implementação de código (tarefas 1.0–5.0) está **completa, testada e aprovada**: `go build ./...`,
`go vet ./...`, `go test -race -count=1 ./...` (149 pacotes), `golangci-lint`, todos os gates de
governança do repositório (zero comentários, SQL fora de adapter, cardinalidade de métricas, sem
`internal/agent`, sem switch por `intent.Kind`), a suíte e2e de onboarding (31 cenários / 151 steps)
e a suíte de integração de WhatsApp/agents — todos verdes, com a flag `AGENT_WHATSAPP_TYPING_INDICATOR_ENABLED`
desligada (padrão), zero alteração de asserts ou contagens existentes.

O que resta é puramente uma ação **humana e externa ao repositório**, exigida pelo próprio RF-07 e
pelas subtarefas 6.1/6.2 do PRD: uma chamada real de `SendTypingIndicator` contra a Graph API na
versão pinada, em ambiente de teste com número WhatsApp real, com captura de tela da bolha de
digitação e dos ticks azuis no aparelho (ou registro do erro real da Meta, caso a versão pinada
rejeite o campo). Nenhum agente de IA tem acesso a essas credenciais/dispositivo, e fabricar essa
evidência violaria a política de zero falso positivo. Por isso a tarefa terminou `blocked`, não
`done` — a flag permanece desligada em todos os ambientes, inclusive produção, até essa evidência
ser anexada por um operador humano.

## Próximos Passos

1. Um operador humano com acesso a ambiente de teste Meta e a um telefone real deve executar o
   script/teste manual guiado descrito na tarefa 2.0 (subtarefas 6.1/6.2), anexando à pasta
   `.specs/prd-indicador-digitando-whatsapp/`: status HTTP, corpo da resposta e evidência visual
   (print da bolha/ticks azuis), ou o erro real da Meta em caso de rejeição da versão pinada.
2. Após anexar essa evidência, reabrir a tarefa 6.0 (ou criar tarefa de follow-up) para promover o
   estado de `blocked` para `done` com o veredito final sobre a ativação da flag por ambiente.
3. Se a versão pinada rejeitar `typing_indicator`, abrir decisão separada de bump de versão da
   Graph API (fora do escopo deste PRD), cobrindo em regressão o client de mensagens e o de mídia.
4. Nenhuma alteração de código é necessária até essa validação humana — a implementação está
   completa e não deve ser retrabalhada.

## Conformidade com o PRD

- 100% dos requisitos funcionais (RF-01 a RF-10) estão implementados e cobertos por teste, conforme
  a tabela de cobertura de `tasks.md`.
- RF-07 especificamente depende de fato externo (versão da Graph API aceitar `typing_indicator`),
  que por desenho do PRD (`## Riscos de Integração`, `tasks.md`) "pode bloquear a ativação sem
  bloquear o merge" — este é exatamente o estado em que o PRD termina: código mergeável, ativação
  pendente de gate humano.
- Não há desvios, lacunas de implementação, flexibilizações de regra ou regressões introduzidas.
