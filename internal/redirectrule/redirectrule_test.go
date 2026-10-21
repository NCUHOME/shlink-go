package redirectrule

import (
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func ptr(value string) *string { return &value }

func TestAllConditionTypes(t *testing.T) {
	now := time.Date(2021, 6, 1, 12, 0, 0, 0, time.UTC)
	request := RequestData{
		UserAgent:      "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.5414.86 Mobile Safari/537.36",
		AcceptLanguage: "en-US,en;q=0.9",
		Query:          url.Values{"hello": {"world"}, "foo": {""}, "flag": {""}},
		IP:             net.ParseIP("1.2.3.4"),
		CountryCode:    "US",
		CityName:       "New York",
		Now:            now,
	}

	for _, condition := range []Condition{
		{Type: "device", MatchValue: ptr("android")},
		{Type: "browser", MatchValue: ptr("chrome")},
		{Type: "language", MatchValue: ptr("en")},
		{Type: "query-param", MatchKey: ptr("hello"), MatchValue: ptr("world")},
		{Type: "any-value-query-param", MatchKey: ptr("foo")},
		{Type: "valueless-query-param", MatchKey: ptr("flag")},
		{Type: "ip-address", MatchValue: ptr("1.2.3.4")},
		{Type: "ip-address", MatchValue: ptr("1.2.3.0/24")},
		{Type: "ip-address", MatchValue: ptr("1.2.*.4")},
		{Type: "geolocation-country-code", MatchValue: ptr("us")},
		{Type: "geolocation-city-name", MatchValue: ptr("new york")},
		{Type: "before-date", MatchValue: ptr("2021-06-02T00:00:00+00:00")},
		{Type: "after-date", MatchValue: ptr("2021-05-31T00:00:00+00:00")},
	} {
		require.True(t, matchesCondition(condition, request), condition.Type)
	}
}

func TestConditionMismatches(t *testing.T) {
	request := RequestData{
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/146.0.0.0 Safari/537.36",
		Query:     url.Values{"hello": {"world"}},
		IP:        net.ParseIP("1.2.3.4"),
		Now:       time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	for _, condition := range []Condition{
		{Type: "device", MatchValue: ptr("ios")},
		{Type: "browser", MatchValue: ptr("firefox")},
		{Type: "language", MatchValue: ptr("en")},
		{Type: "query-param", MatchKey: ptr("hello"), MatchValue: ptr("other")},
		{Type: "any-value-query-param", MatchKey: ptr("missing")},
		{Type: "valueless-query-param", MatchKey: ptr("hello")},
		{Type: "ip-address", MatchValue: ptr("1.2.3.5")},
		{Type: "geolocation-country-code", MatchValue: ptr("ES")},
		{Type: "before-date", MatchValue: ptr("2020-01-01T00:00:00+00:00")},
		{Type: "after-date", MatchValue: ptr("2022-01-01T00:00:00+00:00")},
	} {
		require.False(t, matchesCondition(condition, request), condition.Type)
	}
}

func TestMatchUsesPriorityOrderAndFallsBackToDefault(t *testing.T) {
	set := &Set{
		DefaultLongURL: "https://example.com/default",
		RedirectRules: []Rule{
			{LongURL: "https://example.com/first", Priority: 1, Conditions: []Condition{{Type: "device", MatchValue: ptr("android")}}},
			{LongURL: "https://example.com/second", Priority: 2, Conditions: []Condition{{Type: "language", MatchValue: ptr("en")}}},
		},
	}
	request := RequestData{
		UserAgent:      "Mozilla/5.0 (Linux; Android 13) Chrome/109.0.5414.86 Mobile Safari/537.36",
		AcceptLanguage: "en",
	}
	require.Equal(t, "https://example.com/first", (&Service{}).Match(set, request))
	require.Equal(t, "https://example.com/default", (&Service{}).Match(set, RequestData{}))
	require.Empty(t, (&Service{}).Match(nil, request))
}

func TestValidateCondition(t *testing.T) {
	valid := []Condition{
		{Type: "device", MatchValue: ptr("mobile")},
		{Type: "browser", MatchValue: ptr("safari")},
		{Type: "language", MatchValue: ptr("en")},
		{Type: "query-param", MatchKey: ptr("foo"), MatchValue: ptr("bar")},
		{Type: "any-value-query-param", MatchKey: ptr("foo")},
		{Type: "valueless-query-param", MatchKey: ptr("foo")},
		{Type: "ip-address", MatchValue: ptr("10.0.0.0/8")},
		{Type: "geolocation-country-code", MatchValue: ptr("US")},
		{Type: "geolocation-city-name", MatchValue: ptr("Madrid")},
		{Type: "before-date", MatchValue: ptr("2021-01-01T00:00:00+00:00")},
		{Type: "after-date", MatchValue: ptr("2021-01-01T00:00:00+00:00")},
	}
	for _, condition := range valid {
		require.NoError(t, validateCondition(condition), condition.Type)
	}

	invalid := []Condition{
		{Type: "invalid", MatchValue: ptr("foo")},
		{Type: "device", MatchValue: ptr("toaster")},
		{Type: "browser", MatchValue: ptr("netscape")},
		{Type: "query-param", MatchKey: ptr("foo")},
		{Type: "ip-address", MatchValue: ptr("not an ip")},
		{Type: "geolocation-country-code", MatchValue: ptr("USA")},
		{Type: "before-date", MatchValue: ptr("today")},
	}
	for _, condition := range invalid {
		require.Error(t, validateCondition(condition), condition.Type)
	}
}

func TestIPPatterns(t *testing.T) {
	require.True(t, matchesIP(net.ParseIP("1.2.3.4"), "1.2.3.4"))
	require.True(t, matchesIP(net.ParseIP("1.2.3.4"), "1.2.3.0/24"))
	require.True(t, matchesIP(net.ParseIP("1.2.3.4"), "1.2.*.4"))
	require.False(t, matchesIP(net.ParseIP("1.2.3.5"), "1.2.3.4"))
	require.False(t, matchesIP(nil, "1.2.3.4"))
}
