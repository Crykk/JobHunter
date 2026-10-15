package parse

import (
	"net/http"
	"strings"
)

// LooksLikeCloudflare reports challenge / block pages so we fail fast
// instead of trying to parse or solve them.
func LooksLikeCloudflare(header http.Header, body []byte) bool {
	if header != nil {
		if header.Get("CF-Mitigated") != "" {
			return true
		}
		server := strings.ToLower(header.Get("Server"))
		if strings.Contains(server, "cloudflare") && (header.Get("CF-RAY") != "" || looksLikeChallengeBody(body)) {
			// CF-RAY is present on normal Duunitori pages too; only treat as a
			// challenge when the body looks like the interstitial.
			if looksLikeChallengeBody(body) {
				return true
			}
		}
	}
	return looksLikeChallengeBody(body)
}

func looksLikeChallengeBody(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	s := strings.ToLower(string(body))
	markers := []string{
		"cdn-cgi/challenge-platform",
		"cf-challenge",
		"cf-browser-verification",
		"attention required! | cloudflare",
		"enable javascript and cookies to continue",
		"checking your browser before accessing",
		"just a moment...",
	}
	for _, m := range markers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
