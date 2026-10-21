package shorturl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSlug(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
		ok    bool
	}{
		{"my cool slug", "my-cool-slug", true},
		{"foo/b  ar-baz", "foo-b--ar-baz", true},
		{"🦣🦣🦣", "🦣🦣🦣", true},
		{"", "", false},
		{"foo:bar", "", false},
		{"foo?bar", "", false},
	} {
		got, err := NormalizeSlug(tc.input)
		if !tc.ok {
			require.Error(t, err, tc.input)
			continue
		}
		require.NoError(t, err, tc.input)
		require.Equal(t, tc.want, got)
	}
}

func TestNormalizeTags(t *testing.T) {
	require.Equal(t,
		[]string{"foo", "bar", "aäa"},
		NormalizeTags([]string{" Foo ", "bar", "FOO", "", "Aäa"}),
	)
}

func TestCropTitle(t *testing.T) {
	require.Nil(t, CropTitle(nil))
	value := strings.Repeat(" ", 10) + "bar" + strings.Repeat(" ", 10)
	require.Equal(t, "bar", *CropTitle(&value))
	long := strings.Repeat("c", 513)
	require.Equal(t, strings.Repeat("c", 512), *CropTitle(&long))
}

func TestValidLongURL(t *testing.T) {
	for _, value := range []string{
		"https://example.com",
		"mailto:foo@example.com",
		"file:///foo/bar",
		"shlink://some/path",
	} {
		require.True(t, ValidLongURL(value), value)
	}
	for _, value := range []string{"", "  ", "foo.com", "://bad"} {
		require.False(t, ValidLongURL(value), value)
	}
}

func TestValidAuthority(t *testing.T) {
	for _, value := range []string{
		"s.test",
		"example.com",
		"some-domain.com",
		"example.com:8080",
		"this_domain_is_detached.com",
	} {
		require.True(t, ValidAuthority(value), value)
	}
	for _, value := range []string{
		"",
		"localhost",
		"localhost:80000",
		"127.0.0.1",
		"540.38.386.0",
		"???/&%$&",
		"example.com:888888",
		"has space.com",
	} {
		require.False(t, ValidAuthority(value), value)
	}
}

func TestGenerateCode(t *testing.T) {
	code, err := GenerateCode(12, "0123456789abcdef")
	require.NoError(t, err)
	require.Len(t, code, 12)
	for _, r := range code {
		require.Contains(t, "0123456789abcdef", string(r))
	}
	_, err = GenerateCode(0, "abc")
	require.Error(t, err)
}
