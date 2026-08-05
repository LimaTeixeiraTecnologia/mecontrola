package media

import "errors"

var (
	ErrMediaIDEmpty          = errors.New("whatsapp/media: media id vazio")
	ErrMediaURLMissing       = errors.New("whatsapp/media: url de download ausente na resposta da meta media api")
	ErrMediaAuth             = errors.New("whatsapp/media: autenticacao invalida na meta media api")
	ErrMediaBadRequest       = errors.New("whatsapp/media: requisicao invalida na meta media api")
	ErrMediaServer           = errors.New("whatsapp/media: erro interno da meta media api")
	ErrDownloadURLEmpty      = errors.New("whatsapp/media: url de download vazia")
	ErrDownloadTooLarge      = errors.New("whatsapp/media: download excede o limite de bytes configurado")
	ErrSHA256Mismatch        = errors.New("whatsapp/media: sha256 do download nao confere com o sha256 informado pela meta")
	ErrUnsupportedMimeType   = errors.New("whatsapp/media: mime type nao suportado para extracao deterministica de duracao")
	ErrDurationUndetermined  = errors.New("whatsapp/media: duracao do audio nao pode ser determinada de forma deterministica")
	ErrAudioDurationExceeded = errors.New("whatsapp/media: duracao do audio excede o limite maximo configurado")
)
