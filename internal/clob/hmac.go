package clob

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

func BuildHMACSignature(secret string, timestamp int64, method, requestPath string, body string) (string, error) {
	// py_clob_client: base64.urlsafe_b64decode(secret)
	decoded, err := base64.URLEncoding.DecodeString(secret)
	if err != nil {
		return "", err
	}

	msg := strings.Builder{}
	msg.WriteString(int64ToString(timestamp))
	msg.WriteString(method)
	msg.WriteString(requestPath)
	if body != "" {
		// NOTE: match python quirk: replace single quotes with double quotes.
		msg.WriteString(strings.ReplaceAll(body, "'", `"`))
	}

	mac := hmac.New(sha256.New, decoded)
	_, _ = mac.Write([]byte(msg.String()))
	sum := mac.Sum(nil)
	return base64.URLEncoding.EncodeToString(sum), nil
}

func int64ToString(v int64) string {
	// tiny helper to avoid strconv import everywhere in this package
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := v < 0
	if neg {
		v = -v
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
