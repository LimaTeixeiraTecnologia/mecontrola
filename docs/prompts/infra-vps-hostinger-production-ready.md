# Prompt Enriquecido: Infraestrutura Production-Ready em VPS Hostinger

Data da pesquisa: 2026-06-04
Idioma de resposta esperado: pt-BR
Escopo: planejamento e especificacao de infraestrutura. Nao implementar nada.

## Prompt Original

> Eu quero montar toda infra, focando em economia, eficiencia, robustez, production-ready de forma inegociavel para hospedar essa aplicacao em uma VPS na https://hpanel.hostinger.com/, pesquise em 2026 as melhores e mais recomendadas praticas de publicacao, foco em seguranca, stack completa em qual SO, se vai usar Docker, o foco e um MVP robusto, confiavel e usavel em producao, eu quero tambem usar local para validar toda feature.
>
> NAO IMPLEMENTE NADA, APENAS CRIE/ENRIQUECA O PROMPT.

## Prompt Enriquecido

Voce e um arquiteto de infraestrutura/SRE senior. Preciso de um plano production-ready, economico, robusto e operacionalmente simples para hospedar a aplicacao `mecontrola` em uma VPS da Hostinger gerenciada pelo hPanel.

Antes de responder, pesquise e valide informacoes atuais em 2026 usando fontes oficiais ou primarias sempre que possivel. Priorize documentacao oficial da Hostinger, Docker, Ubuntu/Canonical, Caddy/Nginx/Traefik quando aplicavel, OWASP e CIS/NIST para seguranca. Cite os links usados e destaque qualquer inferencia.

Contexto conhecido do projeto:

- Repositorio: monolito modular em Go.
- Modulos principais: `identity`, `billing`, `platform`.
- Runtime: Go, HTTP server, worker e migrations.
- Banco detectado no codigo: PostgreSQL.
- Dockerfile existente em `deployment/docker/Dockerfile`, multi-stage, build Go, runtime distroless nonroot.
- Objetivo de negocio: MVP robusto, confiavel e utilizavel em producao com menor custo operacional razoavel.
- Ambiente alvo: uma VPS Hostinger acessada e administrada via hPanel.
- Tambem preciso de ambiente local para validar toda feature antes de publicar.

Restricoes obrigatorias:

1. Nao implementar nada.
2. Nao criar arquivos, scripts, Docker Compose, workflows, comandos destrutivos ou alteracoes no repositorio.
3. Nao assumir detalhes ausentes do projeto sem marcar como suposicao.
4. Nao propor Kubernetes para o MVP, salvo se justificar explicitamente por que seria necessario. A preferencia inicial e VPS unica simples.
5. Nao expor banco, Docker daemon, painel administrativo ou metricas sensiveis na internet.
6. Nao usar senha SSH como padrao operacional; priorizar chaves SSH, usuario nao-root, sudo e hardening.
7. Todo segredo deve ficar fora do Git e com estrategia clara de rotacao.
8. Toda recomendacao deve considerar custo, manutencao, seguranca, confiabilidade, observabilidade, backup e rollback.

Perguntas que a resposta deve resolver:

1. Qual SO usar na VPS em 2026 e por que? Compare Ubuntu Server LTS 24.04/26.04 quando relevante e recomende uma escolha conservadora.
2. Devo usar Docker? Se sim, qual desenho: Docker Engine + Docker Compose, rootless quando viavel, redes privadas, volumes persistentes e imagens imutaveis.
3. Qual reverse proxy usar para um MVP economico e robusto: Caddy, Nginx ou Traefik? Recomende um, justifique trade-offs e explique TLS/HTTPS.
4. Como organizar ambientes `local`, `staging` opcional e `production`, mantendo paridade suficiente sem aumentar custo demais?
5. Como validar toda feature localmente antes de deploy? Inclua build, testes, migracoes, compose local, smoke tests e checklist de aceite.
6. Como desenhar deploy seguro na VPS? Inclua estrategia manual inicial e caminho evolutivo para CI/CD sem overengineering.
7. Como configurar seguranca da VPS? Inclua SSH, firewall, portas, updates automaticos, fail2ban ou alternativa, hardening Docker, permissoes, backups e secrets.
8. Como operar PostgreSQL? Compare banco no mesmo host via container versus gerenciado/externo, com recomendacao para MVP e limites claros para migrar.
9. Como fazer backup e restore? Inclua backups/snapshots do hPanel, dump logico do PostgreSQL, backup de volumes, teste de restauracao e retencao.
10. Como monitorar e diagnosticar? Inclua logs, health checks, metricas minimas, alertas de disco/memoria/CPU, uptime check e observabilidade compativel com baixo custo.
11. Como publicar novas versoes com rollback? Inclua tagging de imagens, backup pre-deploy, migracoes, health check, rollback e criterio de abortar deploy.
12. Quais riscos continuam aceitos no MVP e quais sao inegociaveis antes de expor em producao?

Formato obrigatorio da resposta:

