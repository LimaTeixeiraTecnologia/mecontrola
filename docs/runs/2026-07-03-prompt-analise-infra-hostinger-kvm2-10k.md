# Prompt Enriquecido - Analise Infra Hostinger KVM 2 ate 10k usuarios

Data de preparacao: 2026-07-03
Idioma: pt-BR
Status: PRONTO PARA USO SEM DESVIOS
Carga base: AGENTS.md lido

## Prompt original

```text
Eu quero que analise TODA infra do projeto em deployment e onde está hospedado KVM 2 https://www.hostinger.com/br (use documentações oficiais), o cenário atual é de 0 usuários ativos, mas a nossa progessão é de 10 mil usuários ativos, eu quero que analise a fundo, buscando documentações oficiais, se baseando em TODO codebase e principalmente deployment se a infra construida atende, se é eficiente e confiável, com o menor custo possível até 10 mil usuários ATIVOS, isso é inegociável,

Eu quero um relatório com plano de evolução completo, 0 gaps, 0 lacunas, 0 suposições, 0 ressalvas, 0 falso positivo e realmente production-ready/proof. não invente ou ache resposta.
```

## Ambiguidades eliminadas

- `10 mil usuarios ativos` e ambiguo. Para impedir falso positivo, avalie obrigatoriamente tres envelopes separados: `10k usuarios ativos/mes`, `10k usuarios ativos/dia` e `10k usuarios simultaneos em pico`. Nao misture os resultados.
- `infra atende` foi convertido em gates objetivos de capacidade, confiabilidade, operabilidade, seguranca, custo e recuperacao.
- `menor custo possivel` passa a significar `menor custo mensal total que ainda satisfaz todos os gates obrigatorios`. Nao aceite economia que derruba confiabilidade ou recuperacao.
- `0 gaps / 0 lacunas / production-ready` nao pode ser premissa. So pode ser conclusao se todas as afirmacoes forem comprovadas com evidencia objetiva. Se nao houver prova suficiente, o veredito obrigatorio e `nao comprovado`.

## Prompt enriquecido

