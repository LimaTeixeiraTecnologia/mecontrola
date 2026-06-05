# Prompt enriquecido — SDD completo do épico MVP

## Prompt original

```text
Analisar criteriosamente, minuciosamente: docs/discoveries e docs/epics para de fato começar a fazer o SDD completo e entregar o épico de forma robusta, eficiente, production-ready, production-proof, focando na primeira versão do MVP.
```

## Diagnóstico do prompt original

### Intenção principal

Gerar um **SDD completo, robusto e orientado ao MVP** a partir dos documentos em `docs/discoveries/` e `docs/epics/`, sem partir para implementação.

### Contexto já disponível no repositório

- O repositório é um **monolito modular em Go** com bounded contexts em `internal/`.
- As discoveries e os épicos já definem **roadmap, bloqueios, escopo, restrições inegociáveis, critérios de aceite e dependências**.
- O roadmap atual indica:
  - **E1** é a raiz do MVP.
  - **E2** e **E3** dependem de E1.
  - **E4** é **pós-MVP/backlog** e não deve ser tratado como alvo principal agora.
- O `AGENTS.md` exige que o **working tree atual** prevaleça se houver conflito com docs históricos.

### Lacunas e ambiguidades

1. **"o épico" está ambíguo**: não diz explicitamente se o alvo é E1, E2, E3 ou o próximo épico elegível do roadmap.
2. **"SDD completo" não define formato de saída**, se é documento único, com ADRs, matriz de riscos, rollout, testes, etc.
3. **Não explicita que a resposta deve citar fontes** e separar fato documentado de hipótese.
4. **Não limita o escopo do MVP**, o que pode fazer o agente puxar E4 ou hardening pós-MVP cedo demais.
5. **Não deixa claro que não deve implementar nada**, só produzir a especificação.

## Prompt enriquecido

