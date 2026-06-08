# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | A soma dos percentuais distribuídos entre categorias não pode ultrapassar 100%. | Restrição explícita do usuário. | Impõe validação de consistência na configuração do orçamento. | confirmada |
| H2 | O gasto realizado pode ultrapassar o valor planejado da categoria. | Restrição explícita do usuário. | O limite é informativo, não bloqueante. | confirmada |
| H3 | Deve existir no máximo um orçamento por mês e usuário. | Requisito explícito do usuário. | Define unicidade de negócio mensal por usuário. | confirmada |
| H4 | Lançamentos devem poder entrar via API e via evento. | Requisito explícito do usuário. | Exige decisão sobre autoridade, idempotência e consistência entre canais. | confirmada |
| H5 | Controle preventivo e acompanhamento posterior possuem igual prioridade no MVP. | Escolha P1-C da Rodada 1. | O MVP precisa expor planejamento e realizado de forma acionável. | confirmada |
| H6 | Precisão financeira, clareza e alertas acionáveis são critérios obrigatórios. | Escolha P2-D da Rodada 1. | Impõe qualidade mínima ampla e exige forte contenção de escopo em funcionalidades secundárias. | confirmada |
| H7 | Ultrapassar o orçamento deve gerar notificação ativa. | Escolha P3-C da Rodada 1. | Introduz dependência de entrega de notificação ou contrato explícito com outro módulo. | confirmada |
| H8 | Evitar excesso de escopo é o risco prioritário. | Escolha P4-C da Rodada 1. | Alternativas serão avaliadas com preferência por limites explícitos e entrega incremental. | confirmada |
| H9 | Todo lançamento deve informar obrigatoriamente a categoria de orçamento. | Escolha P1-A da Rodada 2. | Não haverá classificação automática nem fila de não classificados no MVP. | confirmada |
| H10 | O MVP deve expor criação, edição e exclusão de despesas. | Escolha P2-C da Rodada 2. | Exige semântica de correção e trilha auditável para preservar confiabilidade. | confirmada |
| H11 | Alertas devem admitir múltiplos canais e limiares configuráveis. | Escolha P3-C da Rodada 2. | Requer capacidade de preferências e entrega, própria ou externa. | confirmada |
| H12 | Orçamento e percentuais ficam imutáveis após ativação. | Escolha P4-A da Rodada 2. | Simplifica o histórico e impede redistribuição durante o mês. | confirmada |
| H13 | Lançamentos confirmados devem aparecer imediatamente nas consultas. | Escolha P1-A da Rodada 3. | Favorece atualização transacional e restringe projeções assíncronas como fonte primária. | confirmada |
| H14 | Preferências e entrega multicanal pertencem ao módulo budgets. | Escolha P2-B da Rodada 3. | Amplia responsabilidade, dependências externas e operação do módulo. | confirmada |
| H15 | Edições e exclusões físicas devem manter somente o estado financeiro atual. | Escolha P1-B da Rodada 4. | Reduz auditabilidade; exige mecanismo confiável de atualização e recálculo dos totais atuais. | confirmada |
| H16 | A entrega de notificações deve usar providers/adapters existentes ou futuros. | Escolha P2-B da Rodada 4 e explicação do usuário. | Budgets não deve implementar diretamente o protocolo de entrega; depende de contrato e adapter. | confirmada |
| H17 | Falha de notificação não pode afetar a despesa confirmada. | Escolha P3-A da Rodada 4. | Exige persistência e retry desacoplados da transação financeira principal. | confirmada |
| H18 | Total financeiro incorreto ou não reconciliável é risco inaceitável. | Escolha P4-A da Rodada 4. | Reconciliação do estado atual e idempotência são requisitos centrais. | confirmada |
| H19 | Não haverá histórico de edições ou exclusões. | Escolha P1-A da Rodada 4.1. | Reconciliação cobre somente o estado atual; investigação retroativa fica inviável. | confirmada |
| H20 | O envio real por WhatsApp via agente LLM fica fora do critério de conclusão do MVP. | Decisão B da Rodada 5, revisando a escolha P2-C da Rodada 4.1. | Remove dependência bloqueante; o MVP entrega alertas persistentes prontos para integração. | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H21 | Cada categoria de orçamento possui um percentual configurado pelo usuário. | O módulo de categorias pode possuir regras próprias ou categorias não orçamentáveis. | Validar no futuro discovery técnico. | usuário |
| H22 | API e evento podem representar o mesmo lançamento financeiro. | Sem identidade compartilhada, o total pode ser contado duas vezes. | Definir autoridade e chave de idempotência no discovery técnico. | usuário |
| H23 | Percentuais são suficientes para representar o planejamento mensal. | Usuários podem precisar informar valores fixos ou sobras não alocadas. | Validar no discovery técnico/produto. | usuário |
| H24 | O agente LLM futuro fornecerá contrato e disponibilidade adequados para envio via WhatsApp. | Se falsa, a evolução de entrega ativa por WhatsApp será bloqueada, sem impedir o MVP de budgets. | Projetar e validar o agente e seu contrato antes da integração futura. | usuário/equipe técnica |

## Restrições Confirmadas
- MVP deve ser robusto e production-ready, sem ampliar escopo por objetivos artificiais.
- Distribuição percentual total deve ser menor ou igual a 100%.
- Gastos acima do planejado são permitidos e devem alertar o usuário.
- Orçamento é único por mês e usuário.
- O MVP deve evitar funcionalidades secundárias que atrasem a entrega dos critérios obrigatórios.
- Lançamentos sem categoria e classificação automática ficam fora do MVP.
- Orçamentos ativados não podem ser alterados.
- Consistência eventual não pode ser a experiência primária de consulta do MVP.
- Budgets assume preferências, decisão e persistência dos alertas; a entrega usa provider/adapter.
- Falhas de notificação não podem comprometer lançamentos financeiros confirmados.
- O estado financeiro atual deve ser reconciliável mesmo após edições e exclusões físicas.
- Auditoria histórica de edições e exclusões fica fora do MVP.
- O MVP entrega alertas persistentes e contrato/provider preparado, sem exigir integração funcional com agente LLM/WhatsApp.

## Preferências Não Bloqueantes
- Visualização inspirada no painel fornecido, com planejado, gasto, percentual utilizado e totais.
- Categorias de orçamento serão relacionadas ao futuro módulo de categorias.
