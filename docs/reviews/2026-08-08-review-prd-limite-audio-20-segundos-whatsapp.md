# Review prompt — PRD limite de audio 20 segundos WhatsApp

- **Data:** 2026-08-08
- **PRD alvo:** `.specs/prd-limite-audio-20-segundos-whatsapp`
- **Contrato base:** `AGENTS.md` lido antes do enriquecimento
- **Skill usada para este artefato:** `prompt-enricher`

## Prompt original

```text
Execute @.claude/skills/review/ de forma criteriosa e sem flexibilização, validando estritamente contra .specs/prd-limite-audio-20-segundos-whatsapp
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
```

## Ambiguidades resolvidas no enriquecimento

1. O alvo de validação foi explicitado como o **diretório inteiro da spec**, não apenas `prd.md`.
2. O critério de saída foi tornado **determinístico**: somente `APPROVED` encerra o fluxo; `APPROVED_WITH_REMARKS`, `BLOCKED` e `REJECTED` não encerram.
3. Toda divergência passou a exigir **rastreabilidade bidirecional** entre requisito, evidência no código/diff e bug emitido para `bugfix`.
4. `Não verificável pelo diff` foi convertido em **falha bloqueante**, porque o objetivo exige `0 gaps`, `0 lacunas` e `0 ressalvas`.

## Prompt enriquecido

