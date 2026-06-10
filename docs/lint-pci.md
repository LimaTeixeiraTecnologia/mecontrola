# Lint anti-PCI (RF-16)

## Proposito

A aplicacao `mecontrola` e **nao-PCI**: nao persiste, transita nem expoe dados
sensiveis de cartao (PAN, CVV/CVC, trilha magnetica, PIN block). O modulo `card`
armazena apenas metadados nao-sensiveis (apelido, bandeira, ultimos 4 digitos,
dia de fechamento/vencimento).

O gate `task lint:pci` e um validador estatico determinista que bloqueia a
introducao de identificadores ou strings que sugiram coleta desses dados em
codigo de producao. Ele complementa a regra `forbidigo` no `.golangci.yml`
(escopo restrito a `internal/card/`) ampliando a cobertura para todo o
repositorio e atuando em `*.go`, `*.sql`, `*.yaml`, `*.yml` e `*.json`.

## Como rodar

```
task lint:pci
```

Saida esperada em arvore limpa:

```
PASS lint:pci: nenhum termo PCI detectado em codigo de producao
```

Em violacao, a task lista linha por linha cada ocorrencia e retorna exit
status nao-zero.

## Escopo

Diretorios analisados:

- `internal/`
- `migrations/`
- `docs/`
- `cmd/`
- `configs/`

Diretorios ignorados:

- `mocks/`, `vendor/`, `node_modules/`, `.git/`

Arquivos `*_test.go` sao excluidos para evitar ruido em fixtures e testes de
contrato — testes nao executam em producao.

## Padroes bloqueados

Casing-insensitive, com word-boundary (`\b`) para evitar falsos positivos em
palavras comuns como `pinPoint`, `underpinned` ou `transactionPan`.

| Categoria | Tokens |
|-----------|--------|
| Primary Account Number | `PAN`, `pan_number`, `cardholder_pan`, `primary_account_number`, `card_number`, `cardNumber`, `cc_number`, `ccNumber` |
| Codigos de seguranca | `cvv`, `cvc`, `cvv2`, `cvc2`, `cvv_code`, `cvc_code` |
| Trilha magnetica | `track1`, `track2`, `track_data`, `magstripe` |
| PIN | `pin_block`, `pinBlock`, `cardholder_pin`, `cardholderPin` |

Tokens comuns ambiguos (`pin`, `pan`, `card` isolados) **nao** sao bloqueados
para evitar ruido — sempre exigem qualificador inequivoco.

## Politica de excecao

Nenhuma. Qualquer hit deve ser corrigido por uma destas vias:

1. Renomear o identificador (preferencial).
2. Remover o campo/coluna se nao for necessario.
3. Abrir ADR explicito que justifique a necessidade e o controle compensatorio
   (criptografia, tokenizacao, segregacao de rede). Sem ADR aprovado, o gate
   permanece bloqueando.

## Integracao com CI

A task roda no job `lint` do `.github/workflows/ci.yml`, garantindo que toda
PR contra `main` seja avaliada antes do merge.

## Referencias

- PRD: `.specs/prd-card-crud-mvp/prd.md` — RF-16.
- ADR-005 (migrations rollback) e techspec do modulo `card`.
- Regra forbidigo correspondente em `.golangci.yml` (escopo `internal/card/`).
