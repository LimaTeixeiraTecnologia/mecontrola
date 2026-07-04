# 2026-07-03 — Prompt enriquecido para diagnóstico de capacidade do PostgreSQL com backup em S3

## Prompt original

> Eu quero analisar se via ssh root@187.77.45.48 e deployment a form que o banco de dados postgres está construido, com backup no s3, aguenta 10k usuários ativos? 15k usuários ativos? hoje temos 0 usuários, seja extretamente criterioso e sem achar, sem desvios me de um diagnostico completo e efetivo e de forma efetiva, robusto, economico, eficiente, 0 gaps, 0 falso positivo, 0 ressalvas, 0 lacunas.

## Ambiguidades eliminadas

- `aguenta 10k usuarios ativos / 15k usuarios ativos` nao pode ser tratado como conceito unico. Para evitar falso positivo, avalie obrigatoriamente dois modelos de carga por alvo:
  - `10k ativos/dia` e `15k ativos/dia`
  - `10k simultaneos em pico` e `15k simultaneos em pico`
  Nao misture os vereditos.
- `0 gaps / 0 lacunas / 0 ressalvas` nao pode ser premissa. So pode ser conclusao se houver prova objetiva. Na ausencia de prova, o veredito obrigatorio e `nao comprovado`.
- `aguenta` foi convertido em gates objetivos de:
  - capacidade de CPU/memoria/IO/storage
  - conexoes e pooling
  - throughput e concorrencia
  - backup/PITR/restore
  - operabilidade e observabilidade
  - custo minimo para suportar cada alvo
- `via ssh root@187.77.45.48 e deployment` foi convertido em obrigacao de cruzar:
  - evidencias reais do host via SSH read-only
  - manifests e runbooks em `deployment/`
  - bootstrap real da aplicacao a partir de `cmd/server/server.go` e `cmd/worker/worker.go`

## Prompt enriquecido — versão pronta para uso

