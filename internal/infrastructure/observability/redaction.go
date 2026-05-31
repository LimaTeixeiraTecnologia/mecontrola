package observability

// PIIFields lista os nomes de campos que contêm PII (Personally Identifiable Information)
// e devem ser redatados antes de emitir qualquer trace, span ou log.
//
// Redaction enforced via pii_handler.go (NewRedactingSlogHandler), que deve ser instalado
// como handler slog global em NewProvider, e via devkit-go/pkg/observability/otel com Sanitize=true.
// Para adicionar campos, estender esta slice — o handler recarregará na próxima chamada de Handle.
//
// Invariante: nenhum campo listado aqui pode aparecer em claro em telemetria exportada.
var PIIFields = []string{
	"phone",
	"password",
	"token",
	"card_number",
	"amount",
}
