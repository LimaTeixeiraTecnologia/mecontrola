# 2026-07-03 — Prompt enriquecido para unificação dos ambientes local, produção e debug

## Prompt original

> Eu quero ter um comando que eu consiga local no docker ter exatamente o mesmo ambiente que a vps em produção, e também eu quero outro comando para ter o ambiente completo com docker-compose, e também subir só a infra e o debug com o vscode, analisar no codebase o que realmente são utilizadas e remover as legadas de forma efetiva, robusto, economico, eficiente, 0 gaps, 0 falso positivo, 0 ressalvas, 0 lacunas.

## Prompt enriquecido — versão pronta para uso

```text
Atue como um engenheiro sênior de plataforma, Docker, Docker Swarm, Docker Compose, Taskfile, Go e ergonomia de desenvolvimento local. Execute a mudança no repositório real, sem inventar contexto, com foco em consolidar os fluxos de ambiente local e produção, eliminar superfícies legadas e deixar comandos canônicos, previsíveis e documentados.

Antes de qualquer mudança:
1. Leia e siga `AGENTS.md`.
2. Carregue `.agents/skills/agent-governance/SKILL.md`.
3. Como a superfície principal desta tarefa é operacional/Taskfile/Docker, trate `taskfile-production` como skill procedural principal.
4. Se precisar alterar código Go, carregue também `.agents/skills/go-implementation/SKILL.md`.
5. O ponto de partida obrigatório da análise de bootstrap é `cmd/server/server.go` e `cmd/worker/worker.go`.
6. É proibido partir de `internal/platform/runtime` ou usá-lo como referência central.

Objetivo principal:
Implementar uma consolidação real dos ambientes de execução para que o repositório tenha, no final, três fluxos canônicos e sem ambiguidade:

1. um comando canônico para rodar localmente em Docker o ambiente com paridade máxima e intencional com a VPS de produção;
2. um comando canônico para subir o ambiente completo de desenvolvimento com Docker Compose;
3. um comando canônico para subir apenas a infraestrutura necessária para depuração local com VS Code, deixando `server`, `worker` e `migrate` aptos para debug fora dos containers.

Além disso:
- analise o codebase inteiro para descobrir quais arquivos, scripts, task namespaces, composes, runbooks e instruções são realmente usados hoje;
- remova de forma efetiva tudo que for legado, duplicado, contraditório ou morto, desde que a remoção esteja ancorada em evidência real do repositório;
- preserve tudo que ainda for parte de um fluxo ativo e comprovado.

Contexto real do repositório que deve guiar a implementação:
- Repositório: `mecontrola`
- Stack principal: Go `1.26.4`
- Arquitetura: monólito modular em Go
- Entrypoints reais: `cmd/server/server.go` e `cmd/worker/worker.go`
- Superfície atual de ambiente local via Taskfile:
  - `task local:infra`
  - `task local:up`
  - `task local:down`
  - `task local:destroy`
  - `task local:logs`
  - `task local:ps`
  - `task local:db:restart`
- Superfície atual de Swarm local/produção via Taskfile:
  - `task swarm:local:init`
  - `task swarm:local:deploy`
  - `task swarm:local:ps`
  - `task swarm:local:logs`
  - `task swarm:local:rm`
  - `task swarm:prod:*`
- Arquivos de compose existentes:
  - `deployment/compose/compose.yml`
  - `deployment/compose/compose.local.yml`
  - `deployment/compose/compose.prod.yml`
  - `deployment/compose/compose.swarm.yml`
- Dockerfile principal:
  - `deployment/docker/Dockerfile`
- Scripts operacionais relevantes:
  - `deployment/scripts/deploy-swarm.sh`
  - `deployment/scripts/deploy-local.sh`
  - `deployment/scripts/deploy-full.sh`
- Runbook relevante:
  - `deployment/runbooks/deploy.md`
- Fato importante de produção:
  - a produção canônica atual roda em Docker Swarm single-node com `deployment/compose/compose.swarm.yml`
  - não deve existir `.env` persistente na VPS
  - secrets de produção são tratados via `deployment/config/prod.env`, `deployment/config/prod.secrets.env`, SOPS + age e Docker secrets

Drifts e sinais de legado que precisam obrigatoriamente entrar na sua análise:
1. existe mais de uma superfície para ambiente e deploy (`local`, `swarm`, `deploy-local.sh`, compose prod antigo, runbooks e docs), então você deve determinar o que é canônico e o que é legado com base em uso real;
2. `deployment/scripts/deploy-swarm.sh` e `deployment/scripts/deploy-local.sh` ainda possuem fallback para `.env` tratado como legado;
3. `README.md` afirma que existe `.vscode/launch.json`, mas o working tree atual pode não conter `.vscode/`;
4. a produção canônica é Swarm, enquanto ainda existem arquivos e instruções baseados em Compose de produção;
5. qualquer documentação, task ou script que contradiga o fluxo real deve ser corrigido ou removido, nunca mantido como desvio conhecido.

Restrições mandatórias:
1. Trabalhe exclusivamente em cima do estado atual do working tree.
2. Não preserve legado por medo ou conveniência.
3. Não remova nada sem evidência concreta de que é duplicado, morto, contraditório ou substituído por uma superfície canônica melhor.
4. Não deixe TODO, TBD, “avaliar depois”, “talvez”, “opcional”, “depois alinhar”, “fica para outra PR” ou qualquer lacuna operacional.
5. Não faça mudança cosmética sem consolidar comportamento real.
6. Não invente `.vscode/launch.json`, tasks, scripts, profiles, services ou compose overrides sem validar se são realmente necessários.
7. O resultado final deve privilegiar uma superfície canônica única para comandos de ambiente. Se houver múltiplos caminhos, um deles deve ser explicitamente o oficial e os demais devem ser removidos ou relegados apenas se ainda forem necessários por evidência operacional.
8. Quando um fluxo perder sentido no novo escopo, remova por completo arquivos, docs, tasks e referências associadas; não deixe campo morto, comando morto, doc morta ou compatibilidade vazia.
9. Preservar a arquitetura e os entrypoints reais do projeto é obrigatório.
10. Toda decisão deve otimizar robustez, economia operacional, previsibilidade e baixo risco de falso positivo.

Decisão mandatória de UX operacional:
Converja os comandos canônicos para `Taskfile`, salvo se encontrar evidência objetiva e forte de que isso seria pior do que manter scripts soltos. Em caso de convergência para Taskfile:
- o usuário deve conseguir descobrir e executar os três fluxos principais por comandos `task ...`;
- scripts auxiliares podem continuar existindo apenas quando forem dependências internas da task canônica, não como superfície primária paralela sem necessidade.

O que você deve investigar antes de editar:
1. quais tasks são realmente usadas e quais são apenas wrappers redundantes;
2. se `compose.prod.yml` ainda é parte de um fluxo vivo ou se virou herança de uma fase pré-Swarm;
3. se `compose.yml` + `compose.local.yml` cobrem de forma correta o ambiente full local;
4. se `compose.swarm.yml` já é suficiente para a paridade local com produção ou se faltam variáveis, bootstrap, secrets, init, docs ou tasks para torná-lo realmente utilizável em desenvolvimento local;
5. se o fluxo de debug no VS Code existe de verdade ou se a documentação está adiantada em relação aos arquivos do repositório;
6. se existem duplicações entre README, runbooks, Taskfile e scripts;
7. se ainda há referências legadas a `.env` persistente em produção, compose de produção antigo, comandos antigos de deploy ou superfícies que deveriam ter sido removidas.

Implementação esperada:
1. Definir e entregar os três fluxos canônicos finais.
2. Tornar o fluxo de “paridade com produção” realmente acionável em máquina local, com a melhor equivalência possível ao ambiente da VPS atual.
3. Tornar o fluxo “ambiente completo com Docker Compose” realmente simples e direto.
4. Tornar o fluxo “somente infra + debug VS Code” real, coerente e documentado, incluindo criação/ajuste de `.vscode/launch.json` apenas se o codebase provar que esse artefato precisa existir para cumprir o objetivo.
5. Consolidar, renomear, mover ou remover tasks/scripts/arquivos para que a superfície final fique econômica.
6. Corrigir README e runbooks para que reflitam exatamente o comportamento real resultante.
7. Eliminar referências legadas e contraditórias descobertas na análise.

Superfícies mínimas que devem sair consistentes ao final:
1. Taskfile raiz e taskfiles relevantes.
2. Arquivos de compose relevantes.
3. Scripts operacionais ainda necessários.
4. README.
5. Runbooks afetados.
6. `.vscode/launch.json` apenas se for de fato necessário para cumprir o fluxo de debug prometido.

Seção obrigatória da sua resposta final:
Entregue em PT-BR, de forma objetiva, com as seções abaixo e nesta ordem:

1. **Resumo do que foi consolidado**
2. **Comandos canônicos finais**
   - comando de paridade com produção
   - comando de ambiente completo com Compose
   - comando de infra + debug
3. **Inventário do que estava ativo vs legado**
   - tabela com: item, tipo, status final, evidência, decisão
4. **Mudanças implementadas por arquivo**
5. **Legados removidos**
6. **Drifts corrigidos**
7. **Critérios de aceitação atendidos**

Critérios de aceitação obrigatórios:
1. Existe exatamente uma forma canônica e documentada de subir o ambiente com paridade local de produção.
2. Existe exatamente uma forma canônica e documentada de subir o ambiente completo de desenvolvimento com Docker Compose.
3. Existe exatamente uma forma canônica e documentada de subir só a infraestrutura para debug local.
4. O fluxo de debug não depende de documentação fictícia; se `.vscode/launch.json` for prometido, ele precisa existir e funcionar com o desenho adotado.
5. Toda referência legada identificada como morta ou contraditória foi removida do código, Taskfile, scripts e documentação.
6. O repositório termina sem instruções concorrentes para o mesmo objetivo sem justificativa explícita.
7. O fluxo de produção continua alinhado ao fato canônico: Docker Swarm single-node, `compose.swarm.yml`, sem `.env` persistente na VPS.
8. Fallbacks legados a `.env` em produção só podem permanecer se você provar que ainda são necessários; caso contrário, remova.
9. O resultado final reduz ambiguidade operacional, não aumenta.
10. Nenhuma recomendação ou alteração pode depender de contexto fora do repositório atual.

Regras de qualidade:
1. Seja cirúrgico, mas completo.
2. Prefira remover duplicação a coexistir com superfícies paralelas.
3. Toda remoção deve ser respaldada por evidência no codebase.
4. Toda permanência de item antigo deve ser justificada.
5. Não aceite drift residual entre README, Taskfile, compose e scripts.
6. Não feche a tarefa com “quase pronto”.
7. Se houver trade-off inevitável, escolha a opção mais segura, mais simples e mais barata de manter, e aplique.

Execute a mudança fim a fim.
```

