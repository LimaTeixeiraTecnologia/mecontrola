# Transcript do Brainstorming Decisório

## Contexto Inicial

- Objetivo informado: decidir a direção de um MVP robusto e production-ready para um módulo de orçamentos mensais por usuário.
- Hipótese inicial proposta: cada usuário possui no máximo um orçamento por mês, com um valor total distribuído entre categorias de orçamento.
- Exemplo informado: orçamento de R$ 10.000,00 para junho de 2026; uma categoria com 10% possui valor planejado de R$ 1.000,00.
- Comportamento esperado: acompanhar valor gasto acumulado, valor planejado por categoria, percentual utilizado e alertar quando o gasto ultrapassar o planejado.
- Restrição informada: a soma da distribuição percentual entre categorias não pode ultrapassar 100%; o gasto realizado pode ultrapassar o limite.
- Entradas desejadas: lançamentos via API e via evento, ambos capazes de acrescentar valores às categorias.
- Exemplos informados: abastecimento do carro associado a custos fixos; delivery associado a prazeres.
- Material de apoio: imagem de referência com renda mensal, resumo por categoria, metas percentuais e totais.
- Contestação inicial: API e evento como duas entradas equivalentes podem gerar dupla contabilização sem uma regra explícita de identidade e autoridade do lançamento. Também é necessário decidir se a categoria classifica automaticamente o gasto ou se o chamador informa a categoria de orçamento.

## Rodada 1 - Entendimento do Problema

### Perguntas

**P1. Qual problema principal o MVP deve resolver?**

- A. Controle preventivo: ajudar o usuário a decidir se ainda pode gastar antes de realizar novas despesas; exige saldo disponível e alertas acionáveis.
- B. Acompanhamento posterior: consolidar despesas já realizadas e mostrar desvios; prioriza precisão do acumulado e histórico auditável.
- C. Ambos com igual prioridade: combina decisão preventiva e acompanhamento posterior desde o MVP; amplia escopo, estados e critérios de aceite.

**P2. O que torna o MVP bem-sucedido para o usuário?**

- A. Confiabilidade financeira: nenhum lançamento válido é perdido ou contado duas vezes, mesmo chegando por API e evento; prioriza idempotência e rastreabilidade.
- B. Clareza do planejamento: o usuário cria o orçamento, distribui percentuais e entende rapidamente planejado, gasto e excedente; prioriza experiência e leitura.
- C. Ação sobre desvios: além de visualizar, o usuário recebe alertas úteis ao se aproximar ou ultrapassar limites; exige política de alertas desde o MVP.
- D. Os três critérios são obrigatórios: entrega um MVP mais completo, porém com maior custo e prazo.

**P3. Qual deve ser o impacto prático de ultrapassar o orçamento de uma categoria?**

- A. Apenas sinalizar no painel/API: menor complexidade, mas depende de o usuário consultar o sistema.
- B. Gerar alerta persistente e consultável: registra o desvio como informação operacional, sem bloquear despesas.
- C. Notificar ativamente o usuário: aumenta capacidade de reação, mas introduz canal de notificação, preferências e risco de ruído.
- D. Aplicar política configurável por usuário: oferece flexibilidade, mas expande significativamente o MVP.

**P4. Qual risco é mais importante evitar ao decidir agora?**

- A. Resultado financeiro incorreto por duplicidade, perda ou ordem de eventos; exige consistência, idempotência e reconciliação como prioridades.
- B. Modelo rígido que não acomode futuras categorias e regras de distribuição; exige preservar extensibilidade.
- C. MVP amplo demais que demora a entregar valor; exige limitar automações, alertas e personalizações iniciais.
- D. Baixa adoção por exigir configuração manual excessiva; exige defaults e fluxo simples de criação.

### Respostas

- P1: C. Controle preventivo e acompanhamento posterior possuem igual prioridade no MVP.
- P2: D. Confiabilidade financeira, clareza do planejamento e ação sobre desvios são critérios obrigatórios.
- P3: C. Ao ultrapassar o orçamento da categoria, o sistema deve notificar ativamente o usuário.
- P4: C. O risco prioritário é criar um MVP amplo demais que demore a entregar valor.

### Síntese da Rodada