```xml
<goal>
Obter veredito final APPROVED para a implementacao do PRD "limite-audio-20-segundos-whatsapp", com conformidade total e comprovada contra a especificacao, sem falsos positivos, sem gaps, sem lacunas e sem ressalvas.
</goal>

<context>
  <repository>/Users/jailtonjunior/Git/mecontrola</repository>
  <canonical_instructions>AGENTS.md</canonical_instructions>
  <review_skill>@.claude/skills/review/</review_skill>
  <bugfix_skill>@.claude/skills/bugfix/</bugfix_skill>
  <spec_directory>.specs/prd-limite-audio-20-segundos-whatsapp</spec_directory>
  <spec_artifacts>
    <artifact>.specs/prd-limite-audio-20-segundos-whatsapp/prd.md</artifact>
    <artifact>.specs/prd-limite-audio-20-segundos-whatsapp/techspec.md</artifact>
    <artifact>.specs/prd-limite-audio-20-segundos-whatsapp/tasks.md</artifact>
    <artifact>.specs/prd-limite-audio-20-segundos-whatsapp/task-*.md</artifact>
    <artifact>.specs/prd-limite-audio-20-segundos-whatsapp/adr-*.md</artifact>
  </spec_artifacts>
  <scope_of_validation>
    Revisar a implementacao da branch/worktree atual contra a base apropriada do repositório e confrontar estritamente o diff e o codigo resultante com toda a especificacao acima.
  </scope_of_validation>
</context>

<task>
  <step>Carregue AGENTS.md e execute a skill review de forma estrita, usando a base correta do diff para a primeira rodada.</step>
  <step>Confronte a implementacao contra todo o conteudo da spec, incluindo PRD, techspec, tasks e ADRs, sem limitar a revisao apenas aos arquivos citados no diff.</step>
  <step>Monte uma matriz completa de rastreabilidade requisito -> implementacao -> teste/evidencia.</step>
  <step>Valide, no minimo, todos os RFs, criterios de aceite, DoD, regras de negocio, constraints tecnicas e itens explicitamente fora de escopo que nao podem ter sido violados.</step>
  <step>Se houver qualquer problema, gere findings com severidade canonica, converta os bugs acionaveis para o formato canonico exigido pela skill bugfix e execute o ciclo review -> bugfix -> review ate eliminar todos os achados.</step>
  <step>Apos cada remediacao, rode nova revisao. Antes de encerrar, execute uma revisao final completa do escopo inteiro para garantir que nao sobraram regressões nem lacunas fora do delta da ultima correcao.</step>
  <step>Dispare subagentes especializados quando isso aumentar a qualidade da revisao, especialmente para trilhas independentes, verificacao cruzada de requisitos, testes e regressões.</step>
</task>

<rules>
  <rule>Aplicar a skill review sem flexibilizacao.</rule>
  <rule>Aplicar a skill bugfix para todo achado bloqueante ou material.</rule>
  <rule>Somente encerrar com verdict APPROVED.</rule>
  <rule>APPROVED_WITH_REMARKS nao e aceitavel como estado final.</rule>
  <rule>BLOCKED nao e aceitavel como estado final, salvo impossibilidade externa real e comprovada.</rule>
  <rule>Qualquer criterio de aceite nao implementado e falha bloqueante.</rule>
  <rule>Qualquer item do DoD nao implementado e falha bloqueante.</rule>
  <rule>Qualquer regra de negocio ausente, parcial ou incorreta e falha bloqueante.</rule>
  <rule>Qualquer requisito "nao verificavel pelo diff" deve ser tratado como gap e, portanto, como falha bloqueante ate que haja evidencia suficiente.</rule>
  <rule>Zero gaps.</rule>
  <rule>Zero lacunas.</rule>
  <rule>Zero falsos positivos.</rule>
  <rule>Zero ressalvas.</rule>
  <rule>Nao inventar requisitos, comportamento esperado, arquitetura ou evidencias ausentes.</rule>
  <rule>Cada finding deve citar requisito violado, arquivo, linha, impacto e hint de correcao.</rule>
  <rule>Cada finding deve estar ancorado em evidencia primaria do codigo, diff, teste ou documento da spec.</rule>
  <rule>Se uma suspeita nao puder ser comprovada com evidencia suficiente, nao a reporte como bug.</rule>
</rules>

<acceptance_criteria>
  <item>Todos os criterios de aceite da spec estao implementados e evidenciados.</item>
  <item>DoD 100% atendido e evidenciado.</item>
  <item>Todas as regras de negocio estao implementadas exatamente como especificadas.</item>
  <item>Nenhum gap, lacuna, ressalva ou risco residual permanece aberto no estado final.</item>
  <item>Nenhum falso positivo aparece no relatorio final.</item>
  <item>O veredito final e APPROVED.</item>
</acceptance_criteria>

<workflow>
  <phase name="review-inicial">
    Execute a review completa contra a base correta e a spec inteira.
  </phase>
  <phase name="remediacao-condicional">
    Se houver findings, emita a lista canonica de bugs e execute bugfix focado na causa raiz de cada item.
  </phase>
  <phase name="review-pos-bugfix">
    Reavalie a remediacao e confirme a eliminacao do achado sem introduzir regressao.
  </phase>
  <phase name="review-final-completa">
    Refaça uma passada final completa contra todo o escopo da spec. So aceite APPROVED sem findings, sem residual_risks e sem observacoes pendentes.
  </phase>
</workflow>

<output>
  <format>markdown</format>
  <required_sections>
    <section>verdict</section>
    <section>files_reviewed</section>
    <section>refs_loaded</section>
    <section>traceability_matrix</section>
    <section>findings</section>
    <section>bugs_for_bugfix</section>
    <section>validations_run</section>
    <section>residual_risks</section>
    <section>final_decision</section>
  </required_sections>
  <traceability_matrix_shape>
    Para cada item relevante da spec, informe: artefato, identificador/nome do criterio, status (atendido ou nao atendido), evidencia objetiva e arquivos/linhas correspondentes.
  </traceability_matrix_shape>
  <final_state_contract>
    O estado final aceitavel e exclusivamente APPROVED, com findings = [], bugs_for_bugfix = [], residual_risks = [] e confirmacao explicita de 0 gaps, 0 lacunas, 0 falsos positivos e 0 ressalvas.
  </final_state_contract>
</output>
```

## Justificativa das adições

| Adição | Motivo |
|---|---|
| Escopo expandido para PRD + techspec + tasks + ADRs | Evita validar só uma parte da especificação. |
| Estado final somente `APPROVED` | Elimina ambiguidade sobre `APPROVED_WITH_REMARKS`. |
| Matriz de rastreabilidade obrigatória | Força prova objetiva de cobertura e reduz falso positivo. |
| `Não verificável` tratado como falha | Compatível com exigência de zero gaps/lacunas. |
| Revisão final completa após bugfix | Evita aprovar apenas o delta da última remediação. |
| Saída estruturada com bugs canônicos | Facilita o ciclo determinístico review -> bugfix -> review. |

## Variante recomendada

- **Usar exatamente o prompt enriquecido acima**, sem enxugar critérios, porque o objetivo declarado exige revisão determinística e evidência completa, não apenas uma auditoria parcial do diff.
