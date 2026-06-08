# Scorecard de Alternativas

Escala: 1 = pior ou mais oneroso; 5 = melhor ou menos oneroso no contexto da decisão.

| Alternativa | Complexidade | Tempo de entrega | Custo | Escalabilidade | Segurança | Confiabilidade | Observabilidade | Manutenibilidade | Risco operacional | Total | Observação |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| Alternativa 1 - Acumulado direto e notificações dentro de budgets | 4 | 4 | 4 | 3 | 3 | 1 | 2 | 2 | 1 | 24 | Rápida inicialmente, mas edição/exclusão, concorrência e dupla entrada tornam totais e alertas difíceis de reconciliar. |
| Alternativa 2 - Registro canônico reconciliável com atualização transacional | 3 | 3 | 3 | 4 | 3 | 5 | 3 | 4 | 3 | 31 | Melhor equilíbrio para consistência imediata e reconciliação do estado atual; sem histórico e com integração WhatsApp/LLM fora do MVP. |
| Alternativa 3 - Ledger imutável com projeções assíncronas | 2 | 2 | 2 | 5 | 4 | 5 | 5 | 3 | 3 | 31 | Robusta e escalável, mas consistência eventual e operação assíncrona conflitam com a decisão da Rodada 3. |
| Alternativa 4 - Event sourcing completo do orçamento | 1 | 1 | 1 | 5 | 4 | 5 | 5 | 2 | 2 | 26 | Auditabilidade máxima, porém complexidade, prazo e custo caracterizam excesso de escopo para o MVP. |

## Leitura do Resultado
- Alternativa mais equilibrada: B - Registro canônico reconciliável com atualização transacional.
- Alternativa mais rápida: A - Acumulado direto e notificações dentro de budgets.
- Alternativa mais segura: C e D possuem maior rastreabilidade; B possui menor risco de execução no núcleo, mas perde segurança investigativa sem histórico.
- Alternativa mais barata: A no curto prazo, com risco elevado de custo corretivo posterior.
- Alternativa com maior risco operacional: A, devido à baixa reconciliabilidade sob edição, exclusão e entradas concorrentes.

## Análise Qualitativa

- Viabilidade técnica: B é viável no monólito modular atual com transação e identidade idempotente; integração com o agente LLM ainda não pode ser validada.
- Viabilidade operacional: A é frágil para suporte e correção; B permite reconciliação; C e D exigem operação de projeções e reprocessamentos.
- Viabilidade financeira: A custa menos inicialmente, mas aumenta risco de retrabalho; B exige investimento moderado e previsível; C e D possuem custo desproporcional ao MVP.
- Segurança: todas exigem isolamento por usuário e autorização; a ausência de histórico em B impede investigação retroativa de alterações indevidas.
- Escalabilidade: C e D escalam melhor em processamento, mas B atende o estágio presumido sem introduzir consistência eventual.
- Observabilidade: B exige correlação entre entrada, alteração financeira e notificação; C e D oferecem rastreabilidade natural com maior custo.
- Complexidade organizacional: notificações dentro de budgets ampliam ownership e plantão operacional em todas as alternativas.
- Dependências externas: múltiplos canais dependem de providers e disponibilidade externa; essa falha não deve corromper lançamentos financeiros.
- Tempo de implementação: A é mais rápida; B é moderada; C e D são lentas para um MVP.
- Capacidade da equipe: não informada; a ausência dessa evidência aumenta o risco de assumir operação assíncrona sofisticada.