```text
Voce deve executar uma analise tecnica exaustiva, read-only, orientada a evidencias e sem flexibilidade interpretativa para determinar se a infraestrutura atual do projeto, principalmente o que esta definido em `deployment/`, atende com eficiencia, confiabilidade e menor custo possivel a evolucao de 0 usuarios ativos para ate 10 mil usuarios ativos.

O objetivo NAO e implementar nada. O objetivo e produzir um relatorio production-ready/proof, totalmente auditavel, sem inventar resposta, sem preencher lacunas com opiniao e sem declarar pronto se a prova nao existir.

Mandatos inegociaveis:
1. Nao implemente, nao altere codigo, nao altere infraestrutura, nao edite manifests, nao rode comandos destrutivos, nao reinicie servicos e nao assuma resultados.
2. Toda afirmacao tecnica deve apontar evidencia concreta no codebase e, quando envolver comportamento/plataforma/componente, tambem deve citar documentacao oficial do fornecedor ou do projeto.
3. Proibido usar blog, forum, benchmark de terceiros, comparativo comercial ou achismo como base primaria. Para infraestrutura e capacidade, use somente documentacao oficial e fatos observaveis no repositorio/deployment.
4. Se algo nao puder ser comprovado, a classificacao obrigatoria e uma destas: `nao comprovado`, `gap`, `lacuna de observabilidade`, `dado ausente`, `risco residual` ou `bloqueio objetivo`.
5. Nao force conclusao positiva. `production-ready`, `0 gaps`, `0 lacunas` e `atende 10k` so podem ser declarados se todos os gates obrigatorios forem aprovados com prova objetiva.
6. Nao trate `10 mil usuarios ativos` como um conceito unico. Avalie separadamente:
   - Envelope A: `10k usuarios ativos/mes`
   - Envelope B: `10k usuarios ativos/dia`
   - Envelope C: `10k usuarios simultaneos em pico`
   Para cada envelope, diga explicitamente: `atende`, `nao atende` ou `nao comprovado`.

Fontes obrigatorias do codebase:
- `AGENTS.md`
- `go.mod`
- `README.md` com foco nas secoes de stack, Docker Swarm, deploy e backup/restore
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- todo o diretorio `deployment/`
- `.github/workflows/ci-cd.yml`
- `taskfiles/deploy.yml`
- `scripts/loadtest/README.md`
- quaisquer arquivos adicionais do repo estritamente necessarios para fechar a prova

Contexto confirmado no codebase para voce partir, mas ainda assim reconferir:
- Aplicacao principal em Go 1.26.4.
- Arquitetura de producao declarada como Docker Swarm single-node.
- Stack de producao declarada em `deployment/compose/compose.swarm.yml`.
- Servicos declarados no Swarm: `postgres`, `pgbouncer`, `postgres-exporter`, `node-exporter`, `migrate`, `server-1`, `server-2`, `worker-1`, `worker-2`, `caddy`, `otel-lgtm`, `pg-tunnel`.
- Deploy automatizado via GitHub Actions com build, lint, testes, vulncheck, build/push GHCR, scan Trivy, assinatura cosign e deploy em runner self-hosted.
- Backup de infraestrutura parcialmente suportado por Terraform AWS para bucket S3/IAM, com runbooks de restore/PITR.
- Existem scripts de load test no repositorio, mas eles NAO podem ser usados como prova suficiente de capacidade de producao sem contexto e sem correlacao com a infra real.

Especificacao informada da VPS atual, a ser tratada como dado de entrada obrigatorio e validada contra a documentacao oficial e contra a demanda real da stack:
- Sistema operacional: `Ubuntu 24.04 LTS`
- CPU: `2 nucleos`
- Espaco em disco: `100 GB`
- Largura de banda: `8 TB`

Fontes oficiais obrigatorias minimas:
- Hostinger VPS: `https://www.hostinger.com/br/servidor-vps`
- Documentacao oficial do Docker para Swarm/stack deploy/healthchecks/restart/update strategy
- Documentacao oficial do PostgreSQL 16
- Documentacao oficial do pgBouncer
- Documentacao oficial do Caddy 2
- Documentacao oficial do Grafana OTEL LGTM / OpenTelemetry Collector / Prometheus / Loki / Tempo, quando usados como base para afirmar custo operacional, retention ou observabilidade
- Documentacao oficial do GitHub Actions, GHCR e cosign quando usados para avaliar supply chain/deploy
- Documentacao oficial do Terraform e AWS S3/IAM quando usados para avaliar backup e recuperacao

Prova obrigatoria sobre o ambiente Hostinger / KVM 2:
1. Confirmar por documentacao oficial da Hostinger quais sao as capacidades publicas do plano KVM 2 no momento da analise.
2. Considerar como base minima o que a pagina oficial expuser para o KVM 2; se a pagina mostrar os dados, cite explicitamente CPU, RAM, armazenamento NVMe e franquia/banda.
3. Cruzar isso com o dado de entrada informado para a VPS atual (`Ubuntu 24.04 LTS`, `2 nucleos de CPU`, `100 GB de disco`, `8 TB de largura de banda`) e com o que o codebase efetivamente tenta alocar/rodar hoje em `compose.swarm.yml`.
4. Se nao houver evidencia documental suficiente para confirmar algum detalhe critico do plano atual, marque como `nao comprovado` e nao invente.

Escopo tecnico obrigatorio da analise:

Fase 1 - Inventario real da infraestrutura atual
1. Mapear toda a topologia declarada em `deployment/`, sem omitir servicos, jobs, volumes, redes, exposicao de portas, secrets, dependencias, health checks, estrategia de update, rollback, backup, restore e observabilidade.
2. Identificar exatamente o que roda no host principal e o que depende de servicos externos.
3. Separar claramente:
   - compute principal
   - banco
   - proxy/TLS
   - observabilidade
   - pipeline de deploy
   - supply chain
   - backup/restore
   - componentes externos obrigatorios
