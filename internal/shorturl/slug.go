package shorturl

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

func NormalizeSlug(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "-")
	value = strings.ReplaceAll(value, "/", "-")
	if value == "" {
		return "", fmt.Errorf("slug is empty")
	}
	if strings.ContainsAny(value, ":?#[]@") {
		return "", fmt.Errorf("slug contains reserved characters")
	}
	return value, nil
}

func NormalizeTag(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func NormalizeTags(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := NormalizeTag(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func GenerateCode(length int, chars string) (string, error) {
	if length <= 0 || chars == "" {
		return "", fmt.Errorf("invalid short code parameters")
	}
	result := make([]byte, length)
	limit := byte(256 - (256 % len(chars)))
	for i := 0; i < length; {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		if b[0] >= limit {
			continue
		}
		result[i] = chars[int(b[0])%len(chars)]
		i++
	}
	return string(result), nil
}

func ValidLongURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" {
		return false
	}
	return true
}

func ValidAuthority(authority string) bool {
	authority = strings.TrimSpace(authority)
	if authority == "" || strings.ContainsAny(authority, "/\\ ?#") {
		return false
	}
	host := authority
	if strings.Contains(authority, ":") {
		h, port, err := net.SplitHostPort(authority)
		if err != nil {
			return false
		}
		host = h
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return false
		}
	}
	if net.ParseIP(host) != nil {
		return false
	}
	// Reject dotted-quad strings whose octets are out of range. They are not
	// valid IPs, but they also are not valid domain names.
	labels := strings.Split(host, ".")
	if len(labels) == 4 {
		allNumeric := true
		for _, label := range labels {
			if label == "" {
				allNumeric = false
				break
			}
			for _, r := range label {
				if r < '0' || r > '9' {
					allNumeric = false
					break
				}
			}
			if !allNumeric {
				break
			}
		}
		if allNumeric {
			return false
		}
	}
	if host == "localhost" || !strings.Contains(host, ".") {
		return false
	}
	if strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	return true
}

func CropTitle(title *string) *string {
	if title == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*title)
	if utf8.RuneCountInString(trimmed) > 512 {
		trimmed = string([]rune(trimmed)[:512])
	}
	return &trimmed
}
