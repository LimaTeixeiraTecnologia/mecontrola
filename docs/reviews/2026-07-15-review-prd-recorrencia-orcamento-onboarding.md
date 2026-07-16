# Review Prompt Enriquecido

## Prompt original

```text
Use @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-recorrencia-orcamento-onboarding
Critérios obrigatórios:
* Todos os critérios de aceite atendidos (implementados).
* DoD 100% atendido (implementados).
* 0 gaps.
* 0 lacunas.
* 0 falsos positivos.
* 0 ressalvas
* Todas Regras de negócio atendidos (implementados)
Caso encontre qualquer problema, utilize @.claude/skills/bugfix/ e repita o ciclo review → bugfix → review até obter APPROVED, sem falsos positivos e em conformidade total com a especificação.
Dispare subagentes especializados quando agregarem qualidade à revisão.
Não implemente nada. Apenas crie/enriqueça o prompt e salve o arquivo em docs/reviews/.
```

## Prompt enriquecido

```xml
<goal>
Obter um veredito final `APPROVED` para a implementacao relacionada a `.specs/prd-recorrencia-orcamento-onboarding/`, com conformidade total e comprovada contra PRD, techspec, ADRs, criterios de aceite, DoD e regras de negocio, sem falsos positivos, sem lacunas, sem gaps e sem ressalvas.
</goal>

<context>
- Repositorio local: `/Users/jailtonjunior/Git/mecontrola`
- Especificacao alvo: `.specs/prd-recorrencia-orcamento-onboarding/`
- Documentos minimos obrigatorios:
  - `.specs/prd-recorrencia-orcamento-onboarding/prd.md`
  - `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
  - todos os ADRs presentes em `.specs/prd-recorrencia-orcamento-onboarding/`
- Documento complementar relevante:
  - `.specs/prd-recorrencia-orcamento-onboarding/tasks.md`
- Skill principal de revisao: `@.claude/skills/review/`
- Skill obrigatoria de remediacao quando houver achados acionaveis: `@.claude/skills/bugfix/`
- Use subagentes especializados quando agregarem qualidade real, reduzirem risco de falso positivo ou melhorarem cobertura da confrontacao com a especificacao.
</context>

<task>
Execute a skill `@.claude/skills/review/` de forma estrita e deterministica contra a implementacao atual relacionada a `.specs/prd-recorrencia-orcamento-onboarding/`.

Se a revisao identificar qualquer desvio, bug, criterio nao atendido, DoD incompleto, regra de negocio ausente, evidencia insuficiente ou risco residual impeditivo, acione `@.claude/skills/bugfix/` com os achados no formato canonico exigido pela skill e repita o ciclo `review -> bugfix -> review` ate que o resultado final seja `APPROVED`.

Somente encerre quando a conformidade estiver integralmente comprovada por evidencias objetivas no codigo, testes e validacoes executadas.
</task>

<rules>
- Nao flexibilize criterios. Ausencia de evidencia vale como nao conformidade.
- Nao assuma implementacao por intuicao, naming ou proximidade semantica. Comprove no diff, no codigo atual, nos testes e nas validacoes.
- Nao aceite falsos positivos. Se houver incerteza, investigue mais antes de concluir.
- Nao aceite falsos negativos. Se um criterio estiver parcialmente atendido, trate como nao atendido.
- Nao produza `APPROVED_WITH_REMARKS`. O alvo final aceitavel deste fluxo e somente `APPROVED`.
- Nao pare em `BLOCKED` sem explicitar exatamente o bloqueio e por que ele impede prova de conformidade.
- Nao omita criterio de aceite, item de DoD, requisito funcional, regra de negocio, ADR aplicavel ou impacto transversal relevante.
- Nao implemente mudancas diretamente durante a revisao. Use a skill `bugfix` quando houver correcao necessaria dentro do ciclo.
- Se houver alteracoes locais sujas nao relacionadas, isole o escopo explicitamente e registre a limitacao.
- Se a investigacao exigir leitura ampla, use subagentes para economia de contexto e maior cobertura, retornando apenas conclusoes verificadas.
- O fluxo deve respeitar o procedimento da skill `review`, inclusive o confronto incondicional dos criterios de aceite da tarefa quando a task file estiver disponivel.
</rules>