4. Produzir uma tabela com cada componente, funcao, dependencia, estado, persistencia, superficie de falha e arquivo fonte no repo.

Fase 2 - Orcamento objetivo de recursos
1. Somar CPU, memoria, disco, IO, rede e storage declarados nos manifests, considerando:
   - limits
   - reservations
   - replicas
   - volumes persistentes
   - consumo da stack de observabilidade
   - overhead do proprio host/OS/Docker/Swarm
2. Comparar esse orcamento com a capacidade oficial do KVM 2.
3. Nao faca conta superficial. Mostrar claramente:
   - total declarado em worst case
   - total reservado
   - total efetivamente indispensavel para operacao minima
   - margem restante no host
4. Se a stack depender de oversubscription para caber, declarar isso explicitamente e qual o risco operacional.

Fase 3 - Analise de capacidade para 10k
1. Avaliar separadamente os envelopes:
   - 10k ativos/mes
   - 10k ativos/dia
   - 10k simultaneos em pico
2. Para cada envelope, analisar:
   - API ingressa HTTP
   - workers e filas/outbox
   - banco PostgreSQL
   - pool de conexoes via pgBouncer
   - observabilidade
   - backup/restore
   - deploy e rollback
   - storage e crescimento de dados
3. So usar extrapolacoes quando forem sustentadas por:
   - limites declarados no codebase
   - parametros configurados
   - evidencias de throughput documentadas no repositorio
   - documentacao oficial do componente
4. Se nao houver base suficiente para cravar capacidade, diga `nao comprovado` para aquele envelope. Nao converta ausencia de dado em resposta positiva.

Fase 4 - Efetividade, confiabilidade e operacao
1. Verificar se a arquitetura atual e de fato resiliente para producao considerando:
   - single point of failure
   - deploy sem downtime relevante
   - health checks reais
   - rollback real
   - restore real
   - PITR/backup
   - observabilidade suficiente para incidentes
   - seguranca operacional de secrets
   - isolamento de rede
2. Identificar o que esta comprovado pelo repo e o que esta apenas prometido/documentado.
3. Tratar ausencia de teste de restore, ausencia de ensaio de carga proporcional, ausencia de evidencia de failover ou ausencia de monitoracao suficiente como risco/gap, nao como detalhe menor.

Fase 5 - Menor custo possivel sem perder confiabilidade
1. Determinar se o menor custo aceitavel ate 10k e:
   - manter KVM 2 com ajustes
   - subir para outro plano Hostinger
   - manter single-node e enxugar a stack
   - externalizar somente componentes especificos
   - ou adotar outra composicao
2. Nao proponha arquitetura mais cara sem provar necessidade.
3. Nao proponha arquitetura mais barata se ela violar qualquer gate obrigatorio.
4. Quando comparar custos, use somente paginas/precos/documentacao oficial do fornecedor. Se o preco nao puder ser comprovado oficialmente, marcar como `custo nao comprovado`.

Fase 6 - Plano de evolucao completo
1. Entregar um plano faseado do estado atual ate 10k, com no minimo:
   - correcoes obrigatorias imediatas para o estado atual
   - etapa para caber com seguranca no curto prazo
   - etapa para absorver crescimento intermediario
   - etapa final para sustentar 10k no envelope suportado
2. Cada etapa deve ter:
   - objetivo
   - mudanca concreta
   - motivo tecnico
   - impacto esperado
   - custo incremental oficial ou `nao comprovado`
   - risco mitigado
   - criterio de entrada
   - criterio de saida
   - evidencia necessaria para declarar concluido
3. Nao deixar etapas vagas. Nao usar frases como `melhorar performance`, `escalar banco` ou `reforcar observabilidade` sem detalhar como, por que e em qual ponto.

Fase 7 - Veredito final sem falso positivo
1. Responder objetivamente:
   - A infra atual cabe no KVM 2 com margem segura?
   - A infra atual e eficiente em custo?
   - A infra atual e confiavel para producao?
   - A infra atual atende 10k usuarios ativos em cada envelope A/B/C?
   - Qual e o menor caminho de evolucao com prova suficiente?
