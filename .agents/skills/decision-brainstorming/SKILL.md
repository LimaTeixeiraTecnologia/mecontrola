---
name: decision-brainstorming
description: Conduz brainstorming decisório production-ready em PT-BR antes do discovery, confrontando soluções prematuras, mapeando hipóteses, restrições, alternativas, trade-offs, riscos, custos e impactos operacionais. Produz bundle local auditável com decisão preliminar explícita e scorecard validado para handoff a discovery técnico, épico/story discovery ou PRD. Use quando houver ideia inicial, solução sugerida, decisão arquitetural preliminar ou incerteza estratégica antes de discovery. Não use para gerar código, backlog, PRD, épicos, user stories ou arquitetura final.
---

# Brainstorming Decisório Production-Ready

<critical>Todos os artefatos DEVEM ser escritos em PT-BR.</critical>
<critical>A skill DEVE ocorrer antes de qualquer discovery técnico, discovery de épico/story ou PRD.</critical>
<critical>O agente DEVE desafiar a solução sugerida pelo usuário antes de aceitá-la como direção recomendada.</critical>
<critical>Gerar no mínimo 3 e no máximo 5 alternativas antes de convergir.</critical>
<critical>Toda clarificação DEVE ocorrer em formato de múltipla escolha. Se o agente não suportar pergunta estruturada, renderizar opções textuais e aguardar resposta inequívoca.</critical>
<critical>A decisão final DEVE ser explícita do usuário. Não inferir aprovação silenciosa.</critical>
<critical>O bundle só é considerado pronto quando `scripts/validate-bundle.py` retornar `SUCCESS`.</critical>
<critical>O comportamento DEVE ser agnóstico de agente: Claude, Codex, Gemini e Copilot DEVEM seguir a mesma ordem de passos, o mesmo bundle e os mesmos gates.</critical>

## Entrada Obrigatória
- Ideia inicial, problema, oportunidade, hipótese de solução ou decisão arquitetural/estratégica preliminar.
- Contexto mínimo de negócio ou técnico suficiente para formular alternativas comparáveis.

## Entrada Recomendável
- Restrições conhecidas, prazo, orçamento, sistemas impactados, riscos percebidos, evidências, métricas, incidentes, requisitos regulatórios ou materiais de apoio.

## Exemplos de Uso
- Para exemplos de prompts válidos e handoffs esperados, ler `references/examples.md` quando o usuário pedir orientação de uso ou demonstração.

## Saída
Bundle local em `./discoveries/brainstorm-<slug>/`:
- `bundle.json` - índice do brainstorming decisório, metadados e readiness.
- `decision-brief.md` - artefato principal para handoff.
- `transcript.md` - histórico auditável das rodadas, perguntas, respostas e decisões.
- `assumptions.md` - hipóteses, evidências, riscos de premissa e status.
- `option-scorecard.md` - comparação objetiva das alternativas.

## Contrato de Compatibilidade
- Não depender de componentes visuais, widgets, formulários ou APIs específicas de um agente.
- Quando houver suporte a pergunta estruturada, usar esse mecanismo sem alterar o conteúdo das opções.
- Quando não houver suporte a pergunta estruturada, usar opções `A`, `B`, `C`, `D` em texto puro.
- Não gerar código, backlog, tasks, PRD, épicos, user stories ou arquitetura final.
- Registrar toda decisão tomada no `transcript.md` e refletir o estado consolidado no `decision-brief.md`.

## Procedimentos

**Step 1: Inicializar bundle**
1. Identificar um título curto para a decisão a partir do pedido do usuário.
2. Executar `python3 scripts/slugify.py "<titulo>"` para normalizar o slug.
3. Verificar se `./discoveries/brainstorm-<slug>/` já existe. Se existir, perguntar em múltipla escolha se deve reaproveitar, criar novo com sufixo ou cancelar.
4. Executar `python3 scripts/init-bundle.py <slug>` para criar o bundle.
5. Encerrar com `blocked` se houver conflito de diretório, permissão negada ou slug inválido.

**Step 2: Rodada 1 obrigatória - entendimento do problema**
1. Ler `references/clarification-rounds.md`.
2. Formular de 3 a 4 perguntas de múltipla escolha sobre problema, objetivo, impacto esperado e risco de não decidir.
3. Explicitar em cada opção a implicação prática da escolha.
4. Registrar perguntas e respostas em `transcript.md` no bloco `## Rodada 1 - Entendimento do Problema`.
5. Atualizar `assumptions.md` com hipóteses iniciais e evidências conhecidas.

**Step 3: Rodada 2 obrigatória - escopo e restrições**
1. Formular de 3 a 4 perguntas de múltipla escolha sobre escopo inicial, fora de escopo, restrições de prazo, orçamento, compliance, time e operação.
2. Separar restrições confirmadas de preferências negociáveis.
3. Registrar tudo em `transcript.md` no bloco `## Rodada 2 - Escopo e Restrições`.

