# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Strangler Fig atômico — migração de primitivos WhatsApp de `internal/onboarding` para `internal/platform/whatsapp`
- **Data:** 2026-06-08
- **Status:** Aceita
- **Decisores:** Jailton (arquitetura), product owner do MeControla
- **Relacionados:** `.specs/prd-auth-foundation/prd.md` (v3, RF-04, RF-28, RF-31), `.specs/prd-auth-foundation/techspec.md`, `.specs/prd-auth-foundation/adr-001-principal-contract-and-future-http-boundary.md`, `.specs/prd-onboarding-magic-token/prd.md`

## Contexto

O módulo `internal/onboarding` em produção contém primitivos que servirão a ambos o fluxo de pré-ativação (existente) e o novo fluxo pós-ativação (WhatsApp como canal do agent LLM):

- `internal/onboarding/infrastructure/http/server/middleware/meta_signature.go` (HMAC SHA-256 com rotação `current+next`).
- `internal/onboarding/infrastructure/http/server/middleware/raw_body_buffer.go`.
- `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go` (escopo diferente do novo — não é migrado).
- `internal/onboarding/infrastructure/http/server/handlers/meta_models.go` (tipos do envelope Meta).
- Lógica de extração de mensagem em `whatsapp_inbound_handler.go`.

O novo fluxo precisa do mesmo HMAC, do mesmo raw body buffer, dos mesmos tipos. Deixar duplicação em produção é proibido (cripto duplicada amplia surface de erro humano; mudança em um lado e esquecida no outro causa drift). A pergunta de arquitetura é **como migrar sem janela de regressão** e **sem janela de duplicação**.

Restrições inegociáveis: o onboarding está em produção; o webhook não pode regredir comportamento observável; mudança cross-PRD precisa de rastreabilidade (`prd-onboarding-magic-token` recebe spec-version bump).

## Decisão

Migrar os primitivos para `internal/platform/whatsapp/{signature,payload,dedup}` em **PR único atômico** que:

1. Cria os pacotes novos com o código migrado **bit-a-bit** (não reescreve; move).
2. Move a suíte de testes inteira junto (`meta_signature_test.go` → `internal/platform/whatsapp/signature/hmac_test.go`).
3. Reescreve `internal/onboarding/infrastructure/http/server/router.go` e `whatsapp_inbound_handler.go` para consumir o novo pacote.
4. **Deleta** os arquivos antigos no mesmo commit.
5. Atualiza `.specs/prd-onboarding-magic-token/prd.md` (spec-version bump) registrando a migração e referenciando este ADR.
6. Garante que `task` (build + test + integration test do webhook de onboarding) fica verde antes do merge.

**Escopo da migração:**

| Origem (deletado) | Destino |
|---|---|
| `internal/onboarding/.../middleware/meta_signature.go` | `internal/platform/whatsapp/signature/hmac.go` |
| `internal/onboarding/.../middleware/meta_signature_test.go` | `internal/platform/whatsapp/signature/hmac_test.go` |
| `internal/onboarding/.../middleware/raw_body_buffer.go` | `internal/platform/whatsapp/signature/raw_body_buffer.go` |
| `internal/onboarding/.../handlers/meta_models.go` | `internal/platform/whatsapp/payload/types.go` |
| `internal/onboarding/.../handlers/whatsapp_inbound_handler.go::extractFirstMessage` (função) | `internal/platform/whatsapp/payload/parser.go` |
| `internal/onboarding/.../middleware/rate_limit.go` | **NÃO migra** — escopo diferente; novo `internal/platform/whatsapp/ratelimit/limiter.go` é por `user_id`, não por IP/global. |

**Fora do PR atômico:**

- `Dispatcher`, `Limiter`, `EstablishPrincipal`, `auth_events`, etc. — esses ficam para PRs subsequentes do épico (passos 7-9 da Ordem de Build da techspec).
- Apenas a **extração** dos primitivos é atômica.

## Alternativas Consideradas

### Alternativa 1 — 3 PRs sequenciais (cria novo → migra onboarding → deleta antigo)

- **Descrição**: PR1 cria os pacotes novos sem mexer no onboarding (código duplicado); PR2 reescreve onboarding para usar os novos; PR3 deleta o código antigo.
- **Vantagens**: Cada PR é menor e mais fácil de revisar isoladamente; permite rollback granular.
- **Desvantagens**: Janela de coexistência entre PR1 e PR3 mantém criptografia duplicada em produção. Se PR2 ou PR3 atrasarem por qualquer motivo (review, conflito, outro incidente), a duplicação persiste indefinidamente como dívida técnica. Histórico de projetos similares mostra que "PR de cleanup" frequentemente fica esquecido.
- **Motivo de não escolha**: violação direta do RF-28 do PRD (que exige PR único). Risco operacional maior que o benefício de PRs menores.

### Alternativa 2 — Manter ambos por 1 sprint para safety

- **Descrição**: Como #1, mas adicionando uma sprint de coexistência intencional para "monitorar".
- **Vantagens**: Tempo extra para descobrir bugs antes de remover o antigo.
- **Desvantagens**: Tempo extra em troca de **nada concreto** — não há sinais de telemetria a observar (HMAC é binário: passa ou não passa). Aumenta surface de erro humano.
- **Motivo de não escolha**: cargo cult. Sem critério mensurável de "safety atingida", a sprint se torna arbitrária.

### Alternativa 3 — PR único atômico (escolhida)

- **Descrição**: tudo no mesmo PR.
- **Vantagens**: zero janela de duplicação; refactor genuinamente atômico; review único e holístico; rollback é simples `git revert`.
- **Desvantagens**: PR maior; review mais demorado.
- **Mitigação**: o tamanho do PR é dominado por mudanças de import path (mecânicas) e mudanças de package declaration (mecânicas). O conteúdo lógico mudado é pequeno (apenas wiring no router e handler).

