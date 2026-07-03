# Documento de Requisitos do Produto (PRD) — Evolução de Infraestrutura KVM 2 (0 → 10k, envelope B)

<!-- spec-version: 1 -->

- Origem: `docs/runs/2026-07-03-relatorio-analise-infra-hostinger-kvm2-10k.md` (relatório auditável de infra, com prova SSH read-only da VPS de produção)
- Tipo: remediação de confiabilidade + eficiência de custo, alvo de escala **envelope B (10k ativos/dia)** em single-node
- Foco declarado (inegociável): **eficiência, economia e robustez**
- Decisões de escopo travadas (2026-07-03):
  - **D-01** — Alvo = **envelope B** (10k/dia). Sharding de ordenação (ADR-001 do PRD whatsapp) e HA multi-node ficam **fora deste PRD** (envelope C). SPOF single-node é aceito e documentado.
  - **D-02** — O runner de CI self-hosted é **removido totalmente** do host de produção; build/scan/sign/deploy migram para runner GitHub-hosted (deploy via SSH).
  - **D-03** — Observabilidade permanece **self-host** (otel-lgtm); reduz-se o trace sampling em produção em vez de externalizar.

## Visão Geral

O relatório de 2026-07-03 provou, com evidência de código e observação direta da VPS de produção (`root@187.77.45.48`, comandos read-only), que a stack em Docker Swarm single-node **funciona e está folgada em ~0 usuários** (idle ~871 MB / 7,75 GiB; app viva, HTTP 200 em 4 ms), porém **não é production-ready/proof** para crescer com segurança até 10k. Os bloqueios são operacionais e de eficiência, não de reescrita:

1. **DR não validado** — pgBackRest opera (imagem custom deployada, AES-256, WAL→S3, 3 full backups), mas **não há agendamento** (nenhum crontab/systemd-timer instalado; último full 2026-07-01) e o **restore/PITR nunca foi testado**.
2. **Desperdício removível sem custo** — o **runner de CI roda no host de produção** (~50 GiB de disco entre build cache 22 GiB, imagens 25 GiB e workspace 2,4 GiB) e disputa os 2 vCPU nos builds; o **trace sampling está em 100%** em produção (`compose.swarm.yml` força `"1"` sobre `prod.env=0.1`, e o gate anti-storm exige isso).
3. **Sem prova de capacidade** — nenhum teste de carga foi executado em qualquer envelope de 10k; o veredito de B é **não comprovado**.
4. **Margem de recurso apertada em worst-case** — os `limits` do compose somam 5,55 vCPU (2,8× o host) e 6,77 GB RAM (~87% de 8 GB antes do overhead do OS), amparados apenas por swap e pela não-coincidência de picos.
5. **Higiene de release e superfície** — produção roda `571425f` (atrás de `main`); `pg-tunnel` escuta em `0.0.0.0:15432` (mitigado só por ufw); o rate-limit por IP pode estrangular rajadas legítimas dos webhooks da Meta.

Esta iniciativa fecha esses gaps para tornar a infra **comprovadamente confiável e eficiente em custo até o envelope B**, usando os componentes já existentes (Swarm single-node, pgBackRest, otel-lgtm, Caddy, pgBouncer), **sem** introduzir Kafka/NATS, sharding, HA multi-node ou serviços pagos além de, no máximo, o upgrade vertical KVM 2 → KVM 4 quando (e somente se) o teste de carga provar necessidade.

## Objetivos

1. Backup automatizado, versionado e alertado; restore/PITR **comprovado por ensaio**.
2. Zero footprint de CI no host de produção; disco e CPU liberados para a aplicação.
3. Observabilidade eficiente em custo (sampling proporcional) sem perder rastreio de erros.
4. Orçamento de recursos saudável em 8 GB, com gatilho objetivo de upgrade vertical.
5. Capacidade de envelope A e B **comprovada por teste de carga**, ou o gap registrado com honestidade.
6. Produção alinhada à `main`, com alerta de drift; superfície de rede endurecida.

## Não-Objetivos

- Sharding de ordenação por hash (ADR-001 do PRD whatsapp) e escala horizontal de workers — envelope C.
- Alta disponibilidade multi-node ou banco gerenciado — remoção do SPOF fica fora deste PRD.
- Externalização da observabilidade para Grafana Cloud ou similar.
- Troca de provedor de nuvem, de LLM (OpenRouter) ou de broker.

## Requisitos Funcionais

### Backup agendado e alertado
- **RF-01** — Agendar pgBackRest (full semanal, diff diário, incr a cada 6h, conforme `deployment/pgbackrest/crontab.txt`) via mecanismo **versionado e idempotente** (cron/systemd-timer provisionado por script), de modo que o agendamento não dependa de configuração manual não rastreada no host.
- **RF-02** — Alertar quando o último backup full/diff exceder um limiar de idade (backup stale/ausente) e quando `archive-push` falhar, com o alerta visível na stack de observabilidade existente.
- **RF-03** — Garantir, no caminho de deploy, que a imagem custom com pgBackRest (`mecontrola-postgres:*`) é obrigatória em produção; o deploy deve **falhar explicitamente** se `POSTGRES_IMAGE` cair para a imagem default sem pgBackRest.