<acceptance_criteria>
- Todos os criterios de aceite do PRD estao implementados e comprovados com evidencia objetiva.
- Todo o DoD da especificacao esta 100% implementado e comprovado.
- Todas as regras de negocio descritas no PRD, techspec, ADRs e tasks aplicaveis estao implementadas e comprovadas.
- Existem `0 gaps`.
- Existem `0 lacunas`.
- Existem `0 falsos positivos`.
- Existem `0 ressalvas`.
- Nao restam riscos residuais impeditivos.
- O veredito final obrigatorio e `APPROVED`.
</acceptance_criteria>

<required_process>
1. Carregue a skill `@.claude/skills/review/` e cumpra integralmente seu procedimento.
2. Leia obrigatoriamente `prd.md`, `techspec.md`, `tasks.md` e todos os ADRs em `.specs/prd-recorrencia-orcamento-onboarding/`.
3. Extraia e liste explicitamente:
   - todos os criterios de aceite
   - todos os itens de DoD
   - todas as regras de negocio
   - todas as decisoes arquiteturais e restricoes relevantes dos ADRs
   - todas as obrigacoes de implementacao e cobertura descritas em `tasks.md`
4. Confronte cada item extraido contra a implementacao real, sem pular itens.
5. Para cada item, classifique apenas como:
   - `atendido`
   - `nao_atendido`
   - `nao_verificavel`
6. `nao_verificavel` deve ser tratado como impeditivo e nunca como aprovacao implicita.
7. Se existir qualquer item diferente de `atendido`, gere achados acionaveis com severidade apropriada e acione `@.claude/skills/bugfix/`.
8. A skill `bugfix` deve receber bugs no formato canonico exigido por ela.
9. Apos a remediacao, execute nova revisao completa ou revisao incremental conforme a skill `review` determinar, preservando rastreabilidade entre rodadas.
10. Repita o ciclo ate que todos os itens estejam `atendido` e o veredito seja `APPROVED`.
</required_process>

<subagents>
- Dispare subagente de especificacao para minerar PRD, techspec, ADRs e tasks em checklist verificavel quando isso melhorar cobertura.
- Dispare subagente de testes/validacao para confirmar se a evidencia executada realmente prova cada criterio.
- Dispare subagente de arquitetura/dominio quando houver regras de negocio, workflows, estados, resume retrocompativel, structured output ou observabilidade complexos.
- Nao use subagente sem ganho claro de qualidade.
</subagents>

<spec_focus>
Verifique com atencao redobrada os pontos que a especificacao de recorrencia explicita como obrigatorios:
- interpretacao por linguagem natural das tres decisoes: sem recorrencia, 12 meses padrao, ou quantidade especifica de 1 a 12 meses
- prioridade correta da quantidade valida sobre o padrao de 12 meses
- conversao de numeros por extenso para valor numerico
- zero recorrencia indevida para negativo, invalido ou ambiguo
- repergunta sem limite para invalido e ambiguo
- confirmacao imediata da decisao aplicada e reflexo no resumo final
- estado retrocompativel com quantidade de meses
- retomada transparente de onboardings suspensos sem migracao ou drenagem
- contador `agents_onboarding_recurrence_total` com `outcome` fechado e cardinalidade controlada
- gate real-LLM com aderencia >= 0,90 e 0 falso-sucesso
- ausencia de feature flag em runtime
</spec_focus>

<output_format>
Retorne em Markdown, em portugues do Brasil, com a seguinte estrutura minima:

1. `Status do ciclo`
   - rodada atual
   - veredito atual: `BLOCKED`, `REJECTED`, `APPROVED_WITH_REMARKS` ou `APPROVED`
   - objetivo final do ciclo: `APPROVED`

2. `Checklist de conformidade`
   - tabela com colunas: `item`, `categoria`, `status`, `evidencia`, `arquivos`
   - `categoria` deve ser uma de: `criterio_aceite`, `dod`, `regra_negocio`, `adr`, `techspec`, `task`
   - `status` deve ser um de: `atendido`, `nao_atendido`, `nao_verificavel`

