package idempotency

import (
	"bytes"
	"net/http"
)

const maxResponseBodySize = 64 * 1024

type responseRecorder struct {
	http.ResponseWriter
	status   int
	buf      bytes.Buffer
	overflow bool
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w, status: http.StatusOK}
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.overflow {
		return len(b), nil
	}
	if r.buf.Len()+len(b) > maxResponseBodySize {
		r.overflow = true
		return len(b), nil
	}
	return r.buf.Write(b)
}

func (r *responseRecorder) flush() {
	r.ResponseWriter.WriteHeader(r.status)
	_, _ = r.ResponseWriter.Write(r.buf.Bytes())
}
