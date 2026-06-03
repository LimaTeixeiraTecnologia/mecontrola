# Prompt: Brainstorming de Arquitetura Event-Driven com Outbox

## Status
- **Original:** Recebido via CLI.
- **Enriquecido por:** Gemini CLI (prompt-enricher skill).
- **Objetivo:** Definir a fundação técnica para mensageria interna resiliente.

---

## Prompt Original

> Usar a decision-brainstorming skill para definir a melhor forma de ter event-driven na primeira entrega, com dispatcher de evento event driver, mas com outbox, sem mensageria e cronjob no mesmo cmd/worker separando por goroutine e limpeza a cada 90 dias dos eventos, utilizar https://github.com/robfig/cron na versão mais recenete e estável, com colunas aplicaveis para qualquer evento, dead-letter, jobs em varios instancias.

---

## Prompt Enriquecido (Para uso com a skill decision-brainstorming)

### Contexto e Objetivo
O objetivo deste brainstorming é desenhar a arquitetura de **Mensageria Interna Reativa** para um monolito em Go, utilizando o padrão **Transactional Outbox**. Esta solução deve garantir a entrega de eventos (at-least-once) sem depender de infraestrutura externa de mensageria (como RabbitMQ ou Kafka) na primeira entrega.

### Restrições e Requisitos Técnicos
1.  **Linguagem & Runtime:** Go (monolito).
2.  **Dispatcher & Persistência:**
    *   Implementar um dispatcher que consome uma tabela de `outbox`.
    *   A tabela de outbox deve ser genérica o suficiente para suportar qualquer tipo de evento (utilizando colunas de metadados e payload em JSON/JSONB).
3.  **Processamento Assíncrono:**
    *   O processador de eventos deve rodar no mesmo processo/binário (`cmd/worker` ou similar), mas em uma **goroutine separada** do ciclo de vida principal da aplicação.
    *   Utilizar `github.com/robfig/cron` (v3+) para agendamento de tarefas de manutenção e polling, se necessário.
4.  **Resiliência e Operação:**
    *   **Dead-Letter Queue (DLQ):** Mecanismo para isolar eventos que falharam definitivamente após N tentativas.
    *   **Retenção:** Processo automático de limpeza (housekeeping) para remover eventos processados/históricos com mais de 90 dias.
    *   **Concorrência Distribuída:** A solução deve estar preparada para rodar em **múltiplas instâncias** da aplicação, evitando processamento duplicado ou race conditions no consumo da tabela de outbox (sugere-se o uso de locks otimistas ou `SELECT ... FOR UPDATE SKIP LOCKED` se o banco suportar).
5.  **Simplicidade:** Evitar dependências pesadas. O foco é resolver com o banco de dados relacional existente e primitivas do Go.

### Instruções para a Skill decision-brainstorming
Ao executar este brainstorming, siga rigorosamente as etapas:

1.  **Análise de Alternativas (MCP):** Apresente pelo menos duas abordagens para o dispatcher (ex: Polling via Cron vs. Database Triggers/Listen-Notify vs. Transactional Listeners).
2.  **Mapeamento de Riscos:** Avalie o impacto de contenção no banco de dados, crescimento da tabela de outbox e latência de entrega.
3.  **Definição de Schema:** Proponha a estrutura mínima da tabela `outbox` (id, event_type, payload, status, attempts, last_attempt, created_at, processed_at).
4.  **Estratégia de Bloqueio:** Defina como as múltiplas instâncias vão coordenar o trabalho sem broker externo.
5.  **Critérios de Sucesso:** A solução deve garantir que um evento nunca seja perdido se a transação de negócio for confirmada.

### Formato de Saída Esperado
*   Relatório em Markdown (PT-BR).
*   Seção clara de "Decisão Preliminar".
*   Scorecard de comparação entre as alternativas levantadas.
*   Lista de próximos passos para a especificação técnica (techspec).

---

## Justificativa das Adições

*   **Contexto de Concorrência:** Adicionei a necessidade de coordenação via banco de dados (locks), pois "várias instâncias" sem mensageria externa exige cuidado extremo com atomicidade.
*   **Versionamento:** Fixei o uso do `cron` na v3+, que é a versão estável atual do `robfig/cron`.
*   **Protocolo de Brainstorming:** Estruturei o prompt para forçar o uso do protocolo de múltipla escolha (MCP) e análise de riscos, garantindo que o resultado não seja apenas "uma ideia", mas uma decisão técnica fundamentada.
*   **Schema Genérico:** Explicitei a necessidade de campos de controle de estado (attempts, status) essenciais para o funcionamento do Outbox e DLQ.
