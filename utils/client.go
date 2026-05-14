package utils

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

func NewHTTPClient(proxyStr string, timeout time.Duration) *http.Client {
	proxyStr = strings.TrimSpace(proxyStr)
	// 去掉包裹的引号
	proxyStr = strings.Trim(proxyStr, `"'`)
	if proxyStr == "" {
		return &http.Client{Timeout: timeout}
	}

	proxyURL, _ := url.Parse(proxyStr)
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: timeout,
	}
}
