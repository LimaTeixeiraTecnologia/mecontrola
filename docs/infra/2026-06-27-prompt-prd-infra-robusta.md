# Prompt Enriquecido para `$create-prd`

Data de referência: `2026-06-27`

## Prompt original

Eu quero criar/melhorar a infra, para deixar extremamente robusta, para 10 mil usuários ativos com uma média de 15 interações diárias. Eu quero que revise e melhore Docker, docker-compose, postgres, caddy. Estou utilizando VPS da Hostinger KVM2 com Ubuntu 24.04. Utilize documentações oficiais e mais atualizadas em 2026.

Quero a possibilidade de réplicas robustas da `cmd/server` e `cmd/worker`, com foco inegociável em robustez, confiabilidade, segurança e performance, sem desvios, sem flexibilidade, MVP robusto e production-ready.

Hoje temos 2 usuários ativos. Em dezembro teremos 10 mil usuários ativos. Quero uma análise sem desvios de uma infraestrutura que realmente aguente sem grandes mudanças. Uso em produção real, com clientes reais.

Quero a possibilidade real de ter 3 réplicas de `server` e 3 réplicas de `worker`.

## Prompt enriquecido pronto para uso no `$create-prd`

Crie um PRD em pt-BR para uma iniciativa de produto/plataforma chamada **Infraestrutura de Produção Robusta para Escala de 10 Mil Usuários Ativos até Dezembro de 2026**.

O objetivo deste PRD não é implementar nem detalhar código, e sim definir com clareza o problema, o escopo, as restrições, os requisitos funcionais e os critérios de sucesso de uma evolução de infraestrutura real de produção para o sistema `mecontrola`, considerando clientes reais e operação real.

Trate este pedido como **prescritivo, fechado e sem flexibilidade**. Não desvie do objetivo central, não dilua requisitos críticos, não transforme limitação séria em detalhe secundário e não proponha múltiplas estratégias por conveniência. O resultado precisa ser pronto para uso em um fluxo real de PRD, com posicionamento técnico claro, auditável e objetivo.

Antes de escrever o PRD, siga obrigatoriamente esta sequência:

1. analisar documentações oficiais e atualizadas dos componentes relevantes
2. confrontar essas documentações com o estado real do repositório e da infraestrutura atual
3. fazer projeções realistas de capacidade para `10.000` usuários ativos
4. consolidar o PRD com base nessa análise

Essas etapas são mandatórias. Não pule a análise documental oficial. Não presuma capacidade sem justificativa. Se faltar evidência, registre explicitamente a lacuna como risco, restrição, premissa ou questão em aberto.

Use o contexto abaixo como obrigatório:

- Projeto atual: monolito modular em Go.
- Estado atual relevante do repositório:
  - `deployment/compose/compose.yml`
  - `deployment/compose/compose.prod.yml`
  - `deployment/caddy/Caddyfile`
  - `deployment/postgres/postgresql.conf`
  - `deployment/pgbouncer/pgbouncer.ini`
  - `deployment/pgbackrest/pgbackrest.conf`
  - `deployment/docker/Dockerfile`
  - `deployment/docker/Dockerfile.postgres`
  - `deployment/docker/Dockerfile.caddy`
  - `deployment/telemetry/`
  - `deployment/runbooks/`
- Stack atual identificada no repositório:
  - aplicação Go em imagem distroless
  - PostgreSQL 16
  - pgBouncer
  - pgBackRest
  - Caddy 2
  - Docker Compose
  - stack de observabilidade com Prometheus, Grafana, Loki, Tempo e OpenTelemetry
- Ambiente alvo atual:
  - VPS Hostinger KVM2
  - Ubuntu 24.04
  - operação em produção real
  - hoje com 2 usuários ativos
  - meta de `10.000` usuários ativos até dezembro de 2026
  - média esperada de `15` interações por usuário ativo por dia
- Capacidade desejada obrigatória:
  - possibilidade real de operar `3` réplicas de `cmd/server`
  - possibilidade real de operar `3` réplicas de `cmd/worker`
- Diretriz mandatória:
  - foco inegociável em robustez, confiabilidade, segurança, performance e operabilidade
  - sem propor solução frágil, provisória ou apenas “boa o suficiente”
  - sem tratar como MVP descartável
  - sem esconder conflitos entre meta de robustez e limitações reais de uma VPS única
  - sem vender alta disponibilidade inexistente

