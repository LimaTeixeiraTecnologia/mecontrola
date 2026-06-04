# ADR-005 — Admin seed via migration `0004_identity_admin_seed` com `current_setting('app.admin_whatsapp_numbers')`

## Metadados

- **Título:** Bootstrap dos 1–2 admins iniciais via migration idempotente lendo configuração de sessão
- **Data:** 2026-06-03
- **Status:** Aceita
- **Decisores:** Engenharia + Produto (autor do PRD)
- **Relacionados:** PRD (SQ-03, RF-08, HU-05), techspec §Modelos de Dados §0004

## Contexto

`users.is_admin` é coluna boolean default `false`. SQ-03 declara que o mecanismo de promoção dos 1–2 admins iniciais fica para a techspec. Três caminhos avaliados:

1. Migration SQL idempotente lendo env var.
2. Subcomando CLI `mecontrola admin grant <number>`.
3. Operação manual `psql` documentada em runbook.

A migration foi a opção recomendada e aceita. Falta detalhar **como** uma migration SQL acessa uma env var. Postgres não lê `os.Environ`, mas suporta `SET LOCAL app.<key> = '<value>'` em sessão e `current_setting('app.<key>', true)` para leitura tolerante a ausência. O bootstrap do binário pode rodar `SET LOCAL` antes de chamar `migrate.Up()`.

Limitação relevante: `migrate.Up()` abre suas próprias conexões/transações. `SET LOCAL` precisaria ser feito dentro da transação da migration. Solução: usar `set_config('app.admin_whatsapp_numbers', $1, false)` dentro da própria migration via parâmetro injetado, OU usar `ALTER DATABASE ... SET app.admin_whatsapp_numbers = ...` antes de aplicar migrations.

## Decisão

Adotar abordagem em duas etapas executadas no bootstrap do migrator:

1. Antes de chamar `database.RunMigrations`, executar via pool gerenciado:
   ```sql
   ALTER DATABASE current_database() SET app.admin_whatsapp_numbers = $1;
   ```
   Onde `$1` é o CSV vindo da env `ADMIN_WHATSAPP_NUMBERS`. `ALTER DATABASE ... SET` persiste o parâmetro para todas as sessões futuras desse banco, então a sessão do migrator (que abre depois) já encontra o valor via `current_setting`.

2. A migration `0004_identity_admin_seed.up.sql` consome via `current_setting('app.admin_whatsapp_numbers', true)`:
   ```sql
   DO $$
   DECLARE
       raw   TEXT := current_setting('app.admin_whatsapp_numbers', true);
       parts TEXT[];
       nbr   TEXT;
   BEGIN
       IF raw IS NULL OR raw = '' THEN
           RAISE NOTICE 'identity: ADMIN_WHATSAPP_NUMBERS vazio — nenhum admin promovido';
           RETURN;
       END IF;
       parts := string_to_array(raw, ',');
       FOREACH nbr IN ARRAY parts LOOP
           UPDATE users
              SET is_admin = true, updated_at = now()
            WHERE whatsapp_number = trim(nbr)
              AND deleted_at IS NULL;
       END LOOP;
   END $$;
   ```

3. A migration é idempotente: `UPDATE` só promove existentes; números ausentes ficam para promoção pós-onboarding (operador pode rerodar a migration ou aplicar UPDATE manual).

4. `internal/platform/database` ganha helper `SetAdminWhatsAppNumbers(ctx, mgr, csv string) error` que executa o `ALTER DATABASE ... SET` antes do `RunMigrations`. Helper chamado no bootstrap apenas quando a env var existir.

5. Down (`0004_identity_admin_seed.down.sql`) executa `UPDATE users SET is_admin = false WHERE whatsapp_number IN (...)` invertendo, ou apenas `RAISE NOTICE 'no-op'` (decisão pragmática: no-op é seguro porque admin pode ser revogado por outro fluxo).

## Alternativas Consideradas

