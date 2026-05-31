# ADR-011 — Docker distroless nonroot + Fly 2 processes (app + worker)

## Metadados

- **Título:** Base image `gcr.io/distroless/static-debian12:nonroot` + `fly.toml` com 2 processes desde o foundation
- **Data:** 2026-05-31
- **Status:** Aceita
- **Decisores:** @JailtonJunior94
- **Relacionados:** [PRD §RF-07, §RNF-007, §D-23, §D-24, §CS-23, §CS-24](./prd.md), [techspec §Plano de Rollout M5](./techspec.md), [ADR-010 (cobra subcomandos)](./adr-010-cobra-subcommands.md), [R-SEC-001](../../.agents/skills/agent-governance/references/security.md)

## Contexto

A foundation precisa ir para produção (staging Fly) ao final do rollout (M5–M7). Duas decisões operacionais não estavam fixadas:

1. **Base image do Docker** — opções razoáveis: distroless, alpine, scratch, ubuntu. Trade-off entre tamanho, segurança, debuggability.
2. **Estrutura de processes no Fly** — opções: 1 process (`app=mecontrola server`, worker ativado por PRD futuro), 2 processes (`app` + `worker` desde o foundation), supervisord.

O budget do PRD original (R$ 25/mês) foi calibrado para 1 process. Adotar 2 desde o dia 1 estoura levemente o budget, mas valida o split em prod real e elimina classe de bug "wiring de cobra subcommand worker quebra em runtime que ninguém percebe até Epic 09".

## Decisão

### Docker base image

**`gcr.io/distroless/static-debian12:nonroot`**, com pipeline multi-stage:
- **Builder**: `golang:1.26.3-alpine` (≈ 350 MB; descartado após build).
- **Runtime**: `gcr.io/distroless/static-debian12:nonroot` (≈ 2 MB; sem shell, sem package manager, sem libc além de glibc estática; roda como UID 65532).
- **Build flags**: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=<sha>"`.
- **Imagem final**: ≤ 30 MB total (binário + CA certs do base distroless).

### Fly.toml processes

**2 processes desde o primeiro deploy**:

```toml
[processes]
  app    = "/app/mecontrola server"
  worker = "/app/mecontrola worker"
```

- `app` recebe tráfego HTTP via health checks; região `gru`; `vm_size="shared-cpu-1x"`, `vm_memory="256mb"`.
- `worker` sem ingress HTTP; região `gru`; mesmas specs.
- Health checks específicos para `app` (HTTP `/ready`); para `worker`, liveness via processo (Fly default).
- Cada process escala independentemente (`fly scale count app=N worker=M`).

### Budget revisado

R$ 25/mês → **R$ 60/mês** (RNF-007, OBJ-06 atualizados na v8 do PRD). Cobre: 2× shared-cpu-1x 256MB (~R$ 50) + Fly Postgres dev tier (~R$ 0–10 free credit) + Grafana Cloud free tier (R$ 0).

## Alternativas Consideradas

### Docker base image

1. **`alpine:3.20`**: pequeno (~8 MB) com shell. Vantagem: debug em runtime. Desvantagem: musl libc tem histórico de CVEs; shell exploitável; pacote tarball maior que distroless. Rejeitado por R-SEC-001.
2. **`scratch`**: 0 MB de base. Vantagem: superfície mínima. Desvantagem: precisa empacotar CA certs e tzdata manualmente; risco de quebrar TLS (Grafana Cloud) por esquecimento. Distroless já entrega CA certs + tzdata pre-empacotados — ganho de robustez sem perda real.
3. **`ubuntu:24.04`**: ~80 MB; muitos pacotes desnecessários; supply chain enorme. Rejeitado por R-SEC-001.

### Fly processes

1. **1 process apenas (`app=server`)**: economia R$ 30/mês; worker fica como subcomando sem prod. Desvantagem: bug de wiring do cobra/runtime em modo worker só descoberto quando Epic 09 entrar — tarde demais para correção barata.
2. **Supervisord no mesmo container**: 1 instância roda ambos. Vantagem: economia R$ 30/mês. Desvantagem: anti-pattern Fly (12-factor; cada processo deve ser sua machine); crash de worker mata o app; perde scale independente. Rejeitado.
3. **3 processes (server + worker + scheduler-tick)**: antecipa Epic 09. Viola fora-de-escopo do PRD.

