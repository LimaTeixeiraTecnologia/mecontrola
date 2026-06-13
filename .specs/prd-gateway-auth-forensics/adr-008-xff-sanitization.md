# ADR-008 — Sanitização de X-Forwarded-For para client_ip em auth_events

## Metadados

- **Título:** Pegar o último IP de X-Forwarded-For sob a premissa de Caddy como único proxy confiável
- **Data:** 2026-06-12
- **Status:** Aceita
- **Decisores:** Operador do mecontrola
- **Relacionados:** [PRD](prd.md) RF-17; [techspec](techspec.md) seção Modelos de Dados; plano-fonte A5 + B3

## Contexto

A coluna `client_ip INET` em `auth_events` precisa ser populada com o IP do client real. O servidor Go não vê o IP TCP do client (vê o IP do Caddy). A informação chega via header `X-Forwarded-For` (ou `X-Real-IP`).

XFF é um header controlado por proxy. Confiar nele depende da arquitetura:
- Se o app aceita XFF arbitrário, atacante forja o IP que quiser.
- Se o app confia apenas no proxy de borda, a sanitização correta é pegar o IP que **o próprio Caddy escreveu** (não o que o cliente alegou).

Topologia real:
- Internet → Caddy → app Go (mesma rede backend interna).
- Caddy é o único proxy confiável.

## Decisão

**Extração de `client_ip`:**

1. Ler header `X-Forwarded-For`.
2. Se vazio, `client_ip = NULL`.
3. Se presente, split por vírgula, trim de espaços. Pegar o **último elemento** da lista.
4. Validar com `net.ParseIP`. Se inválido (parseIP retorna nil), `client_ip = NULL` + log warn.
5. Se válido, persistir como `INET`.

**Por que o último?** Sob a premissa "Caddy é o único proxy", o Caddy sempre **append**a o IP TCP do peer que ele vê. A lista resultante é `[cliente_alegado, ..., proxies_intermediários, ip_real_do_peer_visto_pelo_Caddy]`. Pegar o último ignora qualquer XFF forjado pelo cliente.

Sem `X-Real-IP` para evitar dependência de configuração específica de Caddy (item B3 do plano-fonte). XFF é universal.

## Alternativas Consideradas

1. **Primeiro IP da lista (cliente alegado)** — comum mas inseguro. **Rejeitada**: cliente controla o início da lista; trivial forjar `X-Forwarded-For: 1.2.3.4` em request direto e o app aceitaria.
2. **Header customizado `X-Real-IP` do Caddy** — Caddy injeta com IP do peer TCP. **Rejeitada como solução principal**: exige config Caddy explícita (overlap com B3). Aceitamos como **fallback opcional pós B3**: se `X-Real-IP` presente e XFF ausente, usar `X-Real-IP`.
3. **`r.RemoteAddr`** — IP do peer TCP visto pelo Go (= IP do Caddy interno). **Rejeitada**: registra IP da rede docker, não do cliente real. Inútil para forense.

## Consequências

### Benefícios Esperados

- Resiliente a XFF forjado por cliente.
- Não depende de header customizado do Caddy (pode rodar antes do B3 ser aplicado).
- Falha de parse cai para `NULL` sem propagar erro — auth_events nunca perde uma linha por causa de IP malformado.

### Trade-offs e Custos

- **Premissa crítica**: se Caddy estiver mal configurado e não fizer append (e.g. modo "passthrough" do XFF), o último IP pode ser o que o cliente alegou. Mitigação: smoke test inclui verificação do header gerado pelo Caddy.
- `NULL` aceito em larga escala mascararia mau funcionamento do Caddy. Mitigação: métrica `auth_events_with_null_client_ip` se ultrapassar 5% de inserts, alertar.

### Riscos e Mitigações

- **R-01**: Caddy reconfigurado por engano e passa a confiar em XFF do cliente. **Mitigação**: smoke test scriptado verifica que `X-Forwarded-For` injetado pelo Caddy sempre adiciona o IP do peer real ao final.
- **R-02**: cliente IPv6 com formato inválido cai para NULL silenciosamente. **Mitigação**: log warn estruturado dispara alerta se taxa de NULL > 5%.

## Plano de Implementação

1. Função pura em `internal/identity/domain/valueobjects/client_ip.go`:
   ```go
   func NewClientIP(xForwardedFor string) (ClientIP, error)
   ```
   Retorna `ClientIP{ip net.IP}` validada ou erro. Adapter trata erro como `NULL`.
2. Adapter (middleware ou handler) lê header, chama `NewClientIP`, propaga para use case.
3. Use case `EstablishPrincipal` recebe `RequestID` e `ClientIP` como inputs.
4. Teste tabela: XFF vazio → NULL; XFF "1.2.3.4" → IP válido; XFF "evil, 1.2.3.4" → último (1.2.3.4); XFF "evil" → NULL; XFF "1.2.3.4, ::1" → último (::1).

## Monitoramento e Validação

- Métrica `auth_events_with_null_client_ip_total` (counter).
- Alerta: razão `null_total / total > 0.05` por 30 min → operador.

## Impacto em Documentação e Operação

- `docs/runbooks/gateway-auth.md`: explicar a premissa "último IP é o real" e validação Caddy.
- Item B3 do plano-fonte (Caddyfile hardening): confirmar que Caddy faz append correto.

## Revisão Futura

Revisar quando:
- Houver mais de um proxy entre Caddy e Internet (CDN, WAF) — recontar a posição na lista.
- Migrar para `X-Real-IP` como primário pós B3.
- Data sugerida: 2027-06-12.
