# ADR-002 — Magic token: 32 bytes `crypto/rand` → base64url + persistência SHA-256

## Metadados

- **Título:** Formato e persistência segura do magic token
- **Data:** 2026-06-06
- **Status:** Aceita
- **Decisores:** PO (jailton), arquitetura (AI)
- **Relacionados:** `.specs/prd-onboarding-magic-token/techspec.md` §8.1, RF-01, RF-15, restrição "Token opaco e não enumerável" do PRD; [ADR-001](./adr-001-onboarding-schema-dedicated-and-tokens-table.md)

## Contexto

O PRD exige token "opaco e não enumerável", carregado em URL (`?sck={token}`), seguro contra enumeração e contra reuso indevido. O token transita por URL pública (checkout Kiwify, redirect, deep link `wa.me`) e fica visível em logs de CDN/proxy se mal logado.

Espaço de candidatos:
- UUIDv4 (122 bits efetivos).
- UUIDv7 (timestamp + random — vaza ordem temporal, ruim para enumeração).
- 16 bytes `rand` → base64url (~22 chars).
- 32 bytes `rand` → base64url (43 chars).
- 32 bytes `rand` → base58 (~44 chars).

Persistência: armazenar em claro é vulnerável a leak de dump; bcrypt é overkill para token de alta entropia; SHA-256 é o padrão da indústria.

## Decisão

**Geração:**
- `crypto/rand.Read(buf [32]byte)` → 256 bits.
- Encoding `base64.RawURLEncoding.EncodeToString` (URL-safe, sem padding, 43 chars).
- Forma final: `Tk_p3w...` (regex no inbound: `(?i)^\s*ATIVAR\s+([A-Za-z0-9_\-]{40,45})\s*$`).

**Persistência:**
- `sha256.Sum256([]byte(token))` → 32 bytes raw em coluna `token_hash BYTEA`.
- Token claro **nunca** persistido nem logado.
- Logs incluem apenas `token_hash_prefix = hex(token_hash)[:8]` (8 chars hex) para diagnóstico.
- Mensagens recebidas do WhatsApp logam `text` com substituição regex `ATIVAR \S+` → `ATIVAR ****`.

**Comparação:**
- Borda calcula `sha256.Sum256([]byte(input_token))` e faz lookup por `token_hash = ?`.
- Lookup O(1) por unique index.

## Alternativas Consideradas

1. **UUIDv4 (16 bytes).** Recusada — 122 bits efetivos é suficiente, mas perde 6 bits de entropia para nada e cria expectativa de "é um UUID" que não é o caso (token não é identificador, é credencial).
2. **UUIDv7.** Recusada — timestamp embutido vaza horário de criação; pode ajudar enumeração por janela.
3. **16 bytes `rand`.** Recusada — 128 bits é seguro mas margem menor; 32 bytes alinha com prática de session tokens.
4. **base58.** Recusada — não nativo na stdlib; ganho de UX (sem `-`/`_`) marginal para um token que não é digitado.
5. **bcrypt na persistência.** Recusada — overkill para alta entropia; lookup O(N).

## Consequências

### Benefícios
- 256 bits de entropia → enumeração não factível.
- URL-safe sem encoding extra.
- Hash SHA-256 protege contra dump de DB.
- Padrão consolidado da indústria.

### Trade-offs
- 43 caracteres é mais longo que UUID (mais texto no copy-paste manual — RF-04). Aceitável: caminho feliz usa botão `wa.me`.
- Persistência como `BYTEA` exige conversão; `hex(token_hash)[:8]` para logs.

### Riscos e Mitigações
- **R:** Implementação que loga token em claro por engano. **M:** Lint de string `token` em logs durante code review; teste unitário que captura output de log e valida ausência de token claro.
- **R:** Geração com `math/rand` em vez de `crypto/rand`. **M:** Test unitário verifica que `Token.Value` produz tokens distintos com alta probabilidade (smoke); revisão obrigatória.
- **R:** Coluna `BYTEA` em SELECT acidental imprime hex que pode aparecer em log. **M:** Repositories sempre retornam VOs; nada de raw struct exposed.

## Plano de Implementação
1. `domain/valueobjects/token.go` com `NewToken()`, `Hash()` (SHA-256 raw 32B), `String()` redacted.
2. Test unitário com 10k iterações validando unicidade.
3. Test que captura `slog` e valida ausência de token em claro.

## Monitoramento
N/A — token format não é dimensão observável.
