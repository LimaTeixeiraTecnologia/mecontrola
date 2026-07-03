# Tarefa 2.0: Ensaio de restore PITR e restore de VPS com evidência

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Os runbooks de restore existem mas **nunca foram executados** ("atualizar após primeiro restore real"). DR não testado = DR inexistente. Executar os dois ensaios em ambiente isolado, medir RTO real e comprovar integridade.

<requirements>
- RF-04: ensaio de restore PITR isolado, RTO medido, integridade validada, evidência anexada.
- RF-05: ensaio de restore completo de VPS a partir do S3, evidência anexada.
- RF-06: runbooks atualizados com RPO/RTO reais e SLO de recuperação do envelope B.
</requirements>

## Subtarefas

- [ ] 2.1 Restore PITR em ambiente descartável: `pgbackrest --type=time --target=<ts> restore`, subir Postgres, validar integridade, cronometrar RTO.
- [ ] 2.2 Restore completo de VPS seguindo `restore-vps.md`: provisionar host, restaurar secrets (`backup-env-s3.sh`), restaurar banco, subir stack, cronometrar RTO.
- [ ] 2.3 Registrar RPO real (derivado de archive_timeout/WAL) e RTO medido; atualizar `restore-pitr.md` e `restore-vps.md`.
- [ ] 2.4 Anexar relatório de evidência em `docs/runs/`.

## Detalhes de Implementação

Ver `techspec.md` REQ-02. Nenhuma alteração em produção — apenas ambiente de ensaio. Depende da cadeia de backup confirmada na Tarefa 1.0.

## Critérios de Sucesso

- Restore PITR concluído com banco íntegro e RTO medido, dentro do SLO declarado.
- Restore de VPS reproduzido do zero com stack saudável e RTO medido.
- Runbooks refletem números reais; nenhum campo permanece "atualizar após primeiro restore".

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários (não aplicável a código; validação por checklist de integridade documentada)
- [ ] Testes de integração (restore PITR e restore de VPS executados end-to-end com evidência)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `deployment/runbooks/restore-pitr.md`, `deployment/runbooks/restore-vps.md`
- `deployment/scripts/backup-env-s3.sh`, `deployment/pgbackrest/pgbackrest.conf`
- `deployment/docker/Dockerfile.postgres`
- `docs/runs/<data>-evidencia-restore.md` (novo)