## Consequências

### Benefícios Esperados

- **Eliminação imediata de duplicação de criptografia em produção**.
- **Estado consistente do código a cada commit**: nunca há um ponto em que dois HMACs coexistem.
- **Rollback simples**: `git revert <sha>` restaura tudo de uma vez.
- **`internal/platform/whatsapp` torna-se o canônico** para qualquer futuro fluxo WhatsApp (não só auth, mas qualquer outro canal de mensagem).
- **Cross-PRD rastreabilidade**: spec-version bump em `prd-onboarding-magic-token` documenta a migração para auditoria futura.

### Trade-offs e Custos

- **PR grande**: ~300-500 linhas movidas (mecânicas) + ~50 linhas de wiring (lógicas). Review exige atenção, mas é direto (diff `git mv` deveria preservar histórico onde possível).
- **Coordenação de timing**: precisa entrar em janela de baixo risco (sem deploy concorrente em onboarding). Mitigado por ser refactor sem mudança de comportamento.

### Riscos e Mitigações

- **Risco**: bug sutil ao mover (typo, import esquecido, package declaration errada) quebra produção do onboarding.
  **Impacto**: Alto (ativação para).
  **Mitigação**: testes movidos primeiro (TDD inverso); CI roda suíte completa de onboarding + nova suíte de whatsapp; integration test do webhook real obrigatório em CI; smoke `task onboarding:smoke` (criar se ausente; ver Plano de Implementação) cobre fluxo end-to-end de ativação.

- **Risco**: `git mv` perde histórico de blame por causa de mudanças simultâneas no arquivo.
  **Impacto**: Médio (perda de contexto histórico).
  **Mitigação**: minimizar mudanças no conteúdo dos arquivos movidos no commit de movimentação; deixar mudanças funcionais para commits subsequentes no mesmo PR.

- **Risco**: `prd-onboarding-magic-token` não recebe spec-version bump no mesmo PR e drift documental fica.
  **Impacto**: Médio (auditoria futura confusa).
  **Mitigação**: RF-31 do PRD exige a atualização no mesmo PR; CI valida com `ai-spec` que spec-version do PRD foi incrementada (regra a ser declarada na techspec).

- **Risco**: outro PR concorrente também mexe em arquivos do onboarding e gera conflito de merge grande.
  **Impacto**: Médio (retrabalho).
  **Mitigação**: comunicação prévia no time; janela de merge prioritária para este PR; freeze de PRs em onboarding durante a janela.

### Plano de Rollback

`git revert <merge-sha>` restaura o estado anterior atomicamente. Como nada novo do épico de auth depende ainda de produção (auth_events, consumer, dispatcher só vêm em PRs subsequentes), o rollback do Strangler Fig é isolado e não corrompe dados.

## Plano de Implementação

1. **Pré-trabalho**: criar `task onboarding:smoke` se não existir (cobre HMAC + ativação completa via webhook real em staging).
2. **Commit 1 do PR**: `git mv` dos arquivos para os novos caminhos; ajusta apenas `package` declarations e imports relativos. **Não muda lógica.**
3. **Commit 2 do PR**: reescreve `internal/onboarding/.../router.go` e `whatsapp_inbound_handler.go` para consumir o novo pacote. **Mantém comportamento idêntico.**
4. **Commit 3 do PR**: atualiza `.specs/prd-onboarding-magic-token/prd.md` (spec-version bump v? → v?+1) com nota referenciando este ADR.
5. **CI obrigatório**: build verde + suíte unitária verde + integration tests do onboarding verdes + nova suíte de whatsapp verde + smoke `task onboarding:smoke` verde.
6. **Merge**: janela de baixo risco; deploy imediato em staging; smoke pós-deploy em staging; deploy em produção; smoke pós-deploy em produção.

Critério de adoção concluída: produção rodando com o novo pacote por 7 dias sem regressão observável em ativação (métrica `onboarding_activation_total` e `meta_signature_status_total{status='valid'}` estáveis).

## Monitoramento e Validação

- Métrica `meta_signature_status_total{status='valid'}` continua subindo no mesmo ritmo pré-migração.
- Métrica `onboarding_activation_total` continua subindo no mesmo ritmo pré-migração.
- Alerta `meta_signature_status_total{status='invalid'} > 1% das requisições em 5 min` (já existente) continua sem disparar.
- Sem regressão observável em error rate do webhook `/whatsapp/inbound` por 7 dias após deploy em produção.

## Impacto em Documentação e Operação

- **`prd-onboarding-magic-token/prd.md`**: spec-version bump + nota de migração (RF-31 do prd-auth-foundation).
- **`AGENTS.md`**: atualizar lista de pacotes em "Padrão Obrigatório de Módulo" para incluir `internal/platform/whatsapp` como infraestrutura compartilhada.
- **Runbook `docs/runbooks/auth-meta-secret-rotation.md`**: cita o novo caminho `internal/platform/whatsapp/signature/hmac.go` ao explicar rotação.
- **Dashboard Grafana**: ajustar queries/painéis que filtravam por nome de processo/pacote para apontar para o novo caminho (se aplicável).

## Revisão Futura

Esta ADR deve ser revisitada se:

- Outro canal de mensagem (Telegram, SMS) for adicionado e justificar promover `internal/platform/whatsapp` para `internal/platform/messaging/whatsapp` (revisar S-02 do PRD).
- A separação `signature/payload/dedup/dispatcher/ratelimit` mostrar-se artificial e demandar consolidação.
- Surgir cliente externo que consuma `internal/platform/whatsapp` como biblioteca pública (improvável — pacote `internal/*` por design não é exportável).
