package media

type MediaDescriptor struct {
	URL      string
	MimeType string
	SHA256   string
	Size     int64
}

type DownloadedMedia struct {
	Bytes    []byte
	MimeType string
	SHA256   string
	Size     int64
}