- O MVP precisa servir tanto para orientar decisões antes do gasto quanto para acompanhar despesas realizadas.
- O produto só será considerado bem-sucedido se combinar precisão financeira, leitura clara do planejamento e alertas acionáveis.
- Exceder o planejado não bloqueia o lançamento, mas exige notificação ativa.
- Existe tensão material entre os critérios obrigatórios amplos e a prioridade de evitar excesso de escopo. A Rodada 2 deve impor limites explícitos ao MVP.

## Rodada 2 - Escopo e Restrições

### Perguntas

**P1. Como um lançamento deve ser associado a uma categoria de orçamento no MVP?**

- A. Categoria obrigatória informada na entrada: API e evento devem trazer a categoria; reduz automação e ambiguidade.
- B. Classificação automática pelo módulo de categorias: o orçamento recebe a categoria já resolvida por regras externas; depende da maturidade desse módulo.
- C. Classificação automática com correção manual: melhora experiência, mas exige estados de classificação, correção e possível recálculo.
- D. Permitir lançamento sem categoria: evita rejeição, mas exige fila de pendências e totais parcialmente classificados.

**P2. Quais operações financeiras entram no MVP?**

- A. Apenas acrescentar despesa imutável: correções usam novo lançamento compensatório; maximiza auditabilidade e limita escopo.
- B. Criar e estornar despesas: cobre erros comuns preservando histórico, com complexidade moderada.
- C. Criar, editar e excluir despesas: experiência direta, mas aumenta risco de inconsistência entre API, eventos e acumulados.
- D. Despesas e receitas: permite derivar renda/orçamento, mas amplia o módulo além do controle de gastos.

**P3. Qual é o limite do alerta ativo no MVP?**

- A. Um único canal já existente, somente ao ultrapassar 100% da categoria; menor escopo e menor risco de ruído.
- B. Um único canal já existente, ao atingir um limiar fixo e ao ultrapassar 100%; oferece prevenção com política simples.
- C. Múltiplos canais e limiares configuráveis; melhora personalização, mas cria subsistema de preferências e entrega.
- D. Apenas produzir evento de alerta para outro módulo notificar; mantém budgets focado, mas o valor ao usuário depende de integração externa.

**P4. Como alterações no orçamento durante o mês devem funcionar no MVP?**

- A. Orçamento e percentuais ficam imutáveis após ativação; simplifica histórico, mas reduz flexibilidade.
- B. Permitir alterações preservando apenas o estado atual; entrega flexibilidade com menor esforço, mas perde explicação histórica.
- C. Permitir alterações com versionamento/auditoria; preserva rastreabilidade, mas amplia modelo e consultas.
- D. Não permitir alterar o total, mas permitir redistribuir percentuais com auditoria; equilibra controle e flexibilidade com regras adicionais.

### Respostas

- P1: A. Toda entrada deve informar obrigatoriamente a categoria de orçamento.
- P2: C. O MVP deve permitir criar, editar e excluir despesas.
- P3: C. O MVP deve suportar múltiplos canais e limiares configuráveis.
- P4: A. O orçamento e seus percentuais ficam imutáveis após ativação.

### Síntese da Rodada

- Categoria obrigatória e orçamento imutável reduzem ambiguidade e complexidade do núcleo.
- Criar, editar e excluir despesas exige decidir se exclusão significa remoção física ou cancelamento auditável.
- Múltiplos canais e limiares configuráveis constituem uma capacidade própria de preferências e notificações. Colocá-la dentro de budgets aumentaria acoplamento e escopo.
- Para compatibilizar robustez com contenção de escopo, as alternativas devem separar a experiência externa das garantias internas e considerar delegar a entrega de notificações.

## Rodada 3 - Alternativas

### Alternativas Comparáveis

**Alternativa A - Acumulado direto e notificações dentro de budgets**

- API e eventos atualizam diretamente o valor gasto acumulado da categoria.
- Editar ou excluir uma despesa aplica diferenças diretamente ao acumulado.
- Budgets armazena preferências, avalia limiares e entrega notificações em múltiplos canais.
- É a direção mais próxima da leitura literal do pedido e pode parecer rápida inicialmente.
- Risco principal: dupla contabilização, perda de histórico, concorrência entre entradas e expansão de budgets para um subsistema de notificações.

**Alternativa B - Registro canônico auditável com projeção transacional**

- Toda entrada é normalizada em um registro canônico de despesa com identidade idempotente.
- Editar e excluir existem para o usuário, mas preservam histórico por revisões e cancelamentos auditáveis.
- Totais por categoria são atualizados de forma transacional e podem ser reconciliados a partir dos registros.
- Budgets avalia o desvio e publica uma intenção/evento de alerta; uma capacidade externa gerencia canais e entrega.
- Equilibra consistência imediata, auditabilidade e escopo, com dependência explícita de uma capacidade de notificações.