- **Subcomando CLI `mecontrola admin grant <number>`** — Vantagens: mais flexível em produção. Desvantagens: exige código de CLI e wiring de DI antes de qualquer admin existir; não rastreável em VCS. Rejeitada — pode entrar em PRD operacional futuro sem prejudicar a fundação.
- **`UPDATE` manual via psql documentado em runbook** — Vantagens: zero código. Desvantagens: sem reprodutibilidade, sem audit trail; fácil esquecer em staging/prod novo. Rejeitada.
- **Tabela `admin_seed_config(whatsapp_number text PK)` populada por seed file** — Vantagens: rastreável em código. Desvantagens: adiciona tabela só-para-bootstrap, mistura schema com config. Rejeitada por ruído arquitetural.
- **Variável de ambiente lida pela aplicação em runtime sem migration** — A app marcaria como admin durante login. Mas RT-06 diz "sem login adicional"; o canal é WhatsApp. Rejeitada — sem ponto de "login" para hook.

## Consequências

### Benefícios Esperados

- Rastreabilidade total: migration versionada no VCS; CSV em env (não em código).
- Idempotente: rerodar a migration não muda nada quando já promovido.
- Funciona em staging/prod com o mesmo mecanismo; CI usa env vazia → no-op silencioso.

### Trade-offs e Custos

- `ALTER DATABASE ... SET` persiste o parâmetro até nova chamada — em rollback total seria preciso limpar com `ALTER DATABASE current_database() RESET app.admin_whatsapp_numbers`. Helper deve oferecer essa opção.
- Operador precisa rodar a migration novamente quando o usuário admin for criado via fluxo de onboarding (não-bloqueante: admin pode ser promovido por UPDATE manual também).

### Riscos e Mitigações

- **Risco:** Env var vazia em staging/prod → admin nunca promovido silenciosamente.
- **Mitigação:** `RAISE NOTICE` na migration; bootstrap loga "ADMIN_WHATSAPP_NUMBERS não configurado" como `slog.Warn`.
- **Risco:** Operador define número errado ou mal formatado.
- **Mitigação:** Os números no env precisam estar em E.164 canônico (`+55DD9NNNNNNNN`). Validar formato no bootstrap antes de chamar `ALTER DATABASE` — rejeitar se algum não casar regex.
- **Risco:** Múltiplos bancos compartilhando cluster com `ALTER DATABASE` vazando entre tenants.
- **Mitigação:** Comando usa `current_database()` — escopo ao banco corrente; sem efeito cross-DB.

## Plano de Implementação

1. Criar `migrations/0004_identity_admin_seed.up.sql` (DO block acima).
2. Criar `migrations/0004_identity_admin_seed.down.sql` com `RAISE NOTICE` (no-op explícito).
3. Adicionar `internal/platform/database/admin_seed.go`:
   ```go
   func SetAdminWhatsAppNumbers(ctx context.Context, mgr *Manager, csv string) error
   ```
   Valida cada token contra regex `^\+55\d{11}$`, executa `ALTER DATABASE current_database() SET app.admin_whatsapp_numbers = $1`.
4. No bootstrap (`cmd/mecontrola/.../boot.go` ou similar):
   ```go
   if csv := os.Getenv("ADMIN_WHATSAPP_NUMBERS"); csv != "" {
       if err := database.SetAdminWhatsAppNumbers(ctx, mgr, csv); err != nil { return err }
   }
   if err := database.RunMigrations(ctx, mgr); err != nil { return err }
   ```
5. Integration test valida: env CSV com 2 números → 2 rows em `users` ficam com `is_admin=true` após `RunMigrations`.

## Monitoramento e Validação

- Log de bootstrap mostra `slog.Info("identity: ADMIN_WHATSAPP_NUMBERS aplicado", slog.Int("count", n))`.
- `SELECT count(*) FROM users WHERE is_admin = true` em staging/prod após primeiro deploy.

## Impacto em Documentação e Operação

- `.env.example` ganha `ADMIN_WHATSAPP_NUMBERS=+5511988887777,+5521977776666`.
- Runbook `docs/runbooks/identity-admin-seed.md` (criar) descreve troca de admin.

## Revisão Futura

Reavaliar quando houver fluxo administrativo no painel web (FE-05) com promoção via UI — migration pode virar legado.
