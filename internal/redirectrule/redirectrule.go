package redirectrule

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"

	"github.com/hakimyu/shlink-go/internal/shorturl"
	"github.com/hakimyu/shlink-go/internal/ua"
)

type Condition struct {
	Type       string  `json:"type"`
	MatchKey   *string `json:"matchKey"`
	MatchValue *string `json:"matchValue"`
}

type Rule struct {
	LongURL    string      `json:"longUrl"`
	Priority   int         `json:"priority"`
	Conditions []Condition `json:"conditions"`
}

type Set struct {
	DefaultLongURL string `json:"defaultLongUrl"`
	RedirectRules  []Rule `json:"redirectRules"`
}

type RequestData struct {
	UserAgent      string
	AcceptLanguage string
	Query          url.Values
	IP             net.IP
	CountryCode    string
	CityName       string
	Now            time.Time
}

type Service struct {
	db *bun.DB
}

func NewService(db *bun.DB) *Service {
	return &Service{db: db}
}

type ValidationError struct {
	Elements []string
}

func (e *ValidationError) Error() string {
	return "invalid elements: " + strings.Join(e.Elements, ", ")
}

func (s *Service) Get(ctx context.Context, shortURLID int64, defaultLongURL string) (*Set, error) {
	rows := make([]struct {
		LongURL    string          `bun:"long_url"`
		Priority   int             `bun:"priority"`
		Conditions json.RawMessage `bun:"conditions"`
	}, 0)
	if err := s.db.NewRaw(
		"SELECT long_url, priority, conditions FROM redirect_rules WHERE short_url_id = ? ORDER BY priority ASC",
		shortURLID,
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	rules := make([]Rule, 0, len(rows))
	for _, row := range rows {
		var conditions []Condition
		if err := json.Unmarshal(row.Conditions, &conditions); err != nil {
			return nil, fmt.Errorf("decode redirect conditions: %w", err)
		}
		if conditions == nil {
			conditions = make([]Condition, 0)
		}
		rules = append(rules, Rule{LongURL: row.LongURL, Priority: row.Priority, Conditions: conditions})
	}
	return &Set{DefaultLongURL: defaultLongURL, RedirectRules: rules}, nil
}

func (s *Service) Replace(ctx context.Context, shortURLID int64, defaultLongURL string, rules []Rule) (*Set, error) {
	for i := range rules {
		rules[i].Priority = i + 1
		if rules[i].Conditions == nil {
			rules[i].Conditions = []Condition{}
		}
		if err := validateRule(rules[i]); err != nil {
			return nil, err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.NewRaw("DELETE FROM redirect_rules WHERE short_url_id = ?", shortURLID).Exec(ctx); err != nil {
		return nil, err
	}
	for _, rule := range rules {
		conditions, err := json.Marshal(rule.Conditions)
		if err != nil {
			return nil, err
		}
		if _, err := tx.NewRaw(
			"INSERT INTO redirect_rules (short_url_id, priority, long_url, conditions) VALUES (?, ?, ?, ?::jsonb)",
			shortURLID, rule.Priority, rule.LongURL, string(conditions),
		).Exec(ctx); err != nil {
			return nil, err
		}
	}
	if _, err := tx.NewRaw(
		"UPDATE short_urls SET has_redirect_rules = ? WHERE id = ?",
		len(rules) > 0, shortURLID,
	).Exec(ctx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &Set{DefaultLongURL: defaultLongURL, RedirectRules: rules}, nil
}

func (s *Service) Match(set *Set, request RequestData) string {
	if set == nil {
		return ""
	}
	if request.Now.IsZero() {
		request.Now = time.Now()
	}
	for _, rule := range set.RedirectRules {
		if matchesRule(rule, request) {
			return rule.LongURL
		}
	}
	return set.DefaultLongURL
}

func matchesRule(rule Rule, request RequestData) bool {
	for _, condition := range rule.Conditions {
		if !matchesCondition(condition, request) {
			return false
		}
	}
	return true
}

func matchesCondition(condition Condition, request RequestData) bool {
	value := ""
	if condition.MatchValue != nil {
		value = *condition.MatchValue
	}
	switch condition.Type {
	case "device":
		return ua.Parse(request.UserAgent).MatchesDevice(value)
	case "browser":
		return strings.EqualFold(ua.Parse(request.UserAgent).Browser, value)
	case "language":
		return matchesLanguage(request.AcceptLanguage, value)
	case "query-param":
		if condition.MatchKey == nil {
			return false
		}
		values, ok := request.Query[*condition.MatchKey]
		return ok && len(values) > 0 && values[0] == value
	case "any-value-query-param":
		if condition.MatchKey == nil {
			return false
		}
		_, ok := request.Query[*condition.MatchKey]
		return ok
	case "valueless-query-param":
		if condition.MatchKey == nil {
			return false
		}
		values, ok := request.Query[*condition.MatchKey]
		return ok && (len(values) == 0 || (len(values) == 1 && values[0] == ""))
	case "ip-address":
		return matchesIP(request.IP, value)
	case "geolocation-country-code":
		return strings.EqualFold(request.CountryCode, value)
	case "geolocation-city-name":
		return strings.EqualFold(request.CityName, value)
	case "before-date":
		date, err := time.Parse(time.RFC3339Nano, value)
		return err == nil && request.Now.Before(date)
	case "after-date":
		date, err := time.Parse(time.RFC3339Nano, value)
		return err == nil && request.Now.After(date)
	}
	return false
}

func validateRule(rule Rule) error {
	if !shorturl.ValidLongURL(rule.LongURL) {
		return &ValidationError{Elements: []string{"longUrl"}}
	}
	for _, condition := range rule.Conditions {
		if err := validateCondition(condition); err != nil {
			return err
		}
	}
	return nil
}

func validateCondition(condition Condition) error {
	switch condition.Type {
	case "device":
		if condition.MatchValue == nil || !ua.ValidDevice(*condition.MatchValue) {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "browser":
		if condition.MatchValue == nil || !ua.ValidBrowser(*condition.MatchValue) {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "language":
		if condition.MatchValue == nil || strings.TrimSpace(*condition.MatchValue) == "" {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "query-param":
		if condition.MatchKey == nil || strings.TrimSpace(*condition.MatchKey) == "" || condition.MatchValue == nil {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "any-value-query-param", "valueless-query-param":
		if condition.MatchKey == nil || strings.TrimSpace(*condition.MatchKey) == "" {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "ip-address":
		if condition.MatchValue == nil || !validIPPattern(*condition.MatchValue) {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "geolocation-country-code":
		if condition.MatchValue == nil || len(*condition.MatchValue) != 2 || !isAlpha(*condition.MatchValue) {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "geolocation-city-name":
		if condition.MatchValue == nil || strings.TrimSpace(*condition.MatchValue) == "" {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	case "before-date", "after-date":
		if condition.MatchValue == nil {
			return &ValidationError{Elements: []string{"conditions"}}
		}
		if _, err := time.Parse(time.RFC3339Nano, *condition.MatchValue); err != nil {
			return &ValidationError{Elements: []string{"conditions"}}
		}
	default:
		return &ValidationError{Elements: []string{"conditions"}}
	}
	return nil
}

func matchesLanguage(header, expected string) bool {
	header = strings.TrimSpace(header)
	if header == "" || header == "*" || expected == "" {
		return false
	}
	expectedLanguage, expectedCountry := splitLocale(expected)
	for _, part := range strings.Split(header, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		if len(fields) == 0 {
			continue
		}
		quality := 1.0
		for _, parameter := range fields[1:] {
			parameter = strings.TrimSpace(parameter)
			if strings.HasPrefix(parameter, "q=") {
				if parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(parameter, "q=")), 64); err == nil {
					quality = parsed
				}
			}
		}
		if quality < 0.9 {
			continue
		}
		language, country := splitLocale(fields[0])
		if !strings.EqualFold(language, expectedLanguage) {
			continue
		}
		if expectedCountry == "" || strings.EqualFold(country, expectedCountry) {
			return true
		}
	}
	return false
}

func splitLocale(value string) (string, string) {
	value = strings.ReplaceAll(strings.TrimSpace(value), "_", "-")
	parts := strings.SplitN(value, "-", 2)
	language := strings.ToLower(parts[0])
	country := ""
	if len(parts) == 2 {
		country = strings.ToUpper(parts[1])
	}
	return language, country
}

func validIPPattern(pattern string) bool {
	if net.ParseIP(pattern) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(pattern); err == nil {
		return true
	}
	parts := strings.Split(pattern, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		if part == "*" {
			continue
		}
		number, err := strconv.Atoi(part)
		if err != nil || number < 0 || number > 255 {
			return false
		}
	}
	return true
}

func matchesIP(ip net.IP, pattern string) bool {
	if ip == nil {
		return false
	}
	if _, network, err := net.ParseCIDR(pattern); err == nil {
		return network.Contains(ip)
	}
	if parsed := net.ParseIP(pattern); parsed != nil {
		return parsed.Equal(ip)
	}
	parts := strings.Split(pattern, ".")
	ip4 := ip.To4()
	if len(parts) != 4 || ip4 == nil {
		return false
	}
	for i, part := range parts {
		if part == "*" {
			continue
		}
		number, err := strconv.Atoi(part)
		if err != nil || number != int(ip4[i]) {
			return false
		}
	}
	return true
}

func isAlpha(value string) bool {
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}