**Alternativa C - Ledger imutável com projeções assíncronas**

- Cada criação, edição ou exclusão vira um lançamento imutável; correções geram reversão e substituição.
- Totais e alertas são projeções reconstruíveis processadas assincronamente.
- Oferece forte rastreabilidade, tolerância a reprocessamento e boa escala.
- Introduz consistência eventual, maior esforço operacional e complexidade para explicar estados ao usuário.

**Alternativa D - Event sourcing completo do orçamento**

- Configuração, ativação, despesas, correções, totais e alertas são derivados integralmente de eventos.
- Permite reconstrução temporal completa e evolução sofisticada.
- Maximiza auditabilidade e flexibilidade futura.
- É a alternativa mais complexa e lenta, incompatível com o risco prioritário de excesso de escopo sem evidência de escala ou compliance que a justifique.

### Perguntas

**P1. Qual garantia deve prevalecer quando API e evento concorrerem ou repetirem o mesmo lançamento?**

- A. Consistência imediata e resposta definitiva: após confirmação, a consulta já deve refletir o valor correto; favorece a Alternativa B.
- B. Consistência eventual com processamento resiliente: pequenos atrasos são aceitáveis em troca de desacoplamento e escala; favorece a Alternativa C.
- C. Melhor esforço com correção posterior: aceita divergência temporária e reconciliação manual; mantém a Alternativa A viável, mas reduz confiabilidade.
- D. Reconstrução integral por eventos: exige event sourcing como premissa; favorece a Alternativa D e amplia o MVP.

**P2. Onde deve ficar a responsabilidade por múltiplos canais e preferências de notificação?**

- A. Em uma capacidade/módulo externo de notificações; budgets decide quando alertar e publica o alerta, preservando foco.
- B. Dentro de budgets; entrega tudo em um módulo, mas aumenta acoplamento, operação e escopo.
- C. No cliente consumidor da API; budgets apenas expõe estado, mas não atende notificação ativa confiável.
- D. Adiar múltiplos canais; entregar somente evento de alerta no MVP e evoluir após validar uso.

### Respostas

- P1: A. Após a confirmação, consultas devem refletir imediatamente o valor correto mesmo com concorrência ou repetição entre API e eventos.
- P2: B. Preferências e entrega de notificações em múltiplos canais devem ficar dentro de budgets.

### Síntese da Rodada

- A exigência de consistência imediata favorece atualização transacional e elimina projeções assíncronas como fonte principal de leitura do MVP.
- Manter notificações dentro de budgets contradiz a contenção de escopo e aumenta sua responsabilidade operacional, mas permanece como escolha explícita.
- A Alternativa B é a direção preliminar mais compatível, adaptada para incorporar preferências e entrega multicanal dentro do módulo.
- A Alternativa A continua comparável por simplicidade inicial, mas não satisfaz adequadamente confiabilidade e auditoria.
- As Alternativas C e D preservam robustez histórica, porém não atendem à preferência de consistência imediata com custo aceitável para o MVP.

## Rodada 4 - Trade-offs

### Avaliação Inicial

- Alternativa A possui menor tempo inicial, mas risco elevado de totais incorretos e histórico insuficiente após edições/exclusões.
- Alternativa B atende consistência imediata, idempotência, edição/exclusão auditável e reconciliação. Incorporar notificações multicanal reduz sua vantagem de escopo.
- Alternativa C melhora desacoplamento e escala, mas introduz atraso perceptível e operação assíncrona incompatíveis com a escolha de consistência imediata.
- Alternativa D oferece reconstrução completa, porém seu custo e complexidade configuram excesso de escopo sem evidência de necessidade.

### Perguntas

**P1. Como resolver a tensão entre edição/exclusão e confiabilidade financeira?**

- A. Expor editar/excluir, mas internamente preservar revisões e cancelamentos auditáveis; aumenta armazenamento e regras, porém mantém rastreabilidade.
- B. Editar/excluir fisicamente e manter apenas o estado atual; simplifica experiência e implementação, mas impede auditoria e reconciliação confiável.
- C. Restringir o MVP a criar e estornar, revendo a escolha anterior; reduz escopo e risco, mas altera a experiência desejada.

**P2. Qual custo operacional de notificações dentro de budgets é aceitável no MVP?**

