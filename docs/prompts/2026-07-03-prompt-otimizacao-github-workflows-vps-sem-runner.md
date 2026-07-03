# 2026-07-03 — Prompt enriquecido para otimização do `.github/workflows`

## Prompt original

> Eu quero otimizar o `.github/workflows` para ser rápido, robusto, econômico, eficiente, 0 gaps, 0 falso positivo, 0 ressalvas, 0 lacunas com base em documentações oficiais e deploy direto na VPS, sem runner na VPS para economizar recurso.

## Prompt enriquecido — versão pronta para uso

```text
Atue como um especialista sênior em GitHub Actions, CI/CD, supply chain security, Docker e deploy em VPS. Sua tarefa é analisar o repositório atual e propor a otimização completa do diretório `.github/workflows` com foco em velocidade, robustez, economia de execução, previsibilidade operacional e deploy direto na VPS sem usar runner instalado na VPS.

Contexto obrigatório do repositório (trabalhe em cima do estado atual do working tree, sem assumir nada fora dele):
- Repositório: `mecontrola`
- Stack principal: Go `1.26.4`
- Arquitetura: monólito modular em Go
- Workflows existentes:
  - `.github/workflows/ci-cd.yml`
  - `.github/workflows/e2e.yml`
  - `.github/workflows/auto-merge.yml`
- O workflow principal atual já contém gates de build, lint, testes unitários, testes de integração, vulncheck, gates de governança, build/push de imagem, scan de imagem, assinatura, deploy, healthcheck e notify.
- O runbook atual em `deployment/runbooks/deploy.md` define como alvo operacional:
  - deploy via GitHub Actions
  - runner efêmero descriptografa secrets com SOPS + age
  - atualização de Docker secrets via SSH
  - deploy Docker Swarm na VPS
  - healthcheck e rollback
  - sem `.env` persistente na VPS
- Há um drift importante a ser tratado na análise: o workflow atual usa `runs-on: [self-hosted, staging]` no job de deploy, mas o objetivo mandatório é eliminar dependência de runner na VPS e fazer deploy direto a partir de runner hospedado pelo GitHub, via SSH, sem agente residente na VPS.

Objetivo principal:
Desenhar a melhor arquitetura de workflows para este repositório, com base em documentação oficial e no código atual, de modo que o pipeline fique mais rápido, robusto, econômico e seguro, removendo gargalos, duplicações, serializações desnecessárias, riscos operacionais, falso positivo em gates e dependência de runner na VPS.

Restrições mandatórias:
1. Não implemente nada.
2. Não altere arquivos.
3. Não invente contexto ausente.
4. Não proponha solução genérica desconectada do repositório real.
5. Não use opinião sem lastro; toda recomendação deve estar ancorada em documentação oficial e no estado atual dos arquivos.
6. O deploy deve acontecer direto na VPS via SSH a partir de runner efêmero do GitHub, sem self-hosted runner na VPS.
7. A solução deve priorizar economia de minutos, cache eficiente, paralelismo seguro, menor superfície de falha e menor custo operacional contínuo.
8. Não aceite “melhoria parcial”, “talvez”, “depende” ou “poderia”; se existir ambiguidade real, explicite exatamente o ponto, o impacto e a decisão mais segura.
9. Não deixar TODO, TBD, “avaliar depois”, “opcional” ou lacunas de decisão.
10. Se houver trade-off inevitável, explicite com objetividade qual opção você recomenda e por quê.

Fontes permitidas e obrigatórias:
- Estado atual do repositório
- Documentação oficial do GitHub Actions e GitHub Security
- Documentação oficial do Docker/Buildx/GHCR
- Documentação oficial do Sigstore Cosign
- Documentação oficial do Trivy
- Documentação oficial do SOPS/age

Escopo exato da análise:
1. Revisar os workflows atuais e identificar:
   - jobs redundantes
   - setup duplicado
   - uploads de artefato desnecessários
   - gates com custo alto e baixo valor
   - serializações evitáveis
   - oportunidades reais de paralelismo
   - riscos de falso positivo e falso negativo
   - fragilidades de segurança
   - pontos que hoje exigem runner na VPS
2. Definir a arquitetura-alvo ideal para:
   - CI principal
   - build/push de imagem
   - scan de imagem
   - assinatura
   - deploy direto na VPS via SSH
   - healthcheck pós-deploy
   - notificações
   - workflow manual de E2E
   - auto-merge do Dependabot
3. Propor como remover o self-hosted runner do deploy sem perder:
   - segurança
   - rollback
   - healthcheck
   - controle de concorrência
   - gestão de secrets
4. Propor como reduzir tempo total de pipeline sem enfraquecer:
   - qualidade
   - segurança
   - confiabilidade de deploy
5. Propor como manter pinagem segura de actions, permissões mínimas e isolamento por ambiente.

O que você deve entregar:
Entregue uma análise técnica completa e pronta para execução humana, em PT-BR, com as seções abaixo e nesta ordem exata:

1. **Resumo executivo**
   - diagnóstico curto do estado atual
   - principais desperdícios
   - arquitetura recomendada
   - ganho esperado em tempo, custo e confiabilidade

2. **Leitura do estado atual**
   - liste os workflows analisados
   - descreva o papel real de cada um
   - identifique o drift entre workflow atual e objetivo de deploy sem runner na VPS

3. **Problemas encontrados**
   - tabela com: problema, arquivo/job afetado, impacto, severidade, evidência no repositório

4. **Arquitetura-alvo recomendada**
   - desenho textual do fluxo ideal fim a fim
   - quais jobs permanecem
   - quais jobs devem ser fundidos
   - quais jobs devem ser separados
   - quais gatilhos devem existir
   - como fica a estratégia de `concurrency`
   - como fica a estratégia de cache
   - como fica a estratégia de artifacts

5. **Deploy direto na VPS sem runner na VPS**
   - explique a solução recomendada
   - detalhe como o runner GitHub-hosted executa o deploy via SSH
   - explique como tratar SSH key, host verification, secrets, SOPS/age, Docker secrets, migrations, deploy Swarm, rollback e cleanup
   - deixe explícito por que essa abordagem é melhor do que manter self-hosted runner na VPS neste contexto

6. **Plano de otimização por arquivo**
   - para cada workflow atual, diga:
     - o que manter
     - o que remover
     - o que consolidar
     - o que renomear
     - o que endurecer
   - se recomendar criar workflow novo, justifique por que ele é necessário e qual responsabilidade isolada ele terá

7. **Segurança e supply chain**
   - permissões mínimas por job
   - pinagem de actions por SHA completo
   - proteção contra vazamento de secrets
   - estratégia para scan e assinatura
   - validações para impedir deploy de imagem não assinada ou com vulnerabilidade bloqueante

8. **Performance e economia**
   - oportunidades reais de redução de tempo
   - oportunidades reais de redução de custo
   - o que pode rodar em paralelo
   - o que deve ser serializado
   - o que deve sair do caminho crítico
   - estimativa qualitativa de impacto para cada recomendação

9. **Plano de mudança pronto para implementação**
   - sequência exata de mudanças recomendadas
   - ordem segura de rollout
   - riscos por etapa
   - critérios objetivos para considerar a mudança concluída

10. **Critérios de aceitação finais**
   - lista objetiva e verificável
   - sem itens vagos

11. **Referências oficiais**
   - liste cada recomendação relevante com link da documentação oficial correspondente

Regras de qualidade da resposta:
1. Nada de resposta genérica.
2. Nada de checklist superficial sem explicar o porquê.
3. Nada de recomendações sem apontar qual arquivo/job atual está sendo corrigido.
4. Nada de assumir serviços, runners, ambientes ou segredos que não estejam evidenciados.
5. Toda recomendação deve explicitar benefício, risco mitigado e impacto operacional.
6. Toda recomendação deve diferenciar claramente:
   - obrigatório
   - recomendado
   - desnecessário
7. Se algo já estiver correto no workflow atual, diga explicitamente que deve ser preservado.
8. Se encontrar conflito entre o runbook e os workflows, trate isso como drift real e resolva na recomendação.
9. Priorize solução simples, sólida e barata de manter.
10. O resultado final deve ficar utilizável como especificação direta de refatoração dos workflows.

Critérios de aceitação obrigatórios da sua resposta:
1. Explicar como eliminar `runs-on: [self-hosted, staging]` do deploy e substituir por runner GitHub-hosted com SSH na VPS.
2. Mapear claramente o fluxo de deploy sem `.env` persistente na VPS e com secrets tratados de forma efêmera.
3. Mostrar como preservar ou melhorar build, lint, unit, integration, vulncheck, scan, sign, healthcheck e notify.
4. Identificar gargalos concretos no workflow atual, não hipóteses abstratas.
5. Indicar onde há desperdício de minutos, duplicação de setup ou custo operacional evitável.
6. Cobrir concorrência, rollback, idempotência, healthcheck e cleanup.
7. Referenciar documentação oficial para os pontos críticos.
8. Entregar um plano sem lacunas operacionais.

Seja rigoroso, específico, pragmático e orientado ao estado real deste repositório.
```

