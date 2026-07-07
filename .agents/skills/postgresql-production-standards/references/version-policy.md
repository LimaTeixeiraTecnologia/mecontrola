# Politica de Versao

## Baseline Oficial
- Baseline documental da skill: `PostgreSQL 18.4 Documentation`
  Fonte: https://www.postgresql.org/docs/current/index.html
- A skill usa o manual oficial atual como baseline editorial e aceita projetos em PostgreSQL `14` a `18`.

## Resolucao Normativa de URLs
- Se a versao detectada for `18`, as URLs podem permanecer na variante `docs/current` do manual oficial.
- Se a versao detectada for `14`, `15`, `16` ou `17`, substitua `docs/current` por `docs/<major>` antes de aplicar a regra oficial.
- Nunca aplique texto normativo de `docs/current` diretamente a um projeto `14-17` sem resolver a major version correspondente.
- Quando uma secao nao existir ou divergir na major detectada, bloquear com `needs_input` em vez de extrapolar.

## Gates Mandatorios
- Se a versao observada for `14`, `15`, `16`, `17` ou `18`, continuar.
- Se a versao for inferior a `14`, bloquear por baseline fora do escopo.
- Se a versao for superior a `18`, bloquear ate confirmar compatibilidade da regra aplicada com a nova versao.
- Se a versao nao puder ser inferida com seguranca por `Dockerfile`, `docker-compose`, manifests, `SHOW server_version`, lockfiles ou configuracoes, retornar `needs_input`.

## Evidencia Aceita para Versao
- `image: postgres:<major>` ou `postgres:<major>.<minor>` em `Dockerfile`, `docker-compose*.yml` ou manifests.
- `server_version`, `SHOW server_version`, `SELECT version()`.
- Arquivos de provisionamento ou IaC que fixem a major version.
- Configuracao gerenciada que declare explicitamente a major version.

## Regras de Compatibilidade
- Nunca recomendar recurso sem confirmar que ele existe na versao observada.
- Quando a regra mudar entre majors suportadas, nomear a versao aplicavel na saida.
- Quando a versao for desconhecida e a recomendacao depender dela, bloquear.