- A. Implementar múltiplos canais diretamente no MVP, incluindo preferências, retries, falhas parciais e observabilidade; atende integralmente o escopo com maior prazo.
- B. Manter preferências e decisão de alerta em budgets, mas usar providers/adapters existentes para entrega; reduz reinvenção, porém mantém acoplamento operacional.
- C. Manter o contrato multicanal em budgets, mas ativar somente um canal no MVP; preserva direção futura e contém prazo, mas entrega parcialmente a escolha anterior.
- D. Rever a fronteira e publicar alertas para capacidade externa; menor escopo em budgets, mas contradiz a decisão anterior.

**P3. Qual comportamento deve ocorrer se a notificação falhar após a despesa ter sido confirmada?**

- A. Confirmar a despesa e persistir a notificação para retry até entrega ou expiração; preserva consistência financeira e exige processamento confiável.
- B. Falhar ou reverter a despesa; garante acoplamento forte com alerta, mas torna lançamento financeiro dependente de canais externos.
- C. Confirmar a despesa e registrar apenas log/métrica da falha; menor complexidade, mas não garante notificação ativa.

**P4. Qual risco é inaceitável no MVP?**

- A. Total financeiro incorreto ou não reconciliável, mesmo que isso aumente prazo e complexidade.
- B. Alerta não entregue, mesmo que isso torne lançamentos dependentes da infraestrutura de notificação.
- C. Entrega mais lenta, mesmo que seja necessária para cumprir todo o escopo escolhido.
- D. Escopo excessivo, aceitando reduzir notificações ou operações para entregar o núcleo confiável primeiro.

### Respostas

- P1: B. Editar e excluir fisicamente, mantendo somente o estado atual.
- P2: B. Budgets mantém preferências e decisão de alerta, usando providers/adapters existentes para entrega. Para o MVP, o canal desejado é WhatsApp por meio de um agente LLM que será implementado no futuro.
- P3: A. A despesa permanece confirmada e a notificação deve ser persistida para retry até entrega ou expiração.
- P4: A. Total financeiro incorreto ou não reconciliável é risco inaceitável.

### Síntese da Rodada

- O usuário aceita perder a trilha histórica das edições e exclusões, desde que o estado financeiro atual permaneça correto e reconciliável.
- Exclusão física reduz auditabilidade e capacidade de investigação, mas não impede necessariamente reconciliar o total atual a partir das despesas restantes.
- Falhas de notificação não podem afetar a confirmação da despesa; alertas precisam de persistência e retry independentes.
- O único canal alvo informado para o MVP é WhatsApp. A entrega depende de um agente LLM futuro, tornando necessário esclarecer se este MVP entrega mensagens ou apenas prepara alertas persistentes para integração.
- A Alternativa B permanece recomendada, ajustada para estado atual reconciliável em vez de histórico financeiro auditável completo.

## Rodada 4.1 - Clarificação de Riscos Materiais

### Perguntas

**P1. Qual nível de rastreabilidade é obrigatório para edições e exclusões físicas?**

- A. Nenhum histórico: manter apenas o estado atual e garantir que os totais possam ser recalculados a partir das despesas existentes; menor escopo, sem investigação retroativa.
- B. Auditoria mínima: apagar/alterar o dado financeiro, mas registrar metadados de quem, quando e operação realizada, sem preservar valores anteriores; custo moderado e investigação limitada.
- C. Preservar valores anteriores em trilha de auditoria separada; mantém exclusão lógica para a experiência, mas equivale funcionalmente à alternativa auditável rejeitada.

**P2. O que este MVP deve entregar em relação ao WhatsApp e ao agente LLM futuro?**

- A. Persistir alertas e expor contrato/provider para integração futura; o MVP é considerado completo sem enviar mensagem real.
- B. Integrar diretamente com WhatsApp sem depender do agente LLM; o MVP só é completo quando a mensagem real for entregue.
- C. Aguardar o agente LLM e incluir a integração com ele no MVP; prazo e viabilidade dependem de componente ainda inexistente.
- D. Criar um adapter temporário e substituí-lo pelo agente LLM depois; entrega mensagem real agora, mas assume retrabalho planejado.

### Respostas

- P1: A. Nenhum histórico de edições ou exclusões; manter somente o estado atual e garantir totais recalculáveis.
- P2: C. O MVP inclui integração com o agente LLM futuro e só é considerado completo com essa integração.