### Restore/PITR comprovado
- **RF-04** — Executar um ensaio de **restore PITR** em ambiente isolado (não-produção), medindo RTO real e validando integridade dos dados, com evidência anexada.
- **RF-05** — Executar um ensaio de **restore completo de VPS** a partir do repositório S3 (banco + config + secrets), com evidência anexada.
- **RF-06** — Atualizar os runbooks `restore-pitr.md` e `restore-vps.md` com **RPO/RTO reais medidos** e declarar o SLO de recuperação do envelope B.

### Remoção do CI do host de produção
- **RF-07** — Remover **totalmente** o runner self-hosted do host de produção (desregistrar no GitHub, remover serviço/usuário/diretórios) e recuperar o disco associado (build cache, imagens órfãs, workspace).
- **RF-08** — Migrar os jobs de build, scan (Trivy) e assinatura (cosign) para runners **GitHub-hosted**, preservando os gates bloqueantes existentes.
- **RF-09** — Reescrever o job de deploy para executar de runner GitHub-hosted **via SSH/docker context**, sem runner no host, preservando descriptografia SOPS+age, criação de secrets, `docker stack deploy`, healthcheck e rollback automático já existentes.
- **RF-10** — Estabelecer higiene de disco recorrente (prune agendado de build cache e imagens antigas) e **alerta de disco** acima de limiar no host.

### Eficiência de observabilidade
- **RF-11** — Reduzir o trace sampling em produção (amostragem proporcional/parent-based, preservando 100% de traces com erro), **alinhando** `compose.swarm.yml` e `prod.env` para um único valor efetivo.
- **RF-12** — Ajustar o gate `deploy-anti-storm` para **permitir** o sampling reduzido controlado, sem reabrir o risco que o gate original mitigava.
- **RF-13** — Documentar formalmente o **SPOF de observabilidade single-node aceito** (envelope B) e os limites de retenção dos sinais.

### Orçamento de recursos e pool
- **RF-14** — Revisar `limits`/`reservations` do compose para que o **worst-case de memória caiba com margem** em 8 GB (host − overhead de OS/Docker/Swarm), eliminando o risco de oversubscription perigosa de RAM.
- **RF-15** — Dimensionar e **alertar saturação** do pool (pgBouncer + `DB_MAX_CONNS`), garantindo folga para jobs de background e rajadas de retry do outbox.
- **RF-16** — Documentar o **orçamento de recursos aprovado** e o **gatilho objetivo** de upgrade vertical KVM 2 → KVM 4 (métrica e limiar que disparam a decisão).

### Prova de capacidade
- **RF-17** — Implementar harness de teste de carga (k6) proporcional aos **envelopes A (10k/mês) e B (10k/dia)**, cobrindo webhook de entrada, drenagem de outbox e leitura, com perfis de carga documentados.
- **RF-18** — Definir critérios de aprovação objetivos (p95, taxa de erro, uso de pool e CPU) e produzir **relatório de evidência**, convertendo o veredito de B de "não comprovado" para `comprovado` ou registrando o gap remanescente sem falso positivo.

### Higiene de release e superfície
- **RF-19** — Deployar a `main` atual em produção (sair de `571425f`) com verificação de saúde pós-deploy.
- **RF-20** — Adicionar gate/alerta de **drift de versão** entre produção (`OTEL_SERVICE_VERSION`) e o `HEAD` da `main`.
- **RF-21** — Tratar a interação **rate-limit × webhooks Meta** (allowlist ou limite específico para os IPs de origem da Meta), evitando estrangular rajadas legítimas, sem afrouxar a proteção contra abuso.
- **RF-22** — Restringir o `pg-tunnel` (bind em loopback e/ou remoção quando ocioso), reduzindo a superfície de banco além do ufw.

## Restrições e Premissas

- Single-node Docker Swarm mantido (D-01); nenhuma mudança que exija segundo nó.
- Somente componentes já presentes na stack; nenhum serviço pago novo (exceto upgrade vertical KVM condicional).
- Todas as mudanças de código Go seguem `go-implementation` e as regras `.claude/rules/*`; zero comentários em `.go` de produção.
- Preços Hostinger são snapshot promocional (KVM 2 R$43,99 promo / R$77,99 renov.; KVM 4 R$59,99 / R$149,99) e devem ser reconferidos na contratação.
- Toda afirmação de "comprovado" exige evidência anexada; ausência de prova permanece `não comprovado`.

## Critérios de Aceite Globais

- Backups agendados e verificáveis (`pgbackrest info` com full ≤ 7 dias e diff ≤ 24 h), com alerta funcional de staleness.
- Restore PITR e restore de VPS executados com evidência e RTO medido.
- Nenhum processo de runner no host de produção; disco de produção majoritariamente livre.
- Sampling de produção reduzido e coerente entre compose e env; gate ajustado e verde.
- Worst-case de memória do compose com margem sobre 8 GB; alerta de pool ativo.
- Relatório de carga dos envelopes A e B com veredito honesto.
- Produção na `main`; alerta de drift ativo; `pg-tunnel` restrito.
