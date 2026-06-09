package handlers

import (
	"net/http"
	"strconv"
	"strings"
)

type etagHelper struct{}

func newETagHelper() *etagHelper {
	return &etagHelper{}
}

func (h *etagHelper) header(version int64) string {
	return "\"v" + strconv.FormatInt(version, 10) + "\""
}

func (h *etagHelper) parse(etag string) int64 {
	etag = strings.TrimSpace(etag)
	etag = strings.Trim(etag, "\"")
	if strings.HasPrefix(etag, "v") {
		version, err := strconv.ParseInt(etag[1:], 10, 64)
		if err != nil {
			return 0
		}
		return version
	}
	version, err := strconv.ParseInt(etag, 10, 64)
	if err != nil {
		return 0
	}
	return version
}

func (h *etagHelper) checkIfNoneMatch(r *http.Request, currentVersion int64) bool {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}
	return h.parse(ifNoneMatch) == currentVersion
}

func formatETag(version int64) string {
	return newETagHelper().header(version)
}