## Consequências

### Benefícios Esperados

- **Segurança**: distroless nonroot reduz superfície de ataque drasticamente (sem shell, sem `cat`, sem `sh`).
- **Validação real do split**: `mecontrola worker` em prod desde o dia 1 ⇒ bug de wiring pega cedo.
- **Operação independente**: scale, log, deploy de `worker` separáveis (`fly logs -p worker`).
- **Tamanho**: imagem ≤ 30 MB ⇒ pull rápido, scale rápido, registry barato.
- **SBOM**: distroless tem SBOM oficial do Google ⇒ trivy gera relatório limpo.
- **Conformidade R-SEC-001**: nada de subprocesso shell exploitável; toda escrita auditável.

### Trade-offs e Custos

- **Budget**: +R$ 30/mês (vs single process). Aceito explicitamente na v8 do PRD (RNF-007 atualizado).
- **Debug**: sem shell em runtime ⇒ debug via logs/traces apenas (não `fly ssh console -C bash`). Mitigação: OTel cobre 99% dos casos; debug invasivo via `fly ssh console -C /app/mecontrola --help` ou imagem alternativa em emergência.
- **Build cache complexo**: multi-stage exige cache layer correto (`go mod download` antes de `COPY .`).

### Riscos e Mitigações

- **Risco**: `nonroot` (UID 65532) sem permissão para `bind` em porta <1024.
  - **Mitigação**: HTTP escuta em `8080` (padrão Fly; declarado em `fly.toml`).
- **Risco**: distroless não tem `tzdata` se a versão `static` não vier com ele.
  - **Mitigação**: usar `gcr.io/distroless/static-debian12:nonroot` (variante `static` inclui CA certs + tzdata); validar via integration test que `time.LoadLocation("America/Sao_Paulo")` funciona.
- **Risco**: `worker` em prod consumindo recursos sem fazer nada útil (placeholder).
  - **Mitigação**: log `info` "worker idle, no jobs registered" a cada 60s; gauge `worker_jobs_registered=0` torna óbvio.
- **Risco**: deploy falha silenciosamente em um dos processes mas o outro responde.
  - **Mitigação**: smoke test pós-deploy executa `fly status` e exige `started` em ambos processes; CI bloqueia release se algum falhar.
- **Risco**: budget Fly real ultrapassa R$ 60/mês com tráfego mínimo.
  - **Mitigação**: alerta Fly billing >= R$ 50/mês; revisão mensal nos primeiros 3 meses.

## Plano de Implementação

1. Criar `Dockerfile` multi-stage conforme spec.
2. Criar `fly.toml` com 2 processes + health checks + region `gru` + secrets reference.
3. Atualizar `taskfiles/build.yml` com `task docker:build`, `task docker:scan` (chama trivy image).
4. Atualizar `.github/workflows/cd.yml`: build image + trivy scan + push registry + `flyctl deploy`.
5. Criar `fly.toml` placeholder para staging (`mecontrola-staging`) e prod (`mecontrola`).
6. Smoke test pós-deploy: `fly status -a mecontrola-staging | grep started` em ambos processes.
7. Runbook "Debug em runtime sem shell": instruções para usar `fly logs`, OTel traces, e quando necessário deploy temporário com imagem alpine.

## Monitoramento e Validação

- Métrica Fly: `fly_machine_state{process,state}` (gauge).
- Alerta: `process=worker, state≠started` por >5min.
- Alerta: billing Fly ≥ R$ 50/mês (proativo).
- Log estruturado no startup do worker: `service=mecontrola process=worker version=<sha> jobs_registered=0`.

## Impacto em Documentação e Operação

- README: seção "Deploy" explicando 2 processes.
- Runbook "Deploy via Fly": `flyctl deploy --strategy=rolling` (rolling para não derrubar `app` simultaneamente).
- Runbook "Scale": `fly scale count app=2 worker=1` para emergência.
- Runbook "Debug sem shell": uso de `fly logs`, OTel, traces.
- Onboarding: incluir esta ADR.

## Revisão Futura

- Revisitar process worker quando Epic 09 (scheduler/jobs reais) entrar — pode crescer para `vm_memory=512mb`.
- Revisitar Docker base se distroless deixar de receber updates de segurança.
- Revisitar process split se billing virar dor (improvável no MVP).