```markdown
# Plano de Infraestrutura VPS Hostinger para o mecontrola

## Sumario Executivo
- Decisao recomendada de stack
- Por que esta decisao equilibra economia, seguranca e robustez

## Fontes Consultadas
- Lista com links, data de acesso e o que cada fonte sustentou

## Decisoes Arquiteturais
### SO
### Containerizacao
### Reverse Proxy e TLS
### Banco de Dados
### Ambientes Local/Staging/Producao

## Arquitetura Recomendada
- Diagrama textual simples
- Fluxo de request
- Fluxo de deploy
- Fluxo de backup/restore

## Baseline de Seguranca
- SSH
- Firewall
- Docker/containers
- Secrets
- TLS
- Banco
- Atualizacoes
- Permissoes

## Estrategia Local-First
- Como validar cada feature localmente
- Paridade local/producao
- Checks antes de merge/deploy

## Operacao em Producao
- Deploy
- Rollback
- Logs
- Metricas
- Alertas
- Backup
- Restore drill
- Rotina semanal/mensal

## Custos e Trade-offs
- Escolha minima recomendada
- Quando escalar
- O que evitar no MVP

## Checklist Production-Ready
- Itens bloqueantes
- Itens recomendados
- Itens futuros

## Plano de Implementacao Futuro
- Etapas ordenadas
- Arquivos que provavelmente precisariam ser criados ou alterados
- Validacoes por etapa
- Riscos e criterios de parada

## Perguntas em Aberto
- Somente perguntas que realmente bloqueiam uma decisao segura
```

Criterios de aceite:

- A resposta deve ser acionavel, mas nao deve implementar nada.
- Deve recomendar uma stack completa e coerente para VPS Hostinger.
- Deve justificar Docker ou alternativa com trade-offs reais.
- Deve separar claramente local, producao e eventual staging.
- Deve incluir seguranca minima inegociavel antes de producao.
- Deve incluir plano de backup, restore e rollback testavel.
- Deve incluir criterios para validar feature localmente antes de deploy.
- Deve apontar limites do MVP e quando migrar para arquitetura mais robusta.
- Deve citar fontes atuais de 2026 ou documentacao ainda vigente em 2026.
- Deve marcar suposicoes e evitar prometer garantia absoluta de disponibilidade.

## Ambiguidades Resolvidas no Prompt

- "Toda infra" foi limitado a planejamento de VPS, runtime, deploy, seguranca, observabilidade, backup e validacao local.
- "Production-ready inegociavel" foi traduzido em criterios bloqueantes verificaveis, nao em complexidade desnecessaria.
- "Economia" foi tratada como preferencia por VPS unica e ferramentas simples, com caminho evolutivo.
- "Usar local para validar toda feature" virou uma estrategia local-first com checks antes de deploy.
- "Se vai usar Docker" virou uma decisao a ser justificada, com preferencia provavel por Docker Compose para MVP.

## Base de Pesquisa Usada para Enriquecer

- Hostinger informa que o hPanel da VPS inclui secoes para Settings, Operating System, Backup & Monitoring e Security, com SSH keys, firewall, malware scanner e metricas de uso.
- Hostinger documenta template Ubuntu 24.04 com `docker-ce` e `docker-compose` pre-instalados para VPS Docker.
- Hostinger diferencia backups automaticos e snapshots: backups semanais por padrao, opcao diaria, snapshots manuais temporarios e apenas um snapshot armazenado por vez.
- Docker documenta Compose em producao com arquivo adicional de producao, remocao de bind mounts de codigo, portas diferentes, variaveis de ambiente de producao, restart policy e servicos auxiliares de logs.
- Docker recomenda multi-stage builds e usuario nao-root quando o servico nao precisa de privilegios.
- Docker rootless mode reduz risco ao executar daemon e containers sem privilegio root, mas deve ser validado contra requisitos da stack.
- Ubuntu Server recomenda `ufw` como frontend padrao de firewall e `unattended-upgrades` para atualizacoes automaticas de seguranca.
- Caddy fornece HTTPS automatico com renovacao de certificados e redirect HTTP para HTTPS quando DNS e portas 80/443 estao corretos.
- OWASP Docker Security Cheat Sheet reforca manter host/Docker atualizados, nao expor socket Docker, definir usuario, limitar capabilities, impedir privilege escalation, limitar recursos, filesystem read-only quando possivel, scanning e rootless quando viavel.

## Fontes Consultadas em 2026-06-04

- Hostinger - VPS Dashboard: https://www.hostinger.com/support/5726606-how-to-use-the-vps-dashboard-in-hostinger/
- Hostinger - Docker VPS Template: https://www.hostinger.com/support/8306612-how-to-use-the-docker-vps-template-at-hostinger/
- Hostinger - Backups e Snapshots: https://www.hostinger.com/support/1583232-how-to-back-up-or-restore-a-vps-at-hostinger/
- Docker - Compose em producao: https://docs.docker.com/compose/how-tos/production/
- Docker - Building best practices: https://docs.docker.com/build/building/best-practices/
- Docker - Rootless mode: https://docs.docker.com/engine/security/rootless/
- Ubuntu Server - Firewall/UFW: https://documentation.ubuntu.com/server/how-to/security/firewalls/
- Ubuntu Server - Automatic updates: https://documentation.ubuntu.com/server/how-to/software/automatic-updates/
- Ubuntu Server - OpenSSH: https://documentation.ubuntu.com/server/how-to/security/openssh-server/
- Caddy - Automatic HTTPS: https://caddyserver.com/docs/automatic-https
- OWASP - Docker Security Cheat Sheet: https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html

## Variante Curta para Uso Direto

Crie um plano production-ready, economico e robusto para hospedar o projeto Go `mecontrola` em uma VPS Hostinger via hPanel. Pesquise praticas atuais em 2026 e cite fontes oficiais. Nao implemente nada. Recomende SO, uso ou nao de Docker, reverse proxy/TLS, PostgreSQL, deploy, rollback, backup/restore, hardening de VPS, seguranca Docker, observabilidade minima e estrategia local-first para validar toda feature antes de producao. A resposta deve ser em pt-BR, Markdown, com decisoes justificadas, trade-offs, checklist production-ready, riscos aceitos para MVP e plano futuro de implementacao sem criar arquivos.