### Síntese da Rodada

- A confiabilidade exigida limita-se à correção e reconciliação do estado financeiro atual; não inclui auditoria histórica.
- Edições e exclusões físicas eliminam a capacidade de explicar valores anteriores ou investigar mudanças retroativamente.
- O agente LLM para WhatsApp é dependência bloqueante do MVP. Como ainda será implementado, prazo, custo e viabilidade da entrega ativa permanecem condicionados ao contrato e disponibilidade desse componente.
- A alternativa recomendada deixa de ser descrita como auditável e passa a ser **Registro canônico reconciliável com atualização transacional**.

## Rodada 5 - Seleção de Direção

### Síntese para Decisão

**Recomendação preliminar: Registro canônico reconciliável com atualização transacional**

- Cada despesa possui identidade canônica e idempotente, independentemente de entrar por API ou evento.
- Criação, edição e exclusão física atualizam o estado e os totais na mesma fronteira transacional.
- Consultas refletem imediatamente operações confirmadas.
- Totais atuais podem ser recalculados a partir das despesas existentes.
- Não há histórico de valores editados ou despesas excluídas.
- Budgets avalia limiares configuráveis, persiste alertas e executa retries sem comprometer a despesa.
- O envio ativo ocorre por WhatsApp através do agente LLM futuro, que é dependência bloqueante do MVP.

**Trade-offs aceitos até aqui**

- Simplicidade de edição/exclusão foi priorizada sobre auditoria histórica.
- Consistência imediata foi priorizada sobre desacoplamento e escala assíncrona.
- A responsabilidade de alertas permanece em budgets, ampliando seu escopo operacional.
- O MVP depende de um componente futuro, portanto sua entrega não pode ser estimada de forma confiável antes de definir esse contrato.

### Pergunta

**P1. Qual direção deve ser registrada como decisão final?**

- A. Confirmar a recomendação: registro canônico reconciliável com atualização transacional, sem histórico, e agente LLM como dependência bloqueante do MVP.
- B. Confirmar o núcleo recomendado, mas retirar a integração com agente LLM do critério de conclusão; budgets entrega alertas persistentes prontos para integração futura.
- C. Escolher acumulado direto e aceitar maior risco de inconsistência para reduzir prazo.
- D. Exigir nova rodada para revisar edição/exclusão, fronteira de alertas ou dependência do agente LLM.

### Resposta

- P1: B. Confirmar o núcleo recomendado, retirando a integração com agente LLM do critério de conclusão; budgets entrega alertas persistentes prontos para integração futura.

### Decisão Final

- Direção escolhida: **Registro canônico reconciliável com atualização transacional**.
- O MVP termina com alertas persistentes, configuráveis e sujeitos a retry, além de contrato/provider preparado para integração.
- O envio real por WhatsApp através do agente LLM fica fora do critério de conclusão do MVP.
- A entrega futura via WhatsApp permanece como evolução dependente do agente LLM.

## Decisões Registradas

- O MVP deve atender controle preventivo e acompanhamento posterior com igual prioridade.
- Confiabilidade financeira, clareza do planejamento e ação sobre desvios são critérios obrigatórios.
- O sistema deve notificar ativamente o usuário quando uma categoria ultrapassar o orçamento.
- Evitar um MVP amplo demais é o risco prioritário.
- Entradas devem informar obrigatoriamente a categoria de orçamento.
- O MVP deve expor criação, edição e exclusão de despesas.
- Alertas devem admitir múltiplos canais e limiares configuráveis.
- Orçamento e percentuais ficam imutáveis após ativação.
- Consultas devem refletir imediatamente lançamentos confirmados, mesmo sob concorrência ou repetição.
- Preferências e entrega multicanal de notificações devem ficar dentro de budgets.
- O usuário aceita editar e excluir despesas fisicamente, preservando apenas o estado atual.
- O estado financeiro atual deve permanecer correto e reconciliável; perda de auditabilidade histórica foi aceita preliminarmente.
- Falha de notificação não pode reverter nem impedir a despesa; o alerta deve ser persistido para retry.
- O canal alvo informado é WhatsApp por meio de agente LLM futuro.
- Não haverá histórico de edições ou exclusões; a reconciliação cobre apenas o estado atual.
- A integração real com o agente LLM/WhatsApp fica fora do critério de conclusão do MVP.
- Decisão final explícita: registro canônico reconciliável com atualização transacional, com alertas persistentes prontos para integração futura.
