package media

type resolveMediaResponse struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	FileSize int64  `json:"file_size"`
	ID       string `json:"id"`
}

type mediaErrorResponse struct {
	Error mediaErrorDetail `json:"error"`
}

type mediaErrorDetail struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Code      int    `json:"code"`
	ErrorData any    `json:"error_data,omitempty"`
}
