package handlers

import "strings"

func customerDeviceName(agent string) string {
	browser, device := "Browser", "device"
	switch {
	case strings.Contains(agent, "Edg/"):
		browser = "Edge"
	case strings.Contains(agent, "Firefox/") || strings.Contains(agent, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(agent, "Chrome/") || strings.Contains(agent, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(agent, "Safari/"):
		browser = "Safari"
	}
	switch {
	case strings.Contains(agent, "Android"):
		device = "Android"
	case strings.Contains(agent, "iPhone"):
		device = "iPhone"
	case strings.Contains(agent, "iPad"):
		device = "iPad"
	case strings.Contains(agent, "Windows"):
		device = "Windows"
	case strings.Contains(agent, "Macintosh"):
		device = "Mac"
	case strings.Contains(agent, "Linux"):
		device = "Linux"
	}
	return browser + " on " + device
}
