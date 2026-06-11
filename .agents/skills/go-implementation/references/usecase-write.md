# Use Case Write

<!-- TL;DR
Escrita em application usa Command Object em linguagem ubiqua, concentra regra de negocio e deixa adapters finos.
Keywords: usecase, command, write, application
Load complete when: tarefa altera caso de uso com efeito colateral persistente.
-->

## Objetivo
Padronizar use cases de escrita com responsabilidade clara e baixo falso positivo.

## Regras
- Operacoes de escrita recebem Command Object concreto quando a mudanca introduzir ou alterar efeito colateral persistente.
- O Command deve usar vocabulos do dominio (`IssuedAt`, `OccurredAt`, `RequestedBy`) e nao nomes tecnicos vagos.
- O use case decide trigger de persistencia, publicacao e branching de negocio.
- Adapters apenas montam input, chamam o use case e traduzem erro/saida.
- `clock.Clock` nao entra no use case; o instante chega pelo command ou parametro de dominio apropriado.

## Exemplo
```go
type ActivateSubscriptionCommand struct {
    SubscriptionID string
    ApprovedAt     time.Time
}

type ActivateSubscriptionUseCase struct {
    repo subscriptionRepository
}

func NewActivateSubscriptionUseCase(repo subscriptionRepository) *ActivateSubscriptionUseCase {
    return &ActivateSubscriptionUseCase{repo: repo}
}

func (uc *ActivateSubscriptionUseCase) Execute(ctx context.Context, cmd ActivateSubscriptionCommand) error {
    subscription, err := uc.repo.FindByID(ctx, cmd.SubscriptionID)
    if err != nil {
        return fmt.Errorf("buscar assinatura: %w", err)
    }
    if err := subscription.Activate(cmd.ApprovedAt); err != nil {
        return err
    }
    if err := uc.repo.Save(ctx, subscription); err != nil {
        return fmt.Errorf("salvar assinatura: %w", err)
    }
    return nil
}
```

Exemplo ruim:
- handler decidindo se ativa/cancela pela entidade antes de chamar use case;
- use case recebendo `rawID string, now time.Time`;
- repositorio publicado diretamente pelo adapter.

## Validacao Minima
- `go test -count=1 -race` no pacote alterado quando houver concorrencia ou state mutation relevante.
- `go build` e `go vet` no bounded context alterado quando a escrita tocar multiplos pacotes do modulo.

## Proibido
- Regras de negocio em handler, consumer, job ou producer.
- Command Object tecnico sem linguagem ubiqua.
- Side effect decidido fora da camada application.