```text
Você é um arquiteto/principal engineer responsável por produzir um SDD (Software Design Document) completo, em PT-BR, para o próximo épico elegível do MVP deste repositório.

Seu trabalho NÃO é implementar nada. Seu trabalho é analisar profundamente o material existente e entregar um SDD pronto para handoff de execução, consistente com o repositório, com foco em robustez, eficiência, production-ready e production-proof, mas sem expandir o escopo além da primeira versão do MVP.

## Objetivo

Ler criteriosamente os arquivos em:
- `docs/discoveries/`
- `docs/epics/`
- `AGENTS.md`

e produzir um SDD completo para o épico correto do roadmap de MVP, respeitando dependências, bloqueios, arquitetura, restrições do projeto e o estado atual do working tree como fonte da verdade.

## Regras obrigatórias

1. **Não implemente código, migrations, handlers, jobs, routers, adapters ou testes.**
2. **Não invente contexto ausente.** Se algo não estiver sustentado pelos arquivos lidos, marque como dúvida, decisão pendente ou hipótese explícita.
3. **Considere o working tree atual como fonte da verdade** se houver conflito entre documentação histórica e código atual.
4. **Mantenha o foco no MVP.**
   - Considere **E4 (`reconciliation-hardening`) fora de escopo**, salvo se for citado apenas como backlog/pós-MVP.
   - Respeite a ordem e os bloqueios do roadmap descritos em `docs/epics/README.md`.
5. **Ao escolher o épico-alvo**, siga esta regra:
   - se houver um épico raiz/bloqueador do MVP ainda sem SDD, ele tem prioridade;
   - caso já exista material suficiente para um épico dependente, explicite a dependência;
   - se houver ambiguidade real, deixe registrado qual épico foi escolhido e por quê.
6. **Não proponha soluções que violem `AGENTS.md`.**
   - Preservar monólito modular em Go;
   - respeitar fluxo `infrastructure -> application -> domain`;
   - `domain` sem dependências de infraestrutura;
   - comunicação cross-module só por contrato explícito, interface do consumidor ou event/outbox;
   - evitar abstrações prematuras.
7. **Se mencionar superfícies de runtime existentes, parta de `cmd/server/server.go` e/ou `cmd/worker/worker.go`**, sem usar `internal/platform/runtime` como ponto de partida.
8. **Toda afirmação importante deve estar ancorada em fonte**, citando o arquivo correspondente.
9. **A primeira parte do SDD é obrigatoriamente um handoff estruturado para as etapas downstream `create-prd`, `create-technical-specification` e `create-tasks`.** Isso significa que a abertura do documento deve consolidar problema, objetivo, escopo, restrições, dependências, riscos, critérios de aceite e framing do épico de modo reaproveitável por essas três etapas, sem exigir redescoberta do contexto.
10. **A decomposição de PRDs deve ser pensada desde o SDD em fatias eficientes, verificáveis e implementáveis ponta a ponta.** Evite PRDs monolíticos, vagos ou grandes demais para validação confiável. Estruture o material para que a futura quebra em tasks minimize lacunas, maximize rastreabilidade e reduza risco de falso positivo de implementação concluída.

## Escopo de análise

Analise, no mínimo:

1. Roadmap e bloqueios entre épicos.
2. Descobertas funcionais e técnicas presentes nas discoveries.
3. Restrições inegociáveis de cada épico.
4. Critérios de aceite e impactos arquiteturais.
5. Dependências externas, pré-requisitos não técnicos e riscos residuais.
6. Aderência à arquitetura e governança definida em `AGENTS.md`.
7. Limites explícitos de MVP vs pós-MVP.

## Critério de escolha do épico-alvo

Antes de escrever o SDD, execute esta triagem:

1. Liste os épicos encontrados.
2. Identifique status, `blocked_by`, `blocks`, `next_skill`, `target_module` e se já existe PRD/techspec.
3. Defina qual é o **próximo épico elegível para detalhamento de SDD no MVP**.
4. Explique em no máximo 10 linhas por que ele foi escolhido.

Se a análise apontar que:
- **E1** é o épico base e ainda precisa do SDD completo, priorize **E1**;
- **E2** ou **E3** forem detalhados, explicite claramente que a execução depende de **E1 implemented**;
- **E4** aparecer, trate-o apenas como backlog pós-MVP e não como alvo do SDD principal.

## Formato obrigatório de saída

Entregue um único documento Markdown em PT-BR dividido em **duas macro-partes obrigatórias**:

### Parte 1 — Base obrigatória para handoff downstream

Esta primeira parte deve ser escrita para servir **diretamente** como insumo para:
- `create-prd`
- `create-technical-specification`
- `create-tasks`

Ela deve consolidar, de forma executável e sem ambiguidade:
- enquadramento do épico;
- problema e motivação;
- objetivo do MVP;
- escopo incluído e fora de escopo;
- restrições inegociáveis;
- dependências, bloqueios e pré-requisitos;
- critérios de aceite;
- riscos, dúvidas abertas e decisões pendentes.

### Parte 2 — SDD completo

A segunda parte expande a base anterior no desenho técnico completo.

Use esta estrutura:

1. **Decisão de enquadramento**
   - épico escolhido
   - justificativa da escolha
   - status dos épicos relacionados
   - confirmação explícita de que o documento está focado na primeira versão do MVP

2. **Base obrigatória para `create-prd`, `create-technical-specification` e `create-tasks`**
   - problema
   - contexto consolidado
   - objetivo do épico no MVP
   - escopo incluído
   - fora de escopo
   - restrições inegociáveis
   - dependências e bloqueios
   - critérios de aceite
   - riscos relevantes
   - dúvidas/pendências que precisam de decisão
   - proposta de fatiamento eficiente do PRD em blocos implementáveis e verificáveis

3. **Resumo executivo**
   - épico-alvo escolhido
   - objetivo do SDD
   - por que esse épico é o correto para o MVP agora

4. **Fontes analisadas**
   - lista de arquivos lidos
   - breve papel de cada arquivo

5. **Leitura consolidada do problema**
   - problema de negócio
   - motivação
   - impacto no roadmap
   - relação com discoveries e outros épicos

6. **Escopo do SDD**
   - incluído
   - fora de escopo
   - fronteiras explícitas de MVP

7. **Restrições inegociáveis**
   - extraídas dos épicos/discoveries/AGENTS.md
   - separadas por arquitetura, domínio, dados, integração, segurança/LGPD e operação

8. **Estado atual e drift**
   - o que já existe no repositório e pode ser reaproveitado
   - placeholders, lacunas ou conflitos documentais
   - riscos de assumir algo ainda não materializado

9. **Design proposto**
   - visão arquitetural
   - bounded context / módulo alvo
   - fluxos principais
   - componentes e responsabilidades
   - contratos entre camadas
   - contratos cross-module
   - decisões sobre persistência
   - decisões sobre eventos/outbox/workers
   - decisões sobre HTTP/webhooks/consumers/jobs, se aplicável
   - decisões de idempotência, consistência e ordenação, se aplicável

10. **Modelo de domínio e dados**
   - agregados, entidades, value objects, serviços de domínio
   - tabelas/coleções necessárias
   - chaves, unicidade, soft delete, histórico, índices relevantes
   - invariantes e regras de negócio

11. **Fluxos detalhados**
   - fluxo feliz ponta a ponta
   - fluxos alternativos
   - falhas esperadas
   - retries, deduplicação, rollback, compensação ou reconciliação, se aplicável

12. **Integrações e dependências externas**
    - provedores externos
    - segredos/configuração
    - rate limit
    - timeouts
    - requisitos por ambiente

13. **Segurança, privacidade e compliance**
    - PII envolvida
    - mascaramento de logs
    - LGPD
    - superfícies de abuso/fraude
    - controles mínimos de MVP

14. **Observabilidade e operação**
    - logs estruturados
    - métricas
    - traces/correlação, se fizer sentido
    - alertas mínimos
    - runbook operacional mínimo do MVP

15. **Estratégia de validação**
    - testes unitários
    - testes de integração
    - smoke/E2E
    - critérios de aceite mapeados para evidências
    - validações técnicas proporcionais ao risco

16. **Riscos, trade-offs e decisões pendentes**
    - riscos técnicos
    - riscos operacionais
    - trade-offs conscientes
    - decisões que precisam de ADR ou confirmação antes da execução

17. **Plano incremental de implementação**
    - fases sugeridas
    - dependências entre fases
    - o que pode ser paralelizado
    - o que bloqueia execução
    - como o futuro PRD deve ser quebrado em incrementos pequenos, testáveis e sem ambiguidade de done

18. **Checklist final de prontidão**
    - o que precisa estar definido para o épico seguir para tasks/execução

## Critérios de qualidade obrigatórios

O SDD final deve:

1. Estar **claramente focado no MVP**, sem puxar escopo de hardening pós-MVP para dentro do épico principal.
2. Transformar discoveries e épicos em **decisões técnicas executáveis**, e não apenas resumir os documentos.
3. Explicitar **dependências, riscos, dúvidas e bloqueios**.
4. Separar nitidamente:
   - fato documentado
   - inferência razoável
   - decisão pendente
5. Fazer com que a **Parte 1 seja suficiente para alimentar `create-prd`, `create-technical-specification` e `create-tasks`** sem precisar reanalisar todo o material bruto.
6. Propor um **fatiamento de PRD eficiente**, em unidades de entrega pequenas o bastante para validação real, mas completas o suficiente para produzir valor verificável, evitando blocos grandes que gerem status “implementado” sem cobertura efetiva.
7. Garantir **rastreabilidade entre escopo, critérios de aceite, design e futura decomposição**, reduzindo risco de falso positivo de conclusão.
8. Conectar o design às regras do repositório em `AGENTS.md`.
9. Ser detalhado o suficiente para viabilizar a próxima etapa de decomposição em tasks, sem precisar reinventar o desenho.

## Estilo de resposta

- Escreva em tom técnico, direto e sem floreio.
- Seja minucioso, mas evite repetição.
- Use subtítulos claros.
- Use tabelas apenas quando realmente melhorarem a leitura.
- Cite caminhos de arquivo ao justificar decisões.
- Não devolva pseudo-código excessivo; priorize arquitetura, contratos, fluxos e decisões.

## Exigência adicional de reaproveitamento

A **Parte 1** deve ser redigida como um bloco que possa ser reutilizado quase literalmente:
- como framing inicial do `create-prd`;
- como contexto consolidado do `create-technical-specification`;
- como base de decomposição do `create-tasks`.

Se houver qualquer ponto que impeça esse reaproveitamento, o documento deve sinalizar explicitamente o gap.

Além disso, a resposta deve indicar **como quebrar o PRD futuro em slices/incrementos de implementacao** com estas propriedades:
- cada slice tem objetivo claro e verificavel;
- cada slice possui fronteira funcional nítida;
- cada slice pode ser validado sem depender de conclusões subjetivas;
- cada slice reduz o risco de marcar algo como “pronto” sem cobertura real do escopo.
```

## Variante opcional

Se você quiser usar este prompt de forma parametrizada, substitua o trecho “próximo épico elegível do MVP” por:

```text
o épico `<EPIC_ID ou slug>` informado pelo usuário, ainda que esteja bloqueado para execução; nesse caso, detalhe claramente os bloqueios e limite o documento ao SDD/planejamento, sem pressupor implementação imediata.
```

## Justificativa das adições

- **Escolha explícita do épico-alvo:** remove a ambiguidade de “o épico”.
- **Parte 1 obrigatória para handoff downstream:** garante reaproveitamento imediato em `create-prd`, `create-technical-specification` e `create-tasks`.
- **Fatiamento eficiente de PRD:** aumenta a confiabilidade da execução e reduz falso positivo de entrega concluída.
- **Formato obrigatório do SDD:** evita resposta vaga ou superficial.
- **Âncora em `AGENTS.md`:** garante aderência arquitetural e reduz alucinação.
- **Separação entre fato, inferência e pendência:** melhora auditabilidade.
- **Foco em MVP:** evita que o agente puxe E4/hardening cedo demais.
- **Critérios de qualidade e prontidão:** transforma o prompt em insumo real para a próxima etapa downstream.
