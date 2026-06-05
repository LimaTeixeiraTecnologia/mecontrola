# ADR-003 — Mascaramento de PII como método nos Value Objects (`Masked()`)

## Metadados

- **Título:** Forma do helper de mascaramento de PII em logs estruturados
- **Data:** 2026-06-05
- **Status:** Aceita
- **Decisores:** Time MeControla (owner: Jailton Junior)
- **Relacionados:**
  - PRD: [`prd.md`](./prd.md) — RF-14, F-10
  - Tech Spec: [`techspec.md`](./techspec.md)
  - PRD Q em aberto fechada: **Q-01**
  - Restrições: LGPD, mascaramento obrigatório antes de qualquer sink de observabilidade.

## Contexto

`WhatsAppNumber`, `Email` e `display_name` são PII. O PRD exige (RF-14) que todo log estruturado do módulo identity passe por um helper único e reutilizável de mascaramento. O working tree usa `observability.String("key", value)` como pattern de log estruturado (visível em `cmd/server/server.go` e `internal/platform/outbox`).

Dois caminhos canônicos existem:

1. **Método nos VOs** (`number.Masked()`, `email.Masked()`).
2. **Helper externo em pacote utilitário** (`mask.WhatsApp(s)`, `mask.Email(s)`).

Restrições adicionais:

- `WhatsAppNumber` e `Email` são tipos do domínio (R1: métodos de struct).
- A regra de mascaramento muda junto com a regra de validação — quem garante o formato canônico é o VO; quem mascara também precisa conhecer o formato.
- `display_name` não é VO no MVP (é `string` opcional no agregado), então precisará de tratamento separado.

## Decisão

Mascaramento será implementado como **método sem argumentos nos VOs**:

```go
// internal/identity/domain/valueobjects/whatsapp_number.go
func (w WhatsAppNumber) Masked() string { /* +55 11 9****-7777 */ }

// internal/identity/domain/valueobjects/email.go
func (e Email) Masked() string { /* j***@example.com */ }
```

Para `display_name`, que não é VO, um helper exportado em pacote separado:

```go
// internal/identity/domain/pii/mask.go
func MaskDisplayName(name string) string { /* "Jailton" -> "J******" */ }
```

Uso típico em log:

```go
r.o11y.Logger().Info(ctx, "user.upserted",
    observability.String("whatsapp", user.WhatsApp().Masked()),
    observability.String("email",    user.Email().Masked()),
    observability.String("name",     pii.MaskDisplayName(user.DisplayName())),
    observability.String("user_id",  user.ID().String()),
)
```

Regra de uso: **nenhum log estruturado do módulo identity pode receber `WhatsAppNumber.String()`, `Email.String()` ou `display_name` cru**. `golangci-lint` recebe regra `forbidigo` para alertar.

## Alternativas Consideradas

### A) Helper externo `mask.WhatsApp(s string) string`

- **Vantagens:** desacopla mascaramento do VO; permite mascarar string crua sem instanciar VO.
- **Desvantagens:**
  - Duplica conhecimento de formato (VO e helper precisam saber a estrutura E.164 BR).
  - Encoraja passar strings cruas adiante (viola RF-04: APIs internas nunca trafegam string).
  - Helper externo perde o tipo no call site (`mask.WhatsApp(s)` aceita qualquer string).
- **Motivo de não escolher:** quebra encapsulamento e a regra "VO é fonte de verdade do formato".

### B) Mascaramento implícito via `slog.LogValuer`

- **Vantagens:** `slog` chama `LogValue()` automaticamente; impossível esquecer.
- **Desvantagens:**
  - O projeto usa `observability.Logger` (devkit-go), não `slog` diretamente — `LogValuer` não é honrado nesse caminho.
  - Acopla domínio a uma decisão de framework.
- **Motivo de não escolher:** suporte do framework não está garantido; tornaria a regra invisível em sinks alternativos.

### C) Mascaramento no sink (interceptador no logger)

- **Vantagens:** uma vez configurado, vale para todo log.
- **Desvantagens:**
  - Acopla logger a tipos de domínio.
  - Falha silenciosa: se o sink mudar, PII vaza sem alerta.
- **Motivo de não escolher:** força a lógica para a camada errada.

## Consequências

### Benefícios Esperados

- **Tipo guia o uso:** `Masked()` aparece no autocomplete; esquecer de chamá-lo é evidente.
- **Formato e mascaramento em sincronia** — qualquer evolução do E.164 BR atualiza ambos no mesmo arquivo.
- **Sem dependência transversal** — pacote `domain/valueobjects` permanece puro.
- **Testabilidade trivial:** `TestWhatsAppNumber_Masked` cobre todos os formatos.

### Trade-offs e Custos

- `display_name` não é VO no MVP, então usa um helper externo em `pii/mask.go` — pequena inconsistência aceita conscientemente.
- `Masked()` cria string por chamada (no caminho de log). Custo desprezível em volume real.

### Riscos e Mitigações

- **Risco:** desenvolvedor passa `user.WhatsApp().String()` em log por hábito.
  - **Mitigação:** regra `forbidigo` em `.golangci.yml` para `internal/identity/**`:
    ```yaml
    forbidigo:
      forbid:
        - p: '\.(String)\(\)'  # com analyzer-config, restrita aos VOs PII
          msg: "use Masked() em vez de String() para PII em logs (ADR-003)"
    ```
    Alternativa: code review + teste de smoke que grepa logs por padrão `+55\d{2}9` em outputs de teste.
- **Risco:** `display_name` cru vaza por não estar atrás de VO.
  - **Mitigação:** helper `pii.MaskDisplayName` é o único caminho documentado; revisão exige mascaramento.

## Plano de Implementação

1. Implementar `WhatsAppNumber.Masked()` retornando `+55 DD 9****-NNNN` (4 dígitos finais visíveis).
2. Implementar `Email.Masked()` retornando `<primeira-letra>***@<domínio>` quando local part >= 2 chars; `<primeira-letra>***@<domínio>` mesmo para local part de 1 char (`a@x.com` → `a***@x.com`) para não revelar tamanho.
3. Implementar `pii.MaskDisplayName(name string) string` em `internal/identity/domain/pii/mask.go`:
   - Vazio → `""`.
   - 1 char → `"*"`.
   - >=2 chars → primeira letra + `"****"` (4 asteriscos fixos para não revelar comprimento).
4. Testes parametrizados em `*_test.go` cobrindo cada formato.
5. Adicionar regra `forbidigo` em `.golangci.yml`.
6. Documentar em `doc.go` o contrato de uso.

## Monitoramento e Validação

- **Validação imediata:** `go test ./internal/identity/...` + `golangci-lint run ./internal/identity/...`.
- **Smoke E2E** (CA-04): inspecionar `o11y.Logger().Info(...)` em handler/usecase/repository — nenhuma chamada deve receber string crua de PII.
- **Sinal de drift:** alerta no Loki/Datadog com regex `\+55\d{11}` em campo `whatsapp` indica vazamento.

## Impacto em Documentação e Operação

- `internal/identity/doc.go` documenta `Masked()` e `pii.MaskDisplayName`.
- `.golangci.yml` ganha regra forbidigo.
- Runbook LGPD (E4) reaproveita os helpers sem modificá-los.

## Revisão Futura

- Revisitar se `display_name` for promovido a VO (provável em E3 quando webhook do WhatsApp persistir profile.name).
- Revisitar se outros canais (e-mail transacional, NFS-e Kiwify em E2) precisarem de mascaramento próprio.
- Revisitar se o framework de logging mudar para `slog` puro (avaliar `LogValuer`).
