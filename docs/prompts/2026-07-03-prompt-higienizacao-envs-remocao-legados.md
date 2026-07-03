# 2026-07-03 — Prompt enriquecido para higienização de arquivos `.env`

## Prompt original

> Eu quero higienizar todas .env, analisar no codebase o que realmente são utilizadas e remover as legadas de forma efetiva, robusto, economico, eficiente, 0 gaps, 0 falso positivo, 0 ressalvas, 0 lacunas.

## Ambiguidades eliminadas

| Ponto | Decisão aplicada no prompt |
|---|---|
| O objetivo é só analisar ou também executar? | O prompt enriquecido manda implementar a higienização completa, não parar em diagnóstico. |
| Quais arquivos entram no escopo? | O prompt fecha o escopo sobre todos os env-like files reais encontrados no repositório e no working tree local. |
| Como evitar falso positivo? | O prompt exige inventário por variável com evidência concreta de uso antes de remover. |
| O que fazer com fixtures e exemplos? | O prompt diferencia runtime, deploy, secrets, examples, docs e test fixtures para não apagar arquivo válido por engano. |
| Qual ponto de partida usar no codebase? | O prompt obriga partir de `cmd/server/server.go` e `cmd/worker/worker.go`, além do loader em `configs/config.go`. |

## Prompt enriquecido — versão pronta para uso

