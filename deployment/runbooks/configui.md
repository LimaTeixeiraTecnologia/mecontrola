# Runbook: Config UI (interface visual de config/secrets)

Ferramenta local para visualizar e editar `deployment/config/prod.env` e `deployment/config/prod.secrets.env` via navegador.

## Por que usar

- Ver nomes e valores dos secrets sem lidar com `sops` no terminal.
- Editar configurações não-secretas e secrets em uma interface simples.
- Re-criptografar `prod.secrets.env` automaticamente ao salvar.

## Segurança

- Roda por padrão em `127.0.0.1:8080` (apenas localhost).
- Exige autenticação básica (`admin` + senha bcrypt).
- A chave age (`SOPS_AGE_KEY` ou `SOPS_AGE_KEY_FILE`) fica na memória do processo; nunca é logada.
- Em produção/VPS, sempre coloque atrás de Caddy/HTTPS e restrinja por IP/VPN.

## Pré-requisitos

- `sops` e `age-keygen` instalados.
- Arquivo `.sops.yaml` configurado com a chave pública age.
- `deployment/config/prod.secrets.env` existente (pode estar apenas com placeholders criptografados).
- Variável `SOPS_AGE_KEY` ou `SOPS_AGE_KEY_FILE` exportada.

## Gerar hash de senha

```bash
# via Task
task -t taskfiles/local.yml configui:hash-password

# via Go
go run ./cmd/configui --hash-password
```

Cole o hash em `CONFIG_UI_PASSWORD_HASH`.

## Iniciar localmente

```bash
export SOPS_AGE_KEY="$(cat key.txt)"
export CONFIG_UI_PASSWORD_HASH="$hash_gerado"

# via Task
task -t taskfiles/local.yml configui:run

# via Go
go run ./cmd/configui
```

Acesse `http://localhost:8080` e faça login com usuário `admin` e a senha correspondente.

## Variáveis de ambiente

| Variável | Descrição | Padrão |
|---|---|---|
| `CONFIG_UI_REPO_DIR` | Diretório raiz do repositório. | `.` |
| `CONFIG_UI_ADDR` | Endereço de bind do servidor. | `127.0.0.1:8080` |
| `CONFIG_UI_PASSWORD_HASH` | Hash bcrypt da senha de acesso. | senha temporária impressa no stderr |
| `CONFIG_UI_TEMPLATE` | Caminho opcional para template HTML customizado. | embutido |
| `SOPS_AGE_KEY` / `SOPS_AGE_KEY_FILE` | Chave privada age para descriptografar/criptografar secrets. | — |

## Deploy na VPS (opcional)

Não recomendado deixar a UI permanentemente exposta. Se necessário:

1. Faça build do binário:
   ```bash
   GOOS=linux GOARCH=amd64 go build -o bin/configui ./cmd/configui
   ```

2. Copie para a VPS e execute com bind restrito a localhost + túnel SSH:
   ```bash
   ssh -L 8080:localhost:8080 user@vps
   ./bin/configui
   ```

3. Ou coloque atrás do Caddy com autenticação básica do Caddy e restrição por IP.

## Cuidados

- Nunca commit a senha em plaintext.
- Ao remover uma chave de secrets no navegador, ela será removida permanentemente do arquivo criptografado.
- Comentários em `prod.secrets.env` podem não ser preservados após edição via UI.
