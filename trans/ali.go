package trans

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/light2000/laravel-modeler-studio/logger"
)

type aliyunTranslateResponse struct {
	Code    any    `json:"Code"`
	Message string `json:"Message"`
	Data    *struct {
		Translated string `json:"Translated"`
	} `json:"Data"`
	ErrorCode string `json:"errorCode"`
	ErrorMsg  string `json:"errorMsg"`
}

func aliyunCodeOK(code any) bool {
	switch v := code.(type) {
	case float64:
		return v == 200
	case string:
		return strings.TrimSpace(v) == "200"
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		return s == "200"
	}
}

func md5Base64(s string) string {
	sum := md5.Sum([]byte(s))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func hmacSHA1Base64(data, key string) string {
	mac := hmac.New(sha1.New, []byte(key))
	_, _ = mac.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func aliyunGMTDate(t time.Time) string {
	return t.UTC().Format("Mon, 02 Jan 2006 15:04:05") + " GMT"
}

func randomUUID() (string, error) {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]),
		uint16(b[4])<<8|uint16(b[5]),
		uint16(b[6])<<8|uint16(b[7]),
		uint16(b[8])<<8|uint16(b[9]),
		b[10:]), nil
}

// AliyunTranslate 调用阿里云机器翻译电商版 HTTP 接口（ACS HMAC-SHA1 签名），将中文 q 译为英文。
// appID、secret 对应 AccessKeyId、AccessKeySecret。
func AliyunTranslate(q, appID, secret string, aliyunMTURL string) (string, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return "", fmt.Errorf("待译文本为空")
	}

	payload := struct {
		FormatType     string `json:"FormatType"`
		SourceLanguage string `json:"SourceLanguage"`
		TargetLanguage string `json:"TargetLanguage"`
		SourceText     string `json:"SourceText"`
		Scene          string `json:"Scene"`
	}{
		FormatType:     "text",
		SourceLanguage: "zh",
		TargetLanguage: "en",
		SourceText:     q,
		Scene:          "title",
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("构造请求体: %w", err)
	}
	body := string(bodyBytes)

	u, err := url.Parse(aliyunMTURL)
	if err != nil {
		return "", err
	}
	path := u.Path
	if path == "" {
		path = "/"
	}

	method := "POST"
	accept := "application/json"
	contentType := "application/json;charset=utf-8"
	dateStr := aliyunGMTDate(time.Now())
	bodyMd5 := md5Base64(body)

	uuid, err := randomUUID()
	if err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}

	stringToSign := method + "\n" + accept + "\n" + bodyMd5 + "\n" + contentType + "\n" + dateStr + "\n" +
		"x-acs-signature-method:HMAC-SHA1\n" +
		"x-acs-signature-nonce:" + uuid + "\n" +
		"x-acs-version:2019-01-02\n" +
		path

	signature := hmacSHA1Base64(stringToSign, secret)
	authHeader := "acs " + appID + ":" + signature

	req, err := http.NewRequest(method, aliyunMTURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Content-MD5", bodyMd5)
	req.Header.Set("Date", dateStr)
	req.Header.Set("Host", u.Host)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("x-acs-signature-nonce", uuid)
	req.Header.Set("x-acs-signature-method", "HMAC-SHA1")
	req.Header.Set("x-acs-version", "2019-01-02")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var out aliyunTranslateResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			logger.Errorf("阿里云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
			return "", fmt.Errorf("阿里云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return "", fmt.Errorf("解析响应: %w, body: %s", err, string(respBody))
	}

	if strings.TrimSpace(out.ErrorCode) != "" || strings.TrimSpace(out.ErrorMsg) != "" {
		logger.Errorf("阿里云错误: %s %s", out.ErrorCode, out.ErrorMsg)
		return "", fmt.Errorf("阿里云错误: %s %s", out.ErrorCode, out.ErrorMsg)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("阿里云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("阿里云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	if !aliyunCodeOK(out.Code) {
		logger.Errorf("阿里云错误: %v %s", out.Code, out.Message)
		return "", fmt.Errorf("阿里云 Code=%v %s", out.Code, out.Message)
	}
	if out.Data == nil || strings.TrimSpace(out.Data.Translated) == "" {
		logger.Errorf("无翻译结果: %s", string(respBody))
		return "", fmt.Errorf("无翻译结果: %s", string(respBody))
	}
	logger.Infof("阿里云翻译结果: %s, 请求: %s", string(respBody), aliyunMTURL)
	return strings.TrimSpace(out.Data.Translated), nil
}
