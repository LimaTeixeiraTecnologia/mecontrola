# DECISION BRIEF

## Problema
Usuários precisam planejar um orçamento mensal por categorias e acompanhar gastos realizados sem perder confiabilidade quando lançamentos entram por API ou evento. O sistema deve permitir gastos acima do planejado, sinalizar desvios e evitar que duplicidade, concorrência, edição ou exclusão produzam totais incorretos.

## Objetivo
Entregar um MVP production-ready que combine controle preventivo e acompanhamento posterior. O sucesso exige estado financeiro atual correto e reconciliável, leitura clara de planejado versus gasto e alertas persistentes quando limiares configurados forem atingidos.

## Escopo Inicial
Inclui:
- Um orçamento único por usuário e mês, com valor total e distribuição percentual por categoria.
- Soma dos percentuais limitada a 100%.
- Ativação que torna orçamento e percentuais imutáveis.
- Categoria obrigatória em toda entrada de despesa.
- Criação, edição e exclusão física de despesas.
- Entradas por API e evento com identidade idempotente.
- Atualização transacional e consulta imediatamente consistente.
- Totais atuais recalculáveis a partir das despesas existentes.
- Limiares configuráveis, alertas persistentes e retry independente da despesa.
- Contrato/provider preparado para futura integração com agente LLM e WhatsApp.

Exclui:
- Histórico de valores editados ou despesas excluídas.
- Classificação automática e lançamentos sem categoria.
- Redistribuição do orçamento após ativação.
- Bloqueio de despesas que ultrapassem o planejado.
- Envio real de mensagens por WhatsApp e implementação do agente LLM.
- Ledger assíncrono e event sourcing completo.

## Restrições
- O estado financeiro atual não pode ficar incorreto ou não reconciliável.
- A confirmação da despesa não pode depender da entrega do alerta.
- Gastos podem ultrapassar o planejado.
- O MVP deve conter escopo sem enfraquecer consistência e idempotência.

## Hipóteses
- Categorias de orçamento serão fornecidas pelo futuro módulo de categorias.
- Percentuais são suficientes para representar o planejamento mensal inicial.
- API e evento podem representar o mesmo lançamento e precisam compartilhar identidade idempotente.
- Um agente LLM futuro poderá consumir o contrato/provider de alertas e entregar mensagens via WhatsApp.

## Alternativas Avaliadas
### Alternativa 1 - Acumulado direto e notificações dentro de budgets
Resumo:
Atualiza diretamente os acumulados e aplica diferenças em edições/exclusões.

Viabilidade:
Rápida e barata inicialmente, mas frágil sob concorrência, duplicidade e reconciliação. Não atende o risco inaceitável definido.

### Alternativa 2 - Registro canônico reconciliável com atualização transacional
Resumo:
Normaliza entradas em despesas canônicas idempotentes e atualiza estado e totais na mesma fronteira transacional.

Viabilidade:
Equilibra consistência imediata, reconciliação do estado atual, prazo e operação. Alertas persistentes permanecem desacoplados da confirmação financeira.

### Alternativa 3 - Ledger imutável com projeções assíncronas
Resumo:
Representa mudanças como lançamentos imutáveis e deriva totais por projeções.

Viabilidade:
Robusta e escalável, mas introduz consistência eventual e operação assíncrona incompatíveis com o MVP escolhido.

### Alternativa 4 - Event sourcing completo do orçamento
Resumo:
Deriva integralmente configuração, despesas, totais e alertas de eventos.

Viabilidade:
Oferece máxima reconstrução histórica, porém possui custo, prazo e complexidade desproporcionais.

## Trade-offs
- Edição/exclusão física: simplicidade e estado atual foram priorizados sobre auditoria histórica.
- Consistência imediata: foi priorizada sobre desacoplamento e escala assíncrona.
- Alertas persistentes: aumentam a operação de budgets, mas não comprometem a transação financeira.
- WhatsApp/LLM: integração real foi retirada do critério de conclusão para remover dependência bloqueante.

## Riscos
- Risco: API e evento contabilizarem a mesma despesa duas vezes.
  Impacto: total financeiro incorreto.
  Probabilidade: alta sem identidade compartilhada.
  Mitigação: chave idempotente canônica e restrição de unicidade.
- Risco: edição/exclusão física impedir investigação retroativa.
  Impacto: ausência de explicação histórica e segurança investigativa reduzida.
  Probabilidade: certa por decisão de escopo.
  Mitigação: aceitar explicitamente no MVP e manter reconciliação do estado atual.
- Risco: falha persistente no processamento de alertas.
  Impacto: usuário não recebe sinalização futura.
  Probabilidade: média.
  Mitigação: estado persistente, retry, expiração e observabilidade.
- Risco: contrato futuro do agente LLM divergir do provider preparado.
  Impacto: retrabalho de integração.
  Probabilidade: média.
  Mitigação: validar contrato no discovery técnico e manter fronteira estreita.

## Custos
Estimativa relativa:
média

Drivers de custo:
- Consistência transacional entre despesa e totais.
- Idempotência compartilhada entre API e evento.
- Reconciliação e recálculo dos totais atuais.
- Persistência, retry e observabilidade de alertas.
- Testes de concorrência, duplicidade, edição e exclusão.

## Impactos Operacionais
- Suporte consegue reconciliar o estado atual, mas não investigar valores anteriores excluídos ou editados.
- Alertas exigem processamento confiável, retries, expiração e tratamento de falhas permanentes.
- Operação precisa distinguir saúde financeira da saúde da fila de alertas.
- Integração futura com WhatsApp/LLM pode evoluir sem bloquear o núcleo do MVP.

## Segurança
- Toda operação e consulta deve ser isolada por usuário.
- API e eventos precisam de autenticação/autorização adequadas à origem.
- Ausência de histórico reduz capacidade de investigar alterações indevidas.
- Dados financeiros e payloads de alertas devem evitar exposição indevida.

## Observabilidade
- Métricas de lançamentos aceitos, duplicados, rejeitados, editados e excluídos.
- Métricas de divergência detectada em reconciliação.
- Métricas de alertas criados, pendentes, entregues futuramente, expirados e com falha.
- Logs e traces correlacionados por usuário, orçamento, despesa, origem e chave idempotente.

## Escalabilidade
- Atualização transacional atende o MVP e oferece leitura imediatamente consistente.
- Contenção concorrente por orçamento/categoria deve ser avaliada no discovery técnico.
- Reconciliações podem ser executadas de forma controlada sem tornar projeções assíncronas a fonte primária.
- Alertas podem escalar independentemente da confirmação financeira.

## Alternativa Recomendada
Registro canônico reconciliável com atualização transacional.

## Justificativa
É a alternativa que melhor atende o risco inaceitável de total incorreto, mantém consistência imediata para API e eventos e permite conter o MVP. Ela aceita conscientemente a perda de auditoria histórica e desacopla a entrega futura de WhatsApp/LLM sem remover alertas persistentes do núcleo.

## Decisões Pendentes
- Definir contrato de identidade idempotente compartilhada entre API e evento.
- Confirmar regras e contrato do futuro módulo de categorias.
- Definir semântica dos limiares configuráveis e prevenção de alertas repetidos.
- Definir moeda, precisão, fuso horário e fechamento mensal.
- Validar mecanismo de reconciliação do estado atual.

## Próximo Passo Recomendado
technical-discovery-production com foco em consistência transacional, idempotência entre API e evento, modelo reconciliável, processamento confiável de alertas e contrato futuro com agente LLM/WhatsApp.
