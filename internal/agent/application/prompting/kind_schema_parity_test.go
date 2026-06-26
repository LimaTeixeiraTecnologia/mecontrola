package prompting_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/prompting"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type KindSchemaParitySuite struct {
	suite.Suite
}

func TestKindSchemaParitySuite(t *testing.T) {
	suite.Run(t, new(KindSchemaParitySuite))
}

func parityKinds() []intent.Kind {
	kinds := make([]intent.Kind, 0, int(intent.KindClassifyCategory))
	for k := intent.KindUnknown; k <= intent.KindClassifyCategory; k++ {
		kinds = append(kinds, k)
	}
	return kinds
}

func schemaKindSlugs() ([]string, error) {
	schema := prompting.ParseIntentJSONSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema: campo 'properties' ausente ou tipo inesperado")
	}
	kindProp, ok := props["kind"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema: campo 'kind' ausente ou tipo inesperado")
	}
	rawEnum, ok := kindProp["enum"]
	if !ok {
		return nil, fmt.Errorf("schema: campo 'kind.enum' ausente")
	}
	switch v := rawEnum.(type) {
	case []string:
		return v, nil
	case []any:
		slugs := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("schema: enum[%d] não é string: %T", i, item)
			}
			slugs = append(slugs, s)
		}
		return slugs, nil
	default:
		return nil, fmt.Errorf("schema: 'kind.enum' tipo inesperado: %T", rawEnum)
	}
}

func (s *KindSchemaParitySuite) TestParityKindToSchema() {
	slugs, err := schemaKindSlugs()
	s.Require().NoError(err, "falha ao extrair enum do schema JSON")

	schemaSet := make(map[string]struct{}, len(slugs))
	for _, sl := range slugs {
		schemaSet[sl] = struct{}{}
	}

	for _, k := range parityKinds() {
		slug := k.String()
		_, exists := schemaSet[slug]
		s.True(exists, "kind %d (%q) não está presente no enum de ParseIntentJSONSchema", k, slug)
	}
}

func (s *KindSchemaParitySuite) TestParitySchemaToKind() {
	slugs, err := schemaKindSlugs()
	s.Require().NoError(err, "falha ao extrair enum do schema JSON")

	for _, slug := range slugs {
		k, err := intent.ParseKind(slug)
		s.NoError(err, "slug %q do schema não é reconhecido por ParseKind", slug)
		s.NotEqual(intent.Kind(0), k, "slug %q resultou em Kind zero", slug)
		s.Equal(slug, k.String(), "String() diverge do slug original: kind=%d got=%q want=%q", k, k.String(), slug)
	}
}
