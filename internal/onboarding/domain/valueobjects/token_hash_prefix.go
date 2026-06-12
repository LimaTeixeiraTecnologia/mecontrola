package valueobjects

import "encoding/hex"

const tokenHashPrefixLen = 8

func TokenHashPrefix(hash []byte) string {
	if len(hash) == 0 {
		return ""
	}
	h := hex.EncodeToString(hash)
	if len(h) > tokenHashPrefixLen {
		return h[:tokenHashPrefixLen]
	}
	return h
}