```text
Atue como um especialista sênior em PostgreSQL 16, pgBouncer, pgBackRest, AWS S3, Docker Swarm single-node, capacity planning e auditoria operacional orientada a evidências.

Sua tarefa é executar uma análise técnica exaustiva, read-only e sem desvio para determinar se a forma atual como o PostgreSQL está construído e operado em produção, incluindo pooling e backup/PITR em S3, suporta com segurança e eficiência os alvos de 10k e 15k usuários ativos.

O objetivo NÃO é implementar nada. O objetivo é produzir um diagnóstico completo, auditável e sem falso positivo, baseado no host real via SSH, no diretório `deployment/` e no codebase atual.

Mandatos inegociáveis:
1. Não implemente nada.
2. Não altere arquivos.
3. Não rode comandos destrutivos.
4. Não reinicie serviços.
5. Não faça tuning.
6. Não faça deploy.
7. Não execute migrações.
8. Não crie backup novo em produção.
9. Não faça restore em produção.
10. Não invente contexto ausente.
11. Não use benchmark de blog, fórum, vídeo ou achismo.
12. Toda afirmação deve apontar evidência concreta do host, do repositório e, quando aplicável, documentação oficial do componente.
13. Se algo não puder ser provado, classifique obrigatoriamente como `nao comprovado`, `gap`, `lacuna de observabilidade`, `dado ausente`, `risco residual` ou `bloqueio objetivo`.
14. `0 gaps`, `0 lacunas`, `0 ressalvas` e `aguenta` só podem ser declarados se todos os gates obrigatórios forem aprovados com prova objetiva.

Contrato obrigatório de leitura e fonte da verdade:
1. Leia `AGENTS.md` antes de qualquer conclusão.
2. Assuma o working tree atual como fonte da verdade.
3. Para entender a pressão real da aplicação sobre o banco, parta obrigatoriamente de `cmd/server/server.go` e `cmd/worker/worker.go`.
4. É proibido usar `internal/platform/runtime` como ponto de partida central de análise.
5. Cruze sempre o que o host realmente roda com o que `deployment/` declara. Se divergirem, trate isso como drift real.

Acesso e contexto inicial já conhecidos, mas que ainda assim devem ser reconfirmados:
- SSH de produção: `ssh root@187.77.45.48`
- Repositório na VPS: `/opt/mecontrola`
- Repositório local: `mecontrola`
- Stack principal: Go `1.26.4`
- Produção declarada como Docker Swarm single-node
- Arquivo canônico de produção: `deployment/compose/compose.swarm.yml`
- Topologia declarada de banco:
  - `postgres`
  - `pgbouncer`
  - `postgres-exporter`
  - `pg-tunnel`
- PostgreSQL configurado com:
  - `max_connections = 100`
  - `shared_buffers = 512MB`
  - `effective_cache_size = 1536MB`
  - `work_mem = 16MB`
  - `wal_level = replica`
  - `archive_mode = on`
  - `archive_command = pgbackrest --stanza=mecontrola archive-push %p`
  - `archive_timeout = 600`
- pgBouncer configurado com:
  - `pool_mode = transaction`
  - `max_client_conn = 300`
  - `default_pool_size = 20`
  - `reserve_pool_size = 5`
  - `max_db_connections = 30`
- pgBackRest configurado com:
  - repositório S3
  - retenção full `4`
  - retenção diff `7`
  - `archive-async = y`
  - `archive-push-queue-max = 4GiB`
- Limites declarados no Swarm para o PostgreSQL:
  - `cpus: 1.0`
  - `memory: 2G`
- Estado atual de negócio informado: `0 usuarios ativos`

Fontes obrigatórias do repositório:
- `AGENTS.md`
- `go.mod`
- `README.md` com foco nas seções de acesso remoto, Swarm, banco, backup, restore e observabilidade
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `deployment/compose/compose.swarm.yml`
- `deployment/postgres/postgresql.conf`
- `deployment/pgbouncer/pgbouncer.ini`
- `deployment/pgbackrest/pgbackrest.conf`
- `deployment/pgbackrest/crontab.txt`
- `deployment/runbooks/deploy.md`
- `deployment/runbooks/restore-pitr.md`
- `deployment/runbooks/restore-vps.md`
- `deployment/monitoring/`
- `docs/runs/2026-07-03-relatorio-analise-infra-hostinger-kvm2-10k.md`
- `docs/runs/2026-07-03-evidencia-restore.md`
- quaisquer outros arquivos estritamente necessários para fechar prova sem especulação

Fontes oficiais mínimas obrigatórias:
- Documentação oficial do PostgreSQL 16
- Documentação oficial do pgBouncer
- Documentação oficial do pgBackRest
- Documentação oficial do Docker Swarm / docker stack / healthchecks / restart policy
- Documentação oficial da AWS S3 relevante para backup e durabilidade

Regras obrigatórias de execução no host:
1. Toda inspeção no host deve ser read-only.
2. É permitido inspecionar:
   - SO, CPU, memória, disco, filesystem, IOPS observável, carga atual
   - containers/services/tasks do Swarm
   - configs montadas
   - variáveis não sensíveis
   - logs somente para evidência
   - métricas expostas localmente
   - consultas SQL read-only
   - `pg_settings`, `pg_stat_*`, `pg_database_size`, `pg_isready`
   - status do pgBackRest, inventário de backups e evidência de WAL archive
3. É proibido:
   - `docker stack deploy`
   - `docker service update`
   - `docker service scale`
   - `systemctl restart`
   - `ALTER SYSTEM`
   - `VACUUM FULL`
   - `REINDEX`
   - `pgbench` em produção
   - restore real na produção
   - qualquer comando que gere carga material no host

Definição operacional obrigatória dos alvos:
Você deve avaliar separadamente e nunca misturar:
1. `10k ativos/dia`
2. `15k ativos/dia`
3. `10k simultaneos em pico`
4. `15k simultaneos em pico`

Se o repositório e o host não fornecerem base objetiva suficiente para uma dessas leituras, marque explicitamente `nao comprovado`.

Escopo técnico obrigatório da análise:

Fase 1 — Confirmar a topologia real em produção
1. Confirmar no host se o banco em produção roda exatamente como declarado no repositório.
2. Confirmar imagem, versão do PostgreSQL, versão do pgBouncer, volumes, networks, portas expostas e estratégia de healthcheck.
3. Confirmar como a aplicação chega ao banco:
   - `server` usa `pgbouncer` ou `postgres` direto
   - `worker` usa `pgbouncer` ou `postgres` direto
   - `migrate` usa `pgbouncer` ou `postgres` direto
4. Confirmar se existe desvio entre `compose.swarm.yml`, runbooks e o estado do host.
5. Produzir um mapa real do caminho de conexão:
   `app -> pgbouncer -> postgres -> volume local -> pgbackrest -> S3`

Fase 2 — Provar o orçamento real de capacidade do banco
1. Medir e documentar no host:
   - CPU total do host
   - memória total do host
   - espaço total e livre
   - uso atual do host
   - pressão de IO e load atual
2. Medir e documentar no serviço `postgres`:
   - limites e reservations efetivos
   - consumo atual
   - tamanho atual do banco
   - crescimento observado ou inferível
   - quantidade de conexões abertas
   - conexões em uso, idle e waiting
   - maiores tabelas
   - maiores índices
   - autovacuum
   - checkpoints
   - WAL gerado
3. Medir e documentar no `pgbouncer`:
   - pools
   - clients ativos
   - waiters
   - saturação potencial de `default_pool_size`, `reserve_pool_size` e `max_db_connections`
4. Verificar se a configuração atual de `max_connections = 100` no Postgres e `max_db_connections = 30` no pgBouncer é coerente com:
   - número de réplicas da aplicação
   - padrão de concorrência da aplicação
   - envelopes de 10k e 15k

Fase 3 — Provar a robustez do backup e recuperação
1. Confirmar se o backup em S3 está apenas configurado ou realmente operacional.
2. Confirmar:
   - último backup full
   - último diff
   - último incr
   - integridade reportada pelo pgBackRest
   - existência de archive WAL contínuo
   - backlog de archive queue
   - retenção efetiva
3. Diferenciar claramente:
   - capacidade de atender tráfego
   - capacidade de recuperar o banco após incidente
4. Não confundir `tem backup configurado` com `restore comprovado`.
5. Verificar se há evidência real de restore/PITR executado ou se existe apenas runbook/projeção.
6. Se o restore/RTO não tiver prova real, isso deve bloquear qualquer conclusão forte de `production-ready/proof`.

Fase 4 — Traduzir o codebase em pressão real sobre o banco
1. Partindo de `cmd/server/server.go` e `cmd/worker/worker.go`, identificar:
   - entrypoints que batem em banco
   - caminhos síncronos críticos
   - jobs/workers/eventos que geram escrita
   - risco de burst
   - uso de filas/outbox
   - superfícies com maior chance de contenção transacional
2. Inspecionar o codebase para descobrir:
   - quantos processos da aplicação dependem do banco
   - se há fan-out ou concorrência paralela relevante
   - se existem operações com tendência a lock, scan amplo ou contenção
3. Não assuma throughput do banco sem derivar a pressão da aplicação real.

Fase 5 — Diagnóstico de capacidade por alvo
Para cada alvo (`10k ativos/dia`, `15k ativos/dia`, `10k simultaneos em pico`, `15k simultaneos em pico`), analisar obrigatoriamente:
1. CPU do host
2. memória do host
3. limites do container postgres
4. eficiência do pgBouncer
5. conexões simultâneas disponíveis
6. margem de WAL/checkpoint/autovacuum
7. crescimento de disco
8. retenção de backup
9. janela de restore
10. SPOF do single-node
11. risco operacional do banco dividir host com app, observabilidade e proxy

Para cada alvo, você deve responder apenas uma classificação:
- `atende hoje`
- `atende com ajustes obrigatorios`
- `nao atende`
- `nao comprovado`

Fase 6 — Menor caminho econômico e seguro
1. Se não atender ou não estiver comprovado, diga qual é o menor conjunto de mudanças para suportar cada alvo.
2. Não proponha mudanças caras sem provar necessidade.
3. Não proponha economia que destrua recovery, observabilidade ou margem operacional.
4. Diferencie rigorosamente:
   - ajuste obrigatório
   - ajuste recomendado
   - ajuste desnecessário

Formato de saída obrigatório em Markdown:

# Diagnóstico de Capacidade do PostgreSQL

## 1. Escopo e método
- objetivo
- ambiente analisado
- comandos read-only executados
- arquivos do repositório analisados
- documentações oficiais consultadas
- regras de decisão usadas

## 2. Topologia real comprovada
Tabela obrigatória com colunas:
`componente | funcao | onde roda | como se conecta | persistencia | limite declarado | evidencia no host | evidencia no repo`

## 3. Prova do host e do orçamento de recursos
Tabela obrigatória com colunas:
`recurso | capacidade observada no host | consumo atual | limite/reservation declarados | margem | status`

## 4. Prova da configuracao do PostgreSQL
Tabela obrigatória com colunas:
`parametro | valor atual | origem | impacto | adequado?`

## 5. Prova da configuracao do pgBouncer
Tabela obrigatória com colunas:
`parametro | valor atual | origem | impacto | adequado?`

## 6. Prova do backup, PITR e restore
Tabela obrigatória com colunas:
`item | estado | evidencia | impacto operacional | veredito`

## 7. Pressao real da aplicacao sobre o banco
- caminhos do `server`
- caminhos do `worker`
- principais escritores
- principais leitores
- risco de burst
- risco de lock/contenção
- risco de saturação de pool

## 8. Analise por alvo
### 8.1 10k ativos/dia
### 8.2 15k ativos/dia
### 8.3 10k simultaneos em pico
### 8.4 15k simultaneos em pico

Para cada alvo, preencher obrigatoriamente:
- o que esta comprovado
- o que nao esta comprovado
- gargalo primario
- gargalo secundario
- primeiro componente a saturar
- impacto do backup/restore nesse alvo
- classificacao final: `atende hoje`, `atende com ajustes obrigatorios`, `nao atende` ou `nao comprovado`

## 9. Gaps, lacunas e bloqueios objetivos
Tabela obrigatória com colunas:
`item | categoria | evidencia | impacto | severidade | bloqueia qual alvo | acao obrigatoria`

## 10. Menor plano de evolucao seguro
Tabela obrigatória com colunas:
`alvo | mudanca minima | motivo tecnico | custo/impacto | risco mitigado | prova exigida`

## 11. Veredito final fechado
Responder explicitamente:
- `Hoje, o PostgreSQL atual com a topologia atual suporta 10k ativos/dia?`
- `Hoje, o PostgreSQL atual com a topologia atual suporta 15k ativos/dia?`
- `Hoje, o PostgreSQL atual com a topologia atual suporta 10k simultaneos em pico?`
- `Hoje, o PostgreSQL atual com a topologia atual suporta 15k simultaneos em pico?`
- `O backup em S3 esta apenas configurado ou esta operacionalmente comprovado?`
- `O restore/PITR esta comprovado ou apenas documentado/projetado?`
- `Qual e o primeiro bloqueio tecnico real para 10k?`
- `Qual e o primeiro bloqueio tecnico real para 15k?`
- `Qual e o menor caminho economico e seguro para fechar os gaps?`
- `Foi possivel fechar 0 gaps, 0 lacunas e 0 ressalvas com prova objetiva?`

Regras finais da resposta:
1. Nada de resposta genérica.
2. Nada de “depende” sem fechar exatamente do que depende.
3. Nada de esconder incerteza em texto corrido.
4. Nada de transformar configuração em prova de capacidade.
5. Nada de transformar backup configurado em restore comprovado.
6. Nada de declarar 10k ou 15k suportado sem prova objetiva.
7. Se houver dado ausente, nomeie o dado ausente e o impacto dele.
8. Se houver drift entre host e repositório, trate isso como achado crítico.
9. Se a conclusão forte não puder ser sustentada, use `nao comprovado`.
10. Seja rigoroso, objetivo, auditável e sem desvio.
```

## O que foi adicionado e por quê

| Adição | Justificativa |
|---|---|
| Separação entre `ativos/dia` e `simultâneos em pico` para 10k e 15k | Remove a principal ambiguidade de `usuários ativos` e reduz falso positivo. |
| Obrigação de cruzar SSH read-only, `deployment/` e bootstrap real da aplicação | Força diagnóstico baseado no host real e no fluxo real de acesso ao banco. |
| Contexto explícito de PostgreSQL, pgBouncer e pgBackRest já encontrado no repositório | Evita prompt genérico e ancora a análise na topologia atual. |
| Regras rígidas para diferenciar backup configurado de restore comprovado | Impede conclusão otimista só porque existe S3 + pgBackRest. |
| Gates objetivos por alvo com classificação fechada | Obriga veredito claro para 10k e 15k, sem resposta vaga. |
| Formato de saída fechado com tabelas e perguntas finais obrigatórias | Aumenta auditabilidade e reuso imediato do prompt. |