3. `Achados`
   - lista de achados com `severity`, `file`, `line`, `impact`, `fix_hint`, `origem_especificacao`
   - se nao houver achados, declarar explicitamente `sem achados`

4. `Pacote canonico para bugfix`
   - incluir JSON ou bloco estruturado compativel com a skill `bugfix` para cada achado acionavel
   - omitir apenas quando o veredito ja for `APPROVED`

5. `Validacoes executadas`
   - listar comandos realmente executados
   - diferenciar validacoes passadas, falhas e nao executadas

6. `Decisao`
   - explicar objetivamente por que o veredito atual foi emitido
   - se ainda nao for `APPROVED`, declarar a proxima acao obrigatoria do ciclo

7. `Encerramento`
   - somente quando o veredito final for `APPROVED`, declarar explicitamente:
     - `todos os criterios de aceite implementados`
     - `DoD 100% atendido`
     - `todas as regras de negocio implementadas`
     - `0 gaps`
     - `0 lacunas`
     - `0 falsos positivos`
     - `0 ressalvas`
</output_format>

<example>
Exemplo de classificacao correta de um item:

Entrada:
- Regra de negocio: "quando houver quantidade valida entre 1 e 12, ela deve prevalecer sobre a interpretacao positiva padrao de 12 meses"

Saida esperada:
- `status: atendido` apenas se houver evidencia objetiva no codigo e, preferencialmente, teste cobrindo a prioridade da quantidade valida
- `status: nao_verificavel` se o comportamento parecer plausivel, mas a evidencia nao for suficiente
- `status: nao_atendido` se a implementacao estiver ausente, incompleta ou divergente
</example>

<uncertainty>
Se houver qualquer incerteza, declare-a explicitamente e trate-a como impeditiva para `APPROVED` ate que seja resolvida com evidencia.
</uncertainty>
```

## Justificativas das adicoes

| Adicao | Justificativa |
|---|---|
| Estrutura em XML | Atende a regra dura de prompts multi-parte do repositorio e separa objetivo, contexto, tarefa, regras e formato. |
| Escopo documental minimo | Evita revisao parcial e obriga confronto com `prd.md`, `techspec.md`, `tasks.md` e ADRs. |
| Processo iterativo explicito | Remove ambiguidade sobre quando chamar `bugfix` e quando encerrar o ciclo. |
| Criterios mensuraveis | Torna verificavel o requisito de `0 gaps`, `0 lacunas`, `0 falsos positivos` e `0 ressalvas`. |
| Regras de incerteza | Impede aprovacao por inferencia ou evidencia fraca. |
| Foco especifico da PRD | Direciona a revisao para os pontos mais sensiveis e mais faceis de validar errado nesta feature. |
| Formato de saida minimo | Facilita auditoria, rastreabilidade e handoff para `bugfix`. |
| Uso criterioso de subagentes | Mantem economia de contexto sem perder cobertura de revisao. |

## Variante enxuta

Use a versao abaixo apenas se voce quiser um prompt menor, com menos instrucao operacional e mais delegacao para a skill `review`:

```text
Execute `@.claude/skills/review/` contra `.specs/prd-recorrencia-orcamento-onboarding/`, lendo obrigatoriamente `prd.md`, `techspec.md`, `tasks.md` e todos os ADRs. Extraia todos os criterios de aceite, itens de DoD e regras de negocio e confronte cada item com a implementacao real, classificando apenas como `atendido`, `nao_atendido` ou `nao_verificavel`. Trate qualquer item diferente de `atendido` como impeditivo para aprovacao. Se houver achados, gere o pacote canonico para `@.claude/skills/bugfix/` e repita o ciclo `review -> bugfix -> review` ate obter `APPROVED`. Nao aceite falsos positivos, lacunas, gaps ou ressalvas. So encerre quando puder provar, com evidencias objetivas, que todos os criterios de aceite, DoD e regras de negocio foram 100% implementados.
```
