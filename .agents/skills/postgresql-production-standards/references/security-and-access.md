# Seguranca e Controle de Acesso

## Fontes Oficiais
- Client authentication: https://www.postgresql.org/docs/current/client-authentication.html
- Role attributes: https://www.postgresql.org/docs/current/role-attributes.html
- Privileges: https://www.postgresql.org/docs/current/ddl-priv.html
- Row security policies: https://www.postgresql.org/docs/current/ddl-rowsecurity.html
- Predefined roles: https://www.postgresql.org/docs/current/predefined-roles.html

## Regras Mandatorias
- Aplicar least privilege por role funcional; separar owner de schema, role de migracao, role de aplicacao e role de leitura quando o projeto exigir.
- Conceder `LOGIN` apenas a roles que realmente autenticam.
- Evitar superuser para aplicacao, jobs e observabilidade.
- Restringir `GRANT` ao menor escopo necessario: schema, tabela, sequencia, funcao ou coluna quando aplicavel.
- Usar RLS apenas quando houver requisito observavel de isolamento por linha; quando usar, exigir politicas completas para `SELECT`, `INSERT`, `UPDATE` e `DELETE` conforme o caso real.
- Tratar autenticacao, secrets e regras de acesso como parte do design; nao como detalhe de deploy.

## Bloqueios Obrigatorios
- Bloquear recomendacao de privilegio amplo sem justificar a operacao que depende dele.
- Bloquear RLS sem mapear o ator, o predicado de acesso e o impacto em consultas administrativas ou de manutencao.
- Bloquear recomendacao que exija superuser ou `BYPASSRLS` sem base objetiva.

## Evidencia Minima
- Roles atuais ou pretendidas.
- Operacoes que cada ator precisa executar.
- Requisitos de isolamento de dados.
- Ambiente de autenticacao e provisao de segredos.
