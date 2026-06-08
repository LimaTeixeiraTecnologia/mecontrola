# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | Ciclo de fatura (closing/due) é definido pelo emissor, não pela bandeira | Visa Core Rules e Mastercard Rules delegam ciclo ao emissor; BACEN Res. 4.658 e BCB 468/2025 não fixam dia de fechamento | `closing_day` e `due_day` são atributos por cartão, configurados pelo usuário | confirmada |
| H2 | `mecontrola` é aplicação não-PCI e não pode armazenar PAN/CVV | PCI-DSS 4.0 + LGPD art. 6º e 46; PCI SSC FAQ proíbe CVV pós-autorização | Modelo só persiste bandeira + last4 + holder_name + ciclo; nunca PAN/CVV | confirmada |
| H3 | A regra do usuário sobre fatura (compra antes/depois do fechamento) é monotônica e determinística | Exemplo informado (02/jun→01/jul; 28/jun→01/ago) é coerente com regra "se purchase_date ≤ closing_date_corrente, vence no próximo due; senão, vence no due seguinte" | Função `InvoiceFor` pura, testável, sem efeitos colaterais | confirmada |
| H4 | Cálculo precisa lidar com edge cases de calendário (mês 31, fev/29, virada de ano) | Calendário gregoriano + dias variáveis por mês; closing_day ou due_day = 31 quebra em fevereiro | Algoritmo precisa de regra de clamp (ex.: usar último dia do mês quando day > daysInMonth) | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H5 | Usuário sempre informará `closing_day` e `due_day` como dias do mês (1-31), não como offset (X dias antes do vencimento) | Se modelo for `due_day + offset_days`, modelo de dados e UX mudam | Rodada 2 — perguntar modelo preferido | pendente |
| H6 | Cartão é entidade independente de "conta bancária"/"wallet" | Se for sub-tipo, mudará boundary do módulo | Rodada 2 — perguntar fronteira | pendente |
| H7 | Timezone canônico do cálculo é `America/Sao_Paulo` | Se usuários internacionais existem, datas mudam | Rodada 2 — confirmar timezone | pendente |
| H8 | Bandeira (Visa/Master/Elo/etc.) é metadado opcional ou obrigatório no MVP | Define complexidade (BIN lookup ou enum simples) | Rodada 2 — confirmar campos do CRUD | pendente |
| H9 | Soft-delete vs hard-delete de cartões | Hard-delete pode orfanizar transações futuras; soft-delete exige `deleted_at` | Rodada 2 — confirmar | pendente |

## Restrições Confirmadas
- Stack obrigatório: Go, arquitetura do `mecontrola` (AGENTS.md, R0–R7, DDD, sem `init()`, sem `panic` em produção, `context.Context` em IO).
- Não-PCI: proibido persistir PAN completo, CVV, trilha, PIN.
- Production-ready inegociável: cobertura de testes para edge cases de calendário, observabilidade básica, idempotência onde aplicável.
- O módulo será consumido por módulo de transações ainda não construído — contrato (`InvoiceFor`) precisa ser estável.

## Preferências Não Bloqueantes
- Bandeira e last4 são "nice-to-have" para UX, mas não decididos como obrigatórios no MVP.
- Limite de crédito, multi-titular, emissor (banco) ficam fora do MVP.
- Validação de Luhn pode entrar se last4 for inserido junto com confirmação de bandeira.
