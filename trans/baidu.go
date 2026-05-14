package trans

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/light2000/laravel-modeler-studio/logger"
)

// baiduErrorCodePresent 判断百度 JSON 中的 error_code 是否表示失败（成功时通常无此字段或为 0）。
func baiduErrorCodePresent(code any) bool {
	if code == nil {
		return false
	}
	switch v := code.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case float64:
		return v != 0
	case int:
		return v != 0
	case json.Number:
		s := strings.TrimSpace(string(v))
		return s != "" && s != "0"
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		return s != "" && s != "0"
	}
}

type baiduTranslateResponse struct {
	From        string `json:"from"`
	To          string `json:"to"`
	TransResult []struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	} `json:"trans_result"`
	ErrorCode any    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

func BaiduTranslate(q, appID, secret string) (string, error) {
	salt := rand.IntN(90000) + 10000
	saltStr := strconv.Itoa(salt)
	signSrc := appID + q + saltStr + secret
	sum := md5.Sum([]byte(signSrc))
	sign := hex.EncodeToString(sum[:])

	v := url.Values{}
	v.Set("q", q)
	v.Set("from", "zh")
	v.Set("to", "en")
	v.Set("appid", appID)
	v.Set("salt", saltStr)
	v.Set("sign", sign)

	u := "https://fanyi-api.baidu.com/api/trans/vip/translate?" + v.Encode()
	resp, err := HTTPClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("百度接口 HTTP %d: %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("百度接口 HTTP %d: %s", resp.StatusCode, string(body))
	}
	var out baiduTranslateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		logger.Errorf("解析响应: %v, body: %s", err, string(body))
		return "", fmt.Errorf("解析响应: %w", err)
	}
	if baiduErrorCodePresent(out.ErrorCode) {
		logger.Errorf("百度错误: %v %s", out.ErrorCode, out.ErrorMsg)
		return "", fmt.Errorf("百度错误: %v %s", out.ErrorCode, out.ErrorMsg)
	}
	if len(out.TransResult) == 0 || strings.TrimSpace(out.TransResult[0].Dst) == "" {
		logger.Errorf("无翻译结果: %s", string(body))
		return "", fmt.Errorf("无翻译结果: %s", string(body))
	}
	logger.Infof("百度翻译结果: %s, 请求: %s", string(body), u)
	return strings.TrimSpace(out.TransResult[0].Dst), nil
}
