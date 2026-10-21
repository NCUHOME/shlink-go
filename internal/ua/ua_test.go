package ua

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	androidUA = "Mozilla/5.0 (Linux; Android 13) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/109.0.5414.86 Mobile Safari/537.36"
	iosUA     = "Mozilla/5.0 (iPhone; CPU iPhone OS 16_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/109.0 Mobile/15E148 Safari/605.1.15"
	windowsUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36 Edg/138.0.3351.95"
	linuxUA   = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.6998.198 Safari/537.36"
)

func TestBotDetectionUsesUpstreamFixtures(t *testing.T) {
	require.False(t, IsBot("shlink-tests-agent"))
	require.True(t, IsBot("cf-facebook"))
	require.True(t, IsBot("Googlebot/2.1 (+http://www.google.com/bot.html)"))
	require.False(t, IsBot(""))
}

func TestDeviceParsingMatchesUpstreamFixtures(t *testing.T) {
	require.Equal(t, "android", Parse(androidUA).Device)
	require.Equal(t, "ios", Parse(iosUA).Device)
	require.True(t, Parse(androidUA).MatchesDevice("mobile"))
	require.True(t, Parse(iosUA).MatchesDevice("mobile"))
	require.False(t, Parse(iosUA).MatchesDevice("android"))

	require.True(t, Parse(windowsUA).MatchesDevice("windows"))
	require.True(t, Parse(windowsUA).MatchesDevice("desktop"))
	require.True(t, Parse(linuxUA).MatchesDevice("linux"))
	require.True(t, Parse(linuxUA).MatchesDevice("desktop"))
}

func TestBrowserParsing(t *testing.T) {
	require.Equal(t, "edge", Parse(windowsUA).Browser)
	require.Equal(t, "firefox", Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:148.0) Gecko/20100101 Firefox/148.0").Browser)
	require.Equal(t, "chrome", Parse("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36").Browser)
	require.True(t, ValidBrowser("chrome"))
	require.True(t, ValidBrowser("android_browser"))
	require.False(t, ValidBrowser("netscape"))
}
