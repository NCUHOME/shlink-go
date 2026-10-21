package ua

import (
	"strings"

	"github.com/mileusna/useragent"
)

type Info struct {
	Browser string
	OS      string
	Device  string
	Bot     bool
}

func Parse(raw string) Info {
	parsed := useragent.Parse(raw)
	info := Info{
		Browser: browserName(parsed),
		OS:      osName(raw),
		Device:  deviceName(parsed),
		Bot:     IsBot(raw),
	}
	return info
}

func IsBot(raw string) bool {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if lower == "" {
		return false
	}
	for _, marker := range []string{
		"bot", "crawler", "spider", "slurp", "facebookexternalhit", "cf-facebook",
		"headlesschrome", "lighthouse", "pingdom", "uptimerobot", "curl/", "wget/",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return useragent.Parse(raw).Bot
}

func (i Info) MatchesDevice(device string) bool {
	device = strings.ToLower(strings.TrimSpace(device))
	switch device {
	case "android":
		return i.Device == "android"
	case "ios":
		return i.Device == "ios"
	case "mobile":
		return i.Device == "android" || i.Device == "ios"
	case "windows":
		return i.OS == "windows"
	case "macos":
		return i.OS == "macos"
	case "linux":
		return i.OS == "linux"
	case "chromeos":
		return i.OS == "chromeos"
	case "desktop":
		return i.OS == "windows" || i.OS == "macos" || i.OS == "linux" || i.OS == "chromeos"
	}
	return false
}

func ValidDevice(device string) bool {
	switch strings.ToLower(strings.TrimSpace(device)) {
	case "android", "ios", "mobile", "windows", "macos", "linux", "chromeos", "desktop":
		return true
	}
	return false
}

func ValidBrowser(browser string) bool {
	switch strings.ToLower(strings.TrimSpace(browser)) {
	case "chrome", "firefox", "edge", "safari", "opera", "android_browser":
		return true
	}
	return false
}

func browserName(parsed useragent.UserAgent) string {
	switch {
	case parsed.IsEdge():
		return "edge"
	case parsed.IsOpera() || parsed.IsOperaMini():
		return "opera"
	case parsed.IsChrome():
		return "chrome"
	case parsed.IsFirefox():
		return "firefox"
	case parsed.IsSafari():
		return "safari"
	default:
		return strings.ToLower(parsed.Name)
	}
}

func deviceName(parsed useragent.UserAgent) string {
	switch {
	case parsed.IsAndroid():
		return "android"
	case parsed.IsIOS():
		return "ios"
	default:
		return ""
	}
}

func osName(raw string) string {
	parsed := useragent.Parse(raw)
	switch {
	case parsed.IsWindows():
		return "windows"
	case parsed.IsMacOS():
		return "macos"
	case parsed.IsChromeOS():
		return "chromeos"
	case parsed.IsLinux():
		return "linux"
	case parsed.IsAndroid():
		return "android"
	case parsed.IsIOS():
		return "ios"
	default:
		return strings.ToLower(parsed.OS)
	}
}