**Step 4: Rodada 3 obrigatória - alternativas**
1. Ler `references/evaluation-framework.md`.
2. Gerar de 3 a 5 alternativas comparáveis. Se o usuário sugerir uma solução, incluir essa opção e pelo menos duas alternativas reais.
3. Para pedidos como "quero microserviços", comparar no mínimo monólito modular, monólito distribuível, microserviços e arquitetura híbrida.
4. Registrar alternativas e racional inicial no `transcript.md` no bloco `## Rodada 3 - Alternativas`.
5. Não descartar alternativa por preferência do agente; descartar apenas por restrição explícita ou inviabilidade justificada.

**Step 5: Rodada 4 obrigatória - trade-offs**
1. Avaliar cada alternativa em `option-scorecard.md` com notas de 1 a 5 para complexidade, tempo de entrega, custo, escalabilidade, segurança, confiabilidade, observabilidade, manutenibilidade e risco operacional.
2. Analisar viabilidade técnica, operacional e financeira, segurança, escalabilidade, observabilidade, complexidade organizacional, dependências externas, tempo de implementação e capacidade da equipe.
3. Formular perguntas que forcem escolha entre trade-offs reais, como velocidade versus robustez, custo versus escala ou simplicidade versus autonomia.
4. Registrar tudo em `transcript.md` no bloco `## Rodada 4 - Trade-offs`.

**Step 6: Rodada 5 obrigatória - seleção de direção**
1. Apresentar síntese curta com alternativas, riscos relevantes, custo relativo, impacto operacional e recomendação preliminar.
2. Perguntar em múltipla escolha se o usuário confirma a alternativa recomendada, escolhe outra alternativa ou exige nova rodada por risco pendente.
3. Registrar a decisão explícita do usuário no `transcript.md` no bloco `## Rodada 5 - Seleção de Direção`.
4. Se a decisão não for explícita, encerrar com `needs_input`.

**Step 7: Abrir rodadas adicionais enquanto houver risco material**
1. Ler `references/readiness-gates.md`.
2. Avaliar se falta alternativa comparativa, análise de risco, análise operacional, análise financeira, recomendação explícita ou justificativa explícita.
3. Se faltar, abrir Rodada 6+ focada apenas nos pontos pendentes.
4. Não materializar decisão pronta enquanto houver risco material não analisado.

**Step 8: Materializar artefatos finais**
1. Ler `assets/decision-brief-template.md`, `assets/assumptions-template.md` e `assets/option-scorecard-template.md`.
2. Preencher todos os arquivos com base nas respostas e materiais coletados.
3. Não inventar fatos ausentes; registrar lacunas em `Decisões Pendentes`.
4. Atualizar `bundle.json` com título, status, alternativas avaliadas, alternativa recomendada, decisões e blockers.

**Step 9: Validar e corrigir**
1. Executar `python3 scripts/validate-bundle.py ./discoveries/brainstorm-<slug>`.
2. Se houver erro, ler o stderr, corrigir a seção indicada e reexecutar.
3. Encerrar com `blocked` se a validação continuar falhando após uma rodada honesta de correção.

**Step 10: Relatar handoff**
1. Informar o caminho do bundle gerado.
2. Resumir em até 6 bullets a decisão, alternativa recomendada, trade-offs aceitos, riscos residuais e próximo passo.
3. Recomendar a próxima skill conforme o caso: `technical-discovery-production`, `epic-story-discovery` ou `tracker-to-prd`.
4. Não executar a próxima skill automaticamente.

## Decisões Operacionais
1. Tratar solução sugerida pelo usuário como hipótese, não como decisão.
2. Preferir premissa explícita e auditável a inferência forte.
3. Comparar alternativas em termos de produção, custo e operação, não apenas preferência arquitetural.
4. Manter scorecard determinístico com escala fixa de 1 a 5.
5. Preservar nomes de sistemas, termos de negócio e restrições informadas.
6. Registrar materiais ausentes como risco de decisão.
7. Encerrar com `needs_input` se o usuário não confirmar uma direção.

## Estados Finais
- `done`: bundle materializado, decisão explícita registrada e validação concluída com `SUCCESS`.
- `needs_input`: falta decisão explícita, resposta de clarificação ou insumo indispensável.
- `blocked`: erro de I/O, conflito de diretório, falha persistente de validação ou impossibilidade de criar bundle.
- `failed`: erro inesperado após tentativa de recuperação.

## Tratamento de Erros
- Se `scripts/slugify.py` retornar slug vazio, pedir um título curto e repetir.
- Se `scripts/init-bundle.py` falhar por diretório existente, oferecer reaproveitar, versionar ou cancelar.
- Se houver menos de 3 alternativas viáveis, registrar bloqueio e abrir nova rodada para ampliar o espaço de solução.
- Se o usuário insistir em uma única solução sem comparação, explicar que a skill exige alternativas comparativas e abrir Rodada 3 novamente.
- Se `scripts/validate-bundle.py` apontar seção vazia ou placeholder, completar com evidência ou registrar lacuna não bloqueante quando permitido pelos gates.
- Se faltar risco, custo, impacto operacional, trade-off ou justificativa, voltar à rodada correspondente antes de validar.