Baseie o contexto e as restrições do PRD nas documentações oficiais e atuais consultáveis em 2026, priorizando estas fontes:

- PostgreSQL: `https://www.postgresql.org/docs/`
- Docker: `https://docs.docker.com/`
- Docker Compose: `https://docs.docker.com/compose/`
- Caddy: `https://caddyserver.com/docs/`
- pgBouncer: `https://www.pgbouncer.org/`
- pgBackRest: `https://pgbackrest.org/`
- Grafana: `https://grafana.com/docs/`
- Prometheus: `https://prometheus.io/docs/`
- OpenTelemetry: `https://opentelemetry.io/docs/`
- Hostinger KVM2 / benchmark de referência: `https://www.vpsbenchmarks.com/hosters/hostinger/plans/kvm-2`

Considere explicitamente que, em `2026-06-27`, a documentação corrente do PostgreSQL aponta para a série `current = 18`, e que o PRD deve registrar quando o repositório ainda estiver em componentes defasados frente à documentação corrente, sem assumir upgrade automático como requisito obrigatório sem justificativa.

A análise documental é obrigatória. O PRD deve deixar explícito:

- quais documentos oficiais foram considerados
- quais conclusões vieram dessas fontes
- onde o estado atual converge com a prática recomendada
- onde o estado atual diverge da prática recomendada
- quais pontos dependem de documentação oficial e quais dependem de premissas operacionais

Quero que o PRD seja rigoroso e objetivo sobre:

- qual problema de negócio e de operação esta iniciativa resolve
- quais riscos reais existem ao manter a infraestrutura atual para a meta de dezembro de 2026
- qual resultado de produto/plataforma precisa existir para que o sistema escale sem grandes mudanças estruturais posteriores
- quais capacidades mínimas de produção são obrigatórias para considerar a iniciativa concluída
- quais limitações são aceitáveis e quais são inegociáveis
- quais dependências, riscos, premissas e restrições precisam ficar explícitos para evitar falsa sensação de robustez

O PRD deve tratar como escopo incluído, no mínimo:

- arquitetura alvo de execução para `server`, `worker`, `postgres`, `pgbouncer`, `pgbackrest`, `caddy`, observabilidade e rotinas de `migrate`
- requisitos de disponibilidade e comportamento esperado em reinício, crash, deploy, rollback e degradação parcial
- requisitos para escalar de forma realista para `3` réplicas de `server` e `3` réplicas de `worker`
- requisitos de balanceamento, health checks, startup/shutdown ordenado, readiness e liveness
- requisitos de isolamento de rede, exposição de portas, TLS, headers de segurança, segredos e hardening
- requisitos de persistência, pooling, tuning, backups, restore, PITR e recuperação de desastre para PostgreSQL
- requisitos de observabilidade operacional mínimos para produção: logs, métricas, traces, alertas e sinais de saturação
- requisitos de capacidade e performance compatíveis com `10.000` usuários ativos e `15` interações diárias por usuário
- requisitos derivados de projeção de carga realista, cobrindo pico, concorrência provável, burst, reprocessamento e folga operacional
- requisitos de operação segura em VPS única agora, deixando explícito o que é robusto de verdade e o que continuaria sendo risco por ser single-node
- requisitos para deploy repetível, previsível, auditável e reversível
- requisitos para evitar pontos únicos de falha lógicos dentro do possível no contexto informado

O PRD deve tratar como escopo excluído, salvo se for indispensável para a viabilidade do objetivo:

- reescrita do sistema em microserviços
- mudança de linguagem
- troca de provedor cloud sem evidência objetiva de necessidade
- implementação detalhada de pipelines CI/CD
- passo a passo de comandos de implantação
- mudanças de produto não relacionadas à robustez da infraestrutura

Quero que o documento seja honesto e sem maquiagem técnica. Se uma VPS Hostinger KVM2 única não oferecer alta disponibilidade verdadeira em algum aspecto, o PRD deve registrar isso de forma explícita como restrição, risco residual, dependência externa ou pré-condição de fase posterior. Não quero “HA de papel”.