```text
Atue como um engenheiro sênior de Go, configuração de aplicação, segurança operacional e higiene de codebase. Execute a higienização completa dos arquivos `.env` e correlatos deste repositório, analisando o estado real do working tree atual para identificar o que de fato é utilizado, remover variáveis legadas, eliminar arquivos de ambiente obsoletos e deixar a configuração enxuta, correta e sem drift.

Você deve IMPLEMENTAR a limpeza. Não pare em análise, plano ou sugestão.

Contexto mandatário do repositório:
- Repositório: `mecontrola`
- Stack principal: Go `1.26.4`
- Arquitetura: monólito modular em Go
- Fonte canônica de regras: `AGENTS.md`
- Ponto de partida obrigatório da investigação: `cmd/server/server.go` e `cmd/worker/worker.go`
- Arquivo central de carga de configuração: `configs/config.go`
- O loader usa `viper`, carrega `.env` fora de produção, aplica `AutomaticEnv()` e em produção também tenta resolver secrets em arquivos via `MECONTROLA_SECRETS_PATH` com fallback para `/run/secrets`
- Existe um `cmd/configui/main.go` que manipula `deployment/config/prod.env` e `deployment/config/prod.secrets.env`

Arquivos env-like já evidenciados no workspace:
- `.env`
- `.env.example`
- `.env.prod`
- `deployment/config/prod.env`
- `deployment/config/prod.secrets.env`
- `deployment/config/prod.secrets.env.example`
- `configs/testdata/valid/.env`
- `configs/testdata/insecure-prod/.env`

Restrições mandatórias:
1. Leia e siga `AGENTS.md` antes de qualquer alteração.
2. Carregue obrigatoriamente `.agents/skills/agent-governance/SKILL.md`.
3. Como haverá alteração em código/documentação/configuração de um projeto Go, carregue também `.agents/skills/go-implementation/SKILL.md`.
4. Trabalhe em cima do estado atual do working tree. Não assuma contexto fora do repositório.
5. Não use `internal/platform/runtime` como ponto de partida.
6. Não invente variáveis, fluxos, arquivos, consumers, jobs, providers ou integrações ausentes.
7. Não trate como “usada” uma variável que só aparece em exemplo, comentário, README antigo ou arquivo legado sem consumidor real.
8. Não trate como “legada” uma variável que tenha consumidor real em runtime, testes, deploy, rotação de segredo, fallback operacional ou fixture obrigatória.
9. Se uma variável não fizer mais sentido no escopo real, remova por completo; não deixe campo morto, placeholder inútil, comentário deprecado, compatibilidade ociosa ou drift silencioso.
10. Nunca exponha valores de segredo na resposta, logs, diff explicado ou documentação criada. Só trate nomes de variáveis e arquivos.
11. Se um arquivo `.env*` inteiro estiver obsoleto e sem função real, remova o arquivo por completo.
12. Preserve fixtures versionadas necessárias para testes, mesmo que tenham extensão `.env`.
13. Preserve e mantenha coerentes os fluxos de secrets criptografados via SOPS/age quando ainda forem realmente usados.
14. Zero falso positivo: toda remoção deve estar sustentada por evidência objetiva de não uso.
15. Zero lacunas: ao final, não pode sobrar variável órfã, documentação contraditória, exemplo desatualizado ou código lendo env legado sem necessidade.

Escopo exato da investigação:
1. Mapear todos os pontos de entrada e bootstrap que consomem configuração, começando obrigatoriamente por:
   - `cmd/server/server.go`
   - `cmd/worker/worker.go`
   - `configs/config.go`
   - `cmd/configui/main.go`
2. Mapear toda leitura de ambiente e toda variável configurável via:
   - `viper`
   - `os.Getenv`
   - `os.LookupEnv`
   - qualquer helper/wrapper equivalente
   - documentação operacional que declare envs obrigatórias
3. Revisar todos os arquivos `.env*`, exemplos, templates, runbooks, scripts, workflows e docs que declarem ou dependam de variáveis de ambiente.
4. Verificar diferenças entre:
   - variáveis declaradas nos arquivos `.env*`
   - variáveis listadas em `configs/config.go`
   - variáveis lidas diretamente no código fora do loader central
   - variáveis usadas apenas em testes/fixtures
   - variáveis usadas apenas em deploy/criptografia/configuração operacional
5. Validar também referências em:
   - `README.md`
   - `.sops.yaml`
   - `.gitignore`
   - workflows CI/CD
   - scripts de deploy/setup
   - testes que dependam explicitamente de env vars

Objetivo operacional:
Deixar o repositório com uma superfície de configuração mínima, correta e comprovada, onde:
- toda variável existente tenha consumidor real ou papel operacional explícito e necessário;
- toda variável legada seja removida do código, dos `.env*`, dos exemplos e da documentação;
- todo arquivo `.env*` tenha finalidade clara;
- não exista drift entre loader, exemplos, deploy, docs e uso real no código.

Como executar:
1. Monte um inventário autoritativo de todas as variáveis encontradas nos arquivos `.env*` e nos pontos de leitura do código.
2. Classifique cada variável em uma e apenas uma categoria:
   - runtime real
   - secret real de produção/deploy
   - usada apenas em testes/fixtures
   - usada apenas em ferramenta local
   - usada apenas em documentação/exemplo
   - legada/sem uso real
3. Para cada variável, anote evidência exata de uso ou de ausência de uso com arquivo e linha.
4. Cruze obrigatoriamente esse inventário com:
   - `envKeys()` e `secretEnvKeys()` em `configs/config.go`
   - leituras diretas com `os.Getenv`/equivalentes
   - arquivos `deployment/config/*`
   - exemplos `.env.example`
   - docs e runbooks
5. Remova de forma efetiva:
   - variáveis legadas dos arquivos `.env*`
   - referências legadas do código
   - documentação desatualizada
   - exemplos que mantêm chaves sem consumidor real
   - arquivos env inteiros que não tenham mais função
6. Se existir código exclusivamente para suportar env legada, remova esse código também, desde que a remoção seja segura e comprovada.
7. Se existir env usada apenas por teste ou fixture, preserve apenas onde ela é necessária e mantenha isso consistente com os testes reais.
8. Se existir env opcional de rotação/fallback ainda suportada pelo fluxo atual (por exemplo chaves `*_NEXT` ou secrets lidos de arquivo em produção), preserve somente se houver evidência concreta no código ou no fluxo operacional atual.
9. Atualize exemplos e documentação para refletirem exatamente o estado final correto, sem excesso e sem ausência.
10. Ao terminar, garanta que nenhum consumidor real ficou sem variável necessária e que nenhuma variável sem consumidor real permaneceu no repositório.

Critérios de aceitação obrigatórios:
1. Todo arquivo `.env*` do repositório e do working tree local relevante foi analisado.
2. Toda variável presente em qualquer `.env*` foi classificada com evidência objetiva.
3. Nenhuma variável legada permaneceu em:
   - código
   - exemplos
   - deploy
   - docs
   - fixtures desnecessárias
4. Nenhuma variável realmente usada foi removida por engano.
5. `configs/config.go`, leituras diretas de env, exemplos e docs ficaram coerentes entre si.
6. Se um arquivo env inteiro era legado, ele foi removido.
7. Se um arquivo env continua existindo, sua finalidade ficou clara e sustentada pelo código ou operação real.
8. Fluxos de produção que dependem de secrets por arquivo (`/run/secrets` / `MECONTROLA_SECRETS_PATH`) continuam consistentes com o estado final.
9. Fluxos de `cmd/configui` continuam coerentes com os arquivos de deploy realmente suportados.
10. O resultado final não contém TODO, TBD, “avaliar depois”, compatibilidade morta, comentário enganoso nem ressalva vaga.

Formato obrigatório da sua resposta final:
1. **Resumo do que foi higienizado**
2. **Tabela de inventário final** com colunas:
   - variável
   - categoria
   - arquivos onde aparecia
   - evidência de uso real
   - ação tomada
3. **Arquivos alterados/removidos**
4. **Riscos evitados / drifts eliminados**
5. **Comandos de validação executados**

Regras de qualidade da execução:
1. Faça a menor mudança segura que resolva por completo a causa raiz.
2. Não aceite meia-limpeza.
3. Não preserve variável “por via das dúvidas”.
4. Não remova variável “por aparência”.
5. Toda decisão precisa ser ancorada em evidência do codebase atual.
6. Se houver conflito entre docs antigas e código atual, o working tree atual prevalece.
7. Se houver trade-off real, escolha a opção mais segura e mais simples de manter.
8. Seja rigoroso com segurança: trate nomes, nunca valores secretos.
9. Seja econômico: elimine redundância real, não só ruído visual.
10. Entregue o trabalho pronto, com remoção efetiva dos legados e sem gaps.
```

## O que foi adicionado e por quê

| Adição | Justificativa |
|---|---|
| Escopo fechado com os arquivos `.env*` já detectados | Reduz ambiguidade e força cobertura completa do repositório real. |
| Ponto de partida obrigatório em `cmd/server/server.go`, `cmd/worker/worker.go` e `configs/config.go` | Alinha o prompt ao bootstrap real do projeto e evita investigação fora da composição vigente. |
| Separação entre runtime, deploy, secret, fixture, exemplo e legado | Minimiza falso positivo em remoções de envs que ainda têm papel operacional. |
| Exigência de inventário por variável com evidência | Impede limpeza por suposição e força rastreabilidade da decisão. |
| Regra explícita para remover código/documentação que só sustentava env legado | Fecha a causa raiz e evita sobrar compatibilidade morta. |
| Proteção contra exposição de segredos | Mantém o prompt seguro para uso prático em ambiente real. |
| Critérios de aceitação verificáveis | Aumenta a chance de execução completa, sem lacunas nem resposta superficial. |

## Variante compacta

Use apenas se quiser uma versão menor e aceitar menos contexto inline:

```text
Leia `AGENTS.md`, carregue `.agents/skills/agent-governance/SKILL.md` e `.agents/skills/go-implementation/SKILL.md`, e execute a higienização completa dos arquivos `.env*` deste repositório.

Parta obrigatoriamente de `cmd/server/server.go`, `cmd/worker/worker.go`, `configs/config.go` e `cmd/configui/main.go`. Analise o working tree atual, inventarie todas as variáveis de ambiente, prove com evidência quais são realmente usadas e remova de forma efetiva tudo que for legado no código, nos `.env*`, nos exemplos, no deploy e na documentação.

Considere explicitamente:
- `.env`
- `.env.example`
- `.env.prod`
- `deployment/config/prod.env`
- `deployment/config/prod.secrets.env`
- `deployment/config/prod.secrets.env.example`
- `configs/testdata/**/.env`

Não invente contexto. Não use `internal/platform/runtime` como ponto de partida. Não preserve variável “por via das dúvidas” e não remova variável “por aparência”. Preserve apenas o que tiver consumidor real ou papel operacional comprovado. Se algo for legado, remova por completo, inclusive código/documentação de suporte, sem deixar drift.

Entregue:
1. resumo do que foi higienizado;
2. tabela por variável com categoria, evidência e ação tomada;
3. arquivos alterados/removidos;
4. drifts eliminados;
5. comandos de validação executados.
```