2. A resposta final deve escolher uma classificacao por envelope:
   - `atende hoje`
   - `atende com ajustes obrigatorios`
   - `nao atende`
   - `nao comprovado`
3. Se houver qualquer lacuna relevante sem prova, o veredito global nao pode ser `production-ready/proof`.

Formato de saida obrigatorio em Markdown:

# Relatorio de Analise de Infraestrutura

## 1. Escopo e metodo
- objetivo
- fontes do repo consultadas
- fontes oficiais consultadas
- regras de decisao usadas

## 2. Inventario atual comprovado
Tabela obrigatoria com colunas:
`componente | funcao | onde esta definido | depende de | persistencia | exposicao | observabilidade | criticidade`

## 3. Prova oficial do ambiente Hostinger / KVM 2
Tabela obrigatoria com colunas:
`atributo | valor oficial | fonte oficial | impacto na arquitetura`

## 4. Orcamento real da stack atual
Tabela obrigatoria com colunas:
`servico | replicas | cpu_reservada | cpu_limite | memoria_reservada | memoria_limite | storage | observacoes`

## 5. Comparativo stack x host
Tabela obrigatoria com colunas:
`recurso | capacidade oficial do host | demanda declarada da stack | margem | status`

## 6. Analise por envelope de capacidade
### 6.1 Envelope A - 10k ativos/mes
### 6.2 Envelope B - 10k ativos/dia
### 6.3 Envelope C - 10k simultaneos em pico

Para cada envelope, preencher obrigatoriamente:
- gargalos
- fatores limitantes
- componentes que saturam primeiro
- o que esta comprovado
- o que nao esta comprovado
- veredito: `atende hoje`, `atende com ajustes obrigatorios`, `nao atende` ou `nao comprovado`

## 7. Gaps, lacunas e riscos residuais
Tabela obrigatoria com colunas:
`item | categoria | evidencia | impacto | severidade | bloqueia 10k? | acao obrigatoria`

## 8. Eficiência de custo
Tabela obrigatoria com colunas:
`opcao | custo oficial comprovado | atende quais envelopes | trade-offs | decisao`

## 9. Plano de evolucao completo
Tabela obrigatoria com colunas:
`fase | gatilho | mudanca | custo incremental | risco mitigado | prova exigida para concluir`

## 10. Veredito final
Responder explicitamente:
- `A infra atual no KVM 2 atende hoje o envelope A?`
- `A infra atual no KVM 2 atende hoje o envelope B?`
- `A infra atual no KVM 2 atende hoje o envelope C?`
- `A infra atual e a opcao de menor custo possivel com confiabilidade suficiente?`
- `O relatorio conseguiu fechar 0 gaps e 0 lacunas com prova?`
- `O que impede declarar production-ready/proof, se houver algo?`

Regras finais:
- Nao invente.
- Nao suavize conclusao.
- Nao esconda lacuna dentro de texto corrido.
- Nao declare 10k suportado sem prova objetiva.
- Nao transforme documentacao aspiracional do repo em fato operacional.
- Nao transforme ausencia de dado em suposicao.
```

## Justificativas das adicoes

- Converti o pedido amplo em um roteiro deterministico para evitar resposta vaga ou otimista.
- Transformei `10 mil usuarios ativos` em tres envelopes separados para remover a principal ambiguidade sem pedir nova rodada de esclarecimento.
- Forcei comparacao quantitativa entre capacidade oficial do KVM 2 e o orcamento declarado no `compose.swarm.yml`, porque esse e o centro da decisao.
- Obriguei o uso de documentacao oficial por componente para impedir benchmark improvisado ou conclusao baseada em blog/post.
- Troquei `0 gaps` de desejo por criterio de aceite verificavel, preservando seu nivel de exigencia sem abrir espaco para falso positivo.
- Exigi plano de evolucao faseado com gatilhos, custo e prova de saida para impedir recomendacao generica do tipo `escale depois`.
