// Package fakes fornece um Client fake do platform httpclient para testes unitários
// que apenas precisam injetar um cliente sem orquestrar um httptest.Server.
//
// Para testes de integração realistas, prefira httptest.NewServer + httpclient.NewClient
// apontando para srv.URL.
package fakes

import (
	devkitfake "github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/httpclient"
)

// NewClient cria um *httpclient.Client com observabilidade no-op apontando para baseURL.
// Falha o teste via panic se a configuração for inválida — uso restrito a testes.
func NewClient(baseURL, target string, opts ...httpclient.Option) *httpclient.Client {
	allOpts := append([]httpclient.Option{
		httpclient.WithBaseURL(baseURL),
		httpclient.WithTarget(target),
	}, opts...)

	client, err := httpclient.NewClient(devkitfake.NewProvider(), allOpts...)
	if err != nil {
		panic("httpclient/fakes: falha ao criar fake client: " + err.Error())
	}
	return client
}
