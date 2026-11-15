package xtools

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// oauth1Credentials hold the OAuth 1.0a user-context keys.
type oauth1Credentials struct {
	consumerKey, consumerSecret, token, tokenSecret string
}

// percentEncode implements RFC 3986 encoding as required by OAuth 1.0a.
func percentEncode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteString("%" + strings.ToUpper(hex.EncodeToString([]byte{c})))
		}
	}
	return b.String()
}

func newNonce() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// authorization returns the OAuth 1.0a HMAC-SHA1 Authorization header for a
// request. rawURL may contain a query string; form body parameters go in
// form (JSON bodies are not signed).
func (c oauth1Credentials) authorization(method, rawURL string, form url.Values, nonce string, ts time.Time) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	oauth := map[string]string{
		"oauth_consumer_key":     c.consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(ts.Unix(), 10),
		"oauth_token":            c.token,
		"oauth_version":          "1.0",
	}
	var pairs [][2]string
	for k, v := range oauth {
		pairs = append(pairs, [2]string{percentEncode(k), percentEncode(v)})
	}
	for _, vals := range []url.Values{u.Query(), form} {
		for k, vs := range vals {
			for _, v := range vs {
				pairs = append(pairs, [2]string{percentEncode(k), percentEncode(v)})
			}
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}
		return pairs[i][1] < pairs[j][1]
	})
	joined := make([]string, len(pairs))
	for i, p := range pairs {
		joined[i] = p[0] + "=" + p[1]
	}
	base := strings.ToUpper(method) + "&" + percentEncode(u.Scheme+"://"+u.Host+u.EscapedPath()) + "&" + percentEncode(strings.Join(joined, "&"))
	mac := hmac.New(sha1.New, []byte(percentEncode(c.consumerSecret)+"&"+percentEncode(c.tokenSecret)))
	mac.Write([]byte(base))
	oauth["oauth_signature"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))

	keys := make([]string, 0, len(oauth))
	for k := range oauth {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, percentEncode(k)+`="`+percentEncode(oauth[k])+`"`)
	}
	return "OAuth " + strings.Join(parts, ", "), nil
}