## O que foi adicionado e por quê

| Adição | Justificativa |
|---|---|
| Contexto real do repositório e das superfícies atuais | Força o agente a trabalhar no que existe hoje, não em uma solução genérica de Docker. |
| Obrigação de partir de `cmd/server/server.go` e `cmd/worker/worker.go` | Alinha o prompt ao bootstrap real do projeto e evita desvio para referências erradas. |
| Distinção entre Compose local e Swarm de produção | Mantém a análise coerente com a topologia real já usada na VPS. |
| Lista explícita de drifts conhecidos | Obriga o agente a tratar conflitos reais já visíveis no codebase, inclusive docs possivelmente falsas. |
| Decisão mandatória de convergir para Taskfile | Reduz superfícies paralelas e transforma scripts em implementação interna, não UX primária. |
| Critérios de aceitação fechados | Minimiza falso positivo e impede resposta parcial ou só documental. |
| Seções obrigatórias da resposta final | Garante rastreabilidade do que ficou canônico, do que saiu e por quê. |

## Variante compacta

Use esta versão apenas se você quiser um prompt menor e aceitar menos contexto inline:

```text
Leia `AGENTS.md`, use `agent-governance` e trate `taskfile-production` como skill principal. Se tocar Go, carregue também `go-implementation`. Parta obrigatoriamente de `cmd/server/server.go` e `cmd/worker/worker.go`, nunca de `internal/platform/runtime`.

Implemente a consolidação dos ambientes do repositório para entregar três comandos canônicos, econômicos e sem ambiguidade:
1. paridade local com a VPS de produção;
2. ambiente completo com Docker Compose;
3. somente infraestrutura para debug no VS Code.

Analise o codebase inteiro e remova de forma efetiva tudo que for legado, contraditório, duplicado ou morto, com base em evidência real. Considere especialmente `Taskfile.yml`, `taskfiles/local.yml`, `taskfiles/swarm.yml`, `deployment/compose/{compose.yml,compose.local.yml,compose.prod.yml,compose.swarm.yml}`, `deployment/scripts/{deploy-swarm.sh,deploy-local.sh,deploy-full.sh}`, `deployment/runbooks/deploy.md` e `README.md`.

Leve em conta os drifts já visíveis:
- produção canônica atual é Docker Swarm single-node com `compose.swarm.yml`;
- não deve existir `.env` persistente na VPS;
- ainda há fallback legado para `.env` em scripts;
- o README afirma existir `.vscode/launch.json`, mas o working tree atual pode não ter `.vscode/`.

Converja a UX operacional para `task ...` como superfície canônica, salvo evidência forte em contrário. Corrija documentação, compose, tasks e scripts para que o resultado final tenha exatamente uma forma canônica para cada um dos três objetivos.

Entregue em PT-BR:
1. resumo do que foi consolidado;
2. comandos canônicos finais;
3. inventário ativo vs legado com evidências;
4. mudanças por arquivo;
5. legados removidos;
6. drifts corrigidos;
7. critérios de aceitação atendidos.
```
