package trans

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/light2000/laravel-modeler-studio/logger"
)

const (
	tencentService   = "tmt"
	tencentCTJSON    = "application/json; charset=utf-8"
	tencentAlgorithm = "TC3-HMAC-SHA256"
)

type tencentTranslateResponse struct {
	Response struct {
		TargetText string `json:"TargetText"`
		RequestID  string `json:"RequestId"`
		Error      *struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
	} `json:"Response"`
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write(data)
	return h.Sum(nil)
}

// TencentTranslate 使用腾讯云 API 3.0 签名（TC3-HMAC-SHA256）调用机器翻译 TextTranslate，将中文 q 译为英文。
// appID、secret 对应 SecretId、SecretKey。地域固定为 ap-guangzhou（与就近域名 tmt.tencentcloudapi.com 常见搭配）。
func TencentTranslate(q, appID, secret string, tencentTmtHost string, tencentVersion string, tencentAction string, tencentRegion string) (string, error) {
	tencentTmtURL := "https://" + tencentTmtHost + "/"
	q = strings.TrimSpace(q)
	if q == "" {
		return "", fmt.Errorf("待译文本为空")
	}
	if strings.TrimSpace(appID) == "" || strings.TrimSpace(secret) == "" {
		return "", fmt.Errorf("缺少 SecretId 或 SecretKey")
	}

	payload := struct {
		SourceText string `json:"SourceText"`
		Source     string `json:"Source"`
		Target     string `json:"Target"`
		ProjectID  int    `json:"ProjectId"`
	}{
		SourceText: q,
		Source:     "zh",
		Target:     "en",
		ProjectID:  0,
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("构造请求体: %w", err)
	}
	payloadStr := string(bodyBytes)

	timestamp := time.Now().Unix()
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		tencentCTJSON, tencentTmtHost, strings.ToLower(tencentAction))
	signedHeaders := "content-type;host;x-tc-action"
	hashedRequestPayload := sha256Hex(payloadStr)

	canonicalRequest := strings.Join([]string{
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload,
	}, "\n")

	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, tencentService)
	hashedCanonicalRequest := sha256Hex(canonicalRequest)
	stringToSign := strings.Join([]string{
		tencentAlgorithm,
		fmt.Sprintf("%d", timestamp),
		credentialScope,
		hashedCanonicalRequest,
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+secret), []byte(date))
	secretService := hmacSHA256(secretDate, []byte(tencentService))
	secretSigning := hmacSHA256(secretService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(secretSigning, []byte(stringToSign)))

	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		tencentAlgorithm, appID, credentialScope, signedHeaders, signature)

	req, err := http.NewRequest(httpRequestMethod, tencentTmtURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", tencentCTJSON)
	req.Header.Set("Host", tencentTmtHost)
	req.Header.Set("X-TC-Action", tencentAction)
	req.Header.Set("X-TC-Version", tencentVersion)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", tencentRegion)
	req.Header.Set("Authorization", authorization)

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var out tencentTranslateResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		if resp.StatusCode != http.StatusOK {
			logger.Errorf("腾讯云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
			return "", fmt.Errorf("腾讯云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return "", fmt.Errorf("解析响应: %w, body: %s", err, string(respBody))
	}
	if out.Response.Error != nil {
		e := out.Response.Error
		logger.Errorf("腾讯云错误: %s %s", e.Code, e.Message)
		return "", fmt.Errorf("腾讯云错误: %s %s", e.Code, e.Message)
	}
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("腾讯云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
		return "", fmt.Errorf("腾讯云接口 HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	text := strings.TrimSpace(out.Response.TargetText)
	if text == "" {
		logger.Errorf("无翻译结果: %s", string(respBody))
		return "", fmt.Errorf("无翻译结果: %s", string(respBody))
	}
	logger.Infof("腾讯云翻译结果: %s, 请求: %s", string(respBody), tencentTmtURL)
	return text, nil
}
