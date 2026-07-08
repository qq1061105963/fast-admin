// Package useragent 从 User-Agent 字符串里做最基础的浏览器/系统/设备类型解析，
// 对应 Java 侧 ServletUtil.parseBrowser/parseOs 的"够用就行"实现，不追求精确。
package useragent

import "strings"

func ParseBrowser(ua string) string {
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "edg/"):
		return "Edge"
	case strings.Contains(l, "chrome/") && !strings.Contains(l, "edg/"):
		return "Chrome"
	case strings.Contains(l, "firefox/"):
		return "Firefox"
	case strings.Contains(l, "safari/") && !strings.Contains(l, "chrome/"):
		return "Safari"
	case strings.Contains(l, "msie") || strings.Contains(l, "trident/"):
		return "IE"
	default:
		return "Unknown"
	}
}

func ParseOS(ua string) string {
	l := strings.ToLower(ua)
	switch {
	case strings.Contains(l, "windows"):
		return "Windows"
	case strings.Contains(l, "mac os") || strings.Contains(l, "macintosh"):
		return "macOS"
	case strings.Contains(l, "android"):
		return "Android"
	case strings.Contains(l, "iphone") || strings.Contains(l, "ipad"):
		return "iOS"
	case strings.Contains(l, "linux"):
		return "Linux"
	default:
		return "Unknown"
	}
}

// ParseDevice 对应 Java 侧"包含 mobile 就是 Mobile，否则 PC"的粗略判断。
func ParseDevice(ua string) string {
	if strings.Contains(strings.ToLower(ua), "mobile") {
		return "Mobile"
	}
	return "PC"
}