O PRD deve fazer, de forma mandatória, uma projeção realista de capacidade para `10.000` usuários ativos com média de `15` interações por dia, incluindo no mínimo:

- volume diário estimado de interações
- distribuição provável entre horas de menor uso e janelas de pico
- implicações práticas para réplicas de `server` e `worker`
- pressão esperada sobre banco, pool de conexões, CPU, memória, disco e rede
- margem mínima de segurança operacional recomendada

Não use projeção ingênua do tipo “`10.000 x 15 / 24`” como conclusão suficiente. O PRD deve explicitar que capacidade de produção precisa considerar burst, concorrência, jobs assíncronos, reprocessamento, deploys, falhas transitórias e degradação parcial.

Defina o usuário/ator principal desta iniciativa como:

- time fundador/engenharia responsável por operar a plataforma em produção

Defina o problema central assim:

- a infraestrutura atual precisa ser revisada e fortalecida agora para suportar a meta de escala de dezembro de 2026 sem grandes refações, com operação confiável e segura em produção real

Defina critérios de sucesso mensuráveis no PRD, cobrindo no mínimo:

- capacidade operacional planejada para `10.000` usuários ativos
- operação prevista com `3` réplicas de `server` e `3` réplicas de `worker`
- definição clara de SLOs ou metas equivalentes para disponibilidade, erro, latência e recuperação
- existência de estratégia de backup, restore e recuperação validável
- existência de critérios de rollback e continuidade operacional
- definição explícita do que precisa estar pronto antes de considerar a infraestrutura “production-ready”

Ao redigir o PRD:

- mantenha foco em produto/plataforma, não em implementação detalhada
- não gere tech spec
- não gere tasks
- não proponha alternativas demais; priorize uma direção principal robusta
- não ofereça “depende” como fuga; quando houver incerteza, registre a condição exata e a decisão recomendada
- não trate requisito crítico como `nice to have`
- não flexibilize robustez, segurança, backup, restore, observabilidade ou rollback
- explicite trade-offs apenas quando forem inevitáveis
- numere todos os requisitos funcionais
- crie a seção `Base Documental Oficial Consultada`
- crie a seção `Projeção Realista para 10.000 Usuários Ativos`
- crie a seção `Drift entre Estado Atual e Prática Recomendada`
- inclua a seção `Suposições e Questões em Aberto`
- se houver conflito entre meta de robustez e limite físico/arquitetural do contexto atual, registre isso claramente

Formato de saída obrigatório:

- resumo executivo objetivo
- contexto atual validado
- base documental oficial consultada
- projeção realista para `10.000` usuários ativos
- problema
- objetivos
- escopo incluído
- escopo excluído
- requisitos funcionais numerados
- requisitos não funcionais
- riscos, restrições e dependências
- drift entre estado atual e prática recomendada
- critérios de sucesso mensuráveis
- suposições e questões em aberto

Se faltar contexto para fechar o PRD com qualidade, faça no máximo duas rodadas de esclarecimento, priorizando perguntas sobre:

- orçamento aceitável para infraestrutura
- tolerância real a indisponibilidade
- necessidade ou não de multi-node no horizonte desta fase
- janela de deploy aceitável
- RPO/RTO desejados

Se a análise concluir que a meta declarada não cabe com robustez real em VPS única sem riscos residuais importantes, isso deve aparecer como conclusão explícita no PRD. Não omita esse ponto para parecer mais otimista.

## Justificativas das adições

- Estruturei o pedido para caber exatamente na skill `$create-prd`, que precisa de problema, ator, escopo, restrições e critérios de sucesso.
- Amarrei o prompt ao estado real do repositório para evitar um PRD genérico desconectado de `deployment/`.
- Fixei as fontes oficiais e a data de referência para reduzir drift temporal.
- Explicitei a meta de capacidade em números operacionais para tornar o output auditável.
- Forcei o PRD a registrar limites reais de VPS única para evitar conclusões falsas sobre alta disponibilidade.
- Separei escopo incluído e excluído para manter o foco no que interessa nesta iniciativa.
- Tornei mandatória a sequência análise documental oficial -> confronto com estado atual -> projeção realista -> redação do PRD.
- Endureci o prompt para bloquear respostas evasivas, genéricas ou flexíveis em pontos críticos de produção.
