# ADR-004 — PII masking via pacote `internal/platform/observability/mask` + handler global como rede de segurança

## Metadados

- **Título:** Estratégia de mascaramento parcial de WhatsApp/Email em logs do módulo identity
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + Segurança/LGPD (autor do PRD)
- **Relacionados:** PRD (RF-19, RT-05), techspec §Monitoramento e Observabilidade, código existente `internal/platform/observability/{pii_handler.go, redaction.go}`

## Contexto

RF-19 exige mascaramento de `whatsapp_number` e `email` em todos os logs do módulo identity em **ponto único reutilizável**. O formato decidido é parcial — `+5511****8888` (preserva DDD + 4 últimos dígitos) e `a***@dominio.com` (preserva inicial + domínio) — para permitir triagem em suporte sem expor PII.

O `piiHandler` atual em `internal/platform/observability` substitui qualquer valor com chave em `PIIFields` por `[REDACTED]`, ignorando formato parcial. Há três caminhos:

1. Estender `piiHandler` com registry de transformadores por chave.
2. Criar pacote `mask` separado e usar o handler global apenas como rede de segurança.
3. Helper local em `identity/infrastructure/logging` sem tocar no handler.

## Decisão

Adotar caminho (2): defesa em profundidade.

- Novo pacote `internal/platform/observability/mask` expõe `WhatsApp(s string) string` e `Email(s string) string` com o formato parcial decidido. É a fonte única de mascaramento reutilizável (RF-19 "ponto único reutilizável").
- Código em identity (e outros módulos no futuro) loga **sempre** sob chave canônica `_masked`:

```go
slog.InfoContext(ctx, "identity: upsert por whatsapp concluído",
    slog.String("whatsapp_number_masked", mask.WhatsApp(user.WhatsAppNumber().String())),
)
```

- `internal/platform/observability/redaction.go` adiciona `"whatsapp_number"` e `"email"` (e `"display_name"` opcional) em `PIIFields`. Se algum dev esquecer e logar sob a chave crua `whatsapp_number`, o `piiHandler` substitui por `[REDACTED]` — vazamento é prevenido por construção.
- Auditoria: code review e o runbook do módulo identity registram a convenção de chave `_masked`.

## Alternativas Consideradas

- **Estender `piiHandler` com registry de masker por chave** — Vantagens: centralizado, escala bem com múltiplos formatos. Desvantagens: acopla observability a formatos específicos de cada módulo, dificulta evolução (cada PRD muda o handler global), perde-se a propriedade "logar valor cru = vaza" como sinal forte em revisão. Rejeitada.
- **Helper local em `identity/infrastructure/logging` sem tocar no handler global** — Vantagens: isolado. Desvantagens: outros módulos terão que reinventar; perde rede de segurança; viola "ponto único reutilizável". Rejeitada.

## Consequências

### Benefícios Esperados

- Reuso por billing/onboarding sem dependência cross-module (mask vive em platform).
- Vazamento por descuido é impedido pelo handler global.
- Testes do mask são puramente funcionais (string in, string out).

### Trade-offs e Custos

- Convenção de nome `_masked` precisa ser ensinada (runbook + code review).
- Dois caminhos coexistem (mask + handler) — leve sobrecarga conceitual.

### Riscos e Mitigações

- **Risco:** Dev novo loga `slog.String("whatsapp_number", num.String())` esperando mascaramento parcial e recebe `[REDACTED]` (sinal mais óbvio que vazamento, mas perda de utilidade).
- **Mitigação:** PR template inclui checklist "Log de PII usa `mask.X` + sufixo `_masked`?"; code reviewer pega facilmente.
- **Risco:** Mudança futura no formato (ex.: hash em vez de máscara) precisa atualizar todos os call sites.
- **Mitigação:** Mudança fica isolada em `mask.WhatsApp`/`mask.Email`; nenhum call site muda.

## Plano de Implementação

1. Criar `internal/platform/observability/mask/whatsapp.go`:
   ```go
   func WhatsApp(s string) string {
       // entrada esperada: E.164 BR ("+5511988887777"); resiliente a curtos.
       // saída: "+5511****8888" — preserva DDD (4 chars: +55 + DD) e 4 últimos dígitos.
   }
   ```
2. Criar `internal/platform/observability/mask/email.go`:
   ```go
   func Email(s string) string {
       // "a***@dominio.com" — primeiro char do local-part + "***" + "@" + domínio inteiro.
       // Sem "@" retorna "***".
   }
   ```
3. Testes table-driven cobrindo: input válido, vazio, curto, sem `@`, multi-`@`.
4. Atualizar `internal/platform/observability/redaction.go`:
   ```go
   var PIIFields = []string{"phone", "password", "token", "card_number", "amount", "whatsapp_number", "email", "display_name"}
   ```
5. Documentar a convenção `_masked` em `internal/identity/AGENTS.md`.

## Monitoramento e Validação

- Verificação post-deploy: `grep -E '"(whatsapp_number|email)":"[^*\[]' <log-stream>` retorna vazio.
- Unit tests do `mask` em CI; cobertura 100%.

## Impacto em Documentação e Operação

- `internal/identity/AGENTS.md` — seção "Logging de PII".
- Runbook futuro de incident response sobre vazamento referencia esta ADR como contrato em vigor.

## Revisão Futura

Reavaliar se LGPD/DPO demandar hashing determinístico (ADR-004-v2) para correlação cross-log sem PII em claro.