## O que foi adicionado e por quê

| Adição | Justificativa |
|---|---|
| Contexto explícito do repositório e dos arquivos reais | Evita resposta genérica e força aderência ao working tree atual. |
| Drift do `self-hosted` no deploy | Fixa o principal problema a ser resolvido sem ambiguidade. |
| Restrições mandatórias de “não implementar” | Garante que a resposta fique em modo análise/especificação, não execução. |
| Formato de saída fechado | Reduz desvio e aumenta reusabilidade imediata do prompt. |
| Critérios de aceitação verificáveis | Minimiza falso positivo e resposta superficial. |
| Exigência de referências oficiais | Força rastreabilidade técnica e reduz opinião solta. |
| Escopo detalhado por workflow/job | Obriga cobertura completa e evita gaps. |

## Variante compacta

Use apenas se você quiser uma versão menor, com menos contexto inline e maior dependência da capacidade investigativa do agente:

```text
Analise o estado atual de `.github/workflows` deste repositório e proponha a arquitetura ótima de CI/CD para deixá-lo mais rápido, robusto, econômico, eficiente e seguro, com base exclusiva no working tree atual e em documentação oficial. Não implemente nada.

O ponto mandatório é remover a dependência de runner na VPS: o deploy deve acontecer direto na VPS via SSH a partir de runner GitHub-hosted efêmero, preservando SOPS + age, Docker secrets, Docker Swarm, migrations, rollback, healthcheck, notify e controle de concorrência.

Entregue em PT-BR:
1. diagnóstico do estado atual;
2. problemas concretos por workflow/job;
3. arquitetura-alvo recomendada;
4. plano detalhado para eliminar `runs-on: [self-hosted, staging]`;
5. otimizações de performance/custo;
6. hardening de segurança;
7. plano de mudança pronto para implementação;
8. critérios de aceitação objetivos;
9. links de documentação oficial por recomendação.

Nada de resposta genérica, nada de lacunas, nada de TODO/TBD, nada de sugestões sem evidência no repositório.
```
