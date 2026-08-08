# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Limite de 20s via configuração com novo default, sem endurecer a faixa de validação
- **Data:** 2026-08-07
- **Status:** Aceita
- **Decisores:** Usuário (dono do produto), agente de engenharia
- **Relacionados:** `.specs/prd-limite-audio-20-segundos-whatsapp/prd.md` (RF-01, RF-02, RF-05), `techspec.md`, `docs/us/US-002-limite-audio-20-segundos-whatsapp.md`

## Contexto

O pipeline de áudio do WhatsApp já enforça duração máxima via `AGENT_AUDIO_MAX_DURATION`, com default de 60s em `configs/config.go:1441`, faixa de validação `[1s..60s]` em `configs/config.go:1062-1067` e enforcement em `internal/agents/application/usecases/process_audio_inbound.go:303`. O produto decidiu reduzir o aceite para 20 segundos por custo de STT e latência. Restrições: zero regressão, rollback rápido, nenhum mecanismo novo de validação.

## Decisão

Aplicar o limite de 20s exclusivamente por configuração: alterar o default para `20*time.Second` em `configs/config.go:1441` e alinhar `.env.example:240` e `deployment/config/prod.env:218` para `20s`. Manter a faixa de validação `[1s..60s]` intacta, permitindo que operação ajuste ou reverta o limite por variável de ambiente sem deploy. Nenhum código de enforcement é alterado.

## Alternativas Consideradas

1. Somente configuração de ambiente, mantendo default de 60s no código. Vantagem: diff ainda menor. Desvantagem: qualquer ambiente sem a variável explícita aceitaria 60s, furando a regra de produto. Rejeitada por não garantir o comportamento novo de forma universal.
2. Teto rígido: reduzir a validação para `[1s..20s]`. Vantagem: impede elevar o limite sem deploy. Desvantagem: elimina o rollback operacional instantâneo e exigiria deploy para qualquer ajuste, aumentando o custo de resposta a incidente de UX. Rejeitada por perda de reversibilidade.
3. Novo mecanismo de enforcement (ex.: rejeição no dispatcher antes do outbox). Desvantagem: duplicaria uma capacidade existente e violaria a regra de menor mudança segura. Rejeitada.

## Consequências

### Benefícios Esperados

- Comportamento novo garantido em qualquer ambiente, inclusive os sem variável explícita.
- Rollback ou ajuste fino por variável de ambiente em minutos, sem deploy, dentro de faixa já validada no boot.
- Diff mínimo e auditável; o enforcement existente e seus testes continuam válidos.

### Trade-offs e Custos

- Um ambiente pode operar com limite diferente de 20s por escolha operacional; é um escape intencional e documentado, não um defeito.
- A mensagem de rejeição carrega o número 20 segundos (ver ADR-002) e pode divergir se o limite for alterado via env; mitigado por acoplamento documentado no runbook.

### Riscos e Mitigações

- Risco: ambiente de produção deployado sem atualização do `prod.env`. Impacto: baixo, pois o default novo de 20s cobre a ausência da variável. Mitigação: `prod.env:218` é versionado no repositório e atualizado no mesmo ciclo.
- Plano de rollback: definir `AGENT_AUDIO_MAX_DURATION=60s` (ou outro valor dentro de `[1s..60s]`) no ambiente e reiniciar o processo.

## Plano de Implementação

1. Alterar `configs/config.go:1441` para `20*time.Second`.
2. Atualizar `.env.example:240` e `deployment/config/prod.env:218` para `20s`.
3. Adicionar testes de default e de override em `configs/config_test.go`.
4. Critério de conclusão: testes de config passando e boot de produção validando a faixa existente sem erro.

## Monitoramento e Validação

- Sucesso: 100% dos áudios acima do limite efetivo rejeitados antes do STT, visível na métrica `agents_audio_inbound_total{outcome="rejected",reason="duration_exceeded"}` e no alerta da ADR-004.
- Critério de revisão: taxa de `duration_exceeded` sustentada acima do esperado após o primeiro ciclo indica limite agressivo demais e justifica ajuste via env.

## Impacto em Documentação e Operação

- `deployment/runbooks/audio-whatsapp-stt.md:80` (tabela de configs) e `:340-341` (pré-requisitos de canary).
- `deployment/runbooks/audio-whatsapp-pos-deploy.md:185-188` (cenário 3.4 reescrito: áudio de 55-60s passa a ser rejeição esperada).
- `.env.example` e `deployment/config/prod.env`.

## Revisão Futura

Revisar após 30 dias de produção ou quando a taxa de `duration_exceeded` estabilizar, avaliando se 20s permanece o ponto correto de equilíbrio entre custo e experiência.
