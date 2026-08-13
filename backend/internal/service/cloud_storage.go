package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func qiniuBase64(data []byte) string { return base64.RawURLEncoding.EncodeToString(data) }
func qiniuSign(secret, value string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	_, _ = mac.Write([]byte(value))
	return qiniuBase64(mac.Sum(nil))
}
func qiniuToken(setting ossSettingValue, value string) string {
	return setting.AccessKeyID + ":" + qiniuSign(setting.AccessKeySecret, value)
}

func putQiniuObject(setting ossSettingValue, objectKey, mimeType string, _ int64, body io.Reader) (string, error) {
	deadline := time.Now().Add(10 * time.Minute).Unix()
	policy, _ := json.Marshal(map[string]any{"scope": setting.Bucket + ":" + objectKey, "deadline": deadline, "returnBody": "{\"key\":$(key),\"hash\":$(etag)}"})
	encodedPolicy := qiniuBase64(policy)
	token := qiniuToken(setting, encodedPolicy)
	reader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	contentType := writer.FormDataContentType()
	go func() {
		var uploadErr error
		if uploadErr = writer.WriteField("token", token); uploadErr == nil {
			uploadErr = writer.WriteField("key", objectKey)
		}
		var part io.Writer
		if uploadErr == nil {
			part, uploadErr = writer.CreateFormFile("file", objectKey)
		}
		if uploadErr == nil {
			_, uploadErr = io.Copy(part, body)
		}
		if closeErr := writer.Close(); uploadErr == nil {
			uploadErr = closeErr
		}
		_ = pipeWriter.CloseWithError(uploadErr)
	}()
	endpoint := strings.TrimRight(setting.Endpoint, "/")
	if endpoint == "" {
		endpoint = "https://up.qiniup.com"
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, reader)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := OutboundHTTPClient(2 * time.Minute).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("七牛云上传失败：%s %s", resp.Status, strings.TrimSpace(string(detail)))
	}
	var result struct {
		Hash string `json:"hash"`
	}
	_ = json.Unmarshal(detail, &result)
	return result.Hash, nil
}

func qiniuObjectURL(setting ossSettingValue, objectKey string) (*url.URL, error) {
	raw := strings.TrimRight(setting.PublicBaseURL, "/")
	if raw == "" {
		return nil, errors.New("七牛云下载域名为空")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, errors.New("七牛云下载域名无效")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(objectKey, "/")
	return u, nil
}
func signedQiniuObjectURL(setting ossSettingValue, objectKey string, expiresAt time.Time) (string, error) {
	u, err := qiniuObjectURL(setting, objectKey)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("e", strconv.FormatInt(expiresAt.Unix(), 10))
	u.RawQuery = q.Encode()
	token := qiniuToken(setting, u.String())
	q = u.Query()
	q.Set("token", token)
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func newQiniuDownloadRequest(method string, setting ossSettingValue, objectKey string) (*http.Request, error) {
	signed, err := signedQiniuObjectURL(setting, objectKey, time.Now().Add(10*time.Minute))
	if err != nil {
		return nil, err
	}
	return http.NewRequest(method, signed, nil)
}

func tencentBucketBaseURL(setting ossSettingValue) (*url.URL, error) {
	endpoint := strings.TrimRight(setting.Endpoint, "/")
	if endpoint == "" {
		if setting.Region == "" {
			return nil, errors.New("腾讯云 COS Region 或 Endpoint 不能为空")
		}
		endpoint = "https://cos." + setting.Region + ".myqcloud.com"
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, errors.New("腾讯云 COS Endpoint 无效")
	}
	if !strings.HasPrefix(u.Host, setting.Bucket+".") {
		u.Host = setting.Bucket + "." + u.Host
	}
	return u, nil
}
func newTencentObjectRequest(method string, setting ossSettingValue, objectKey, contentType string, body io.Reader) (*http.Request, error) {
	u, err := tencentBucketBaseURL(setting)
	if err != nil {
		return nil, err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(objectKey, "/")
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", tencentAuthorization(setting, method, u.EscapedPath(), u.Host, time.Now(), 15*time.Minute, false))
	return req, nil
}
func signedTencentObjectURL(setting ossSettingValue, objectKey string, expiresAt time.Time) (string, error) {
	u, err := tencentBucketBaseURL(setting)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(objectKey, "/")
	now := time.Now()
	duration := expiresAt.Sub(now)
	if duration < time.Minute {
		duration = time.Minute
	}
	u.RawQuery = tencentAuthorization(setting, http.MethodGet, u.EscapedPath(), u.Host, now, duration, true)
	return u.String(), nil
}
func tencentAuthorization(setting ossSettingValue, method, escapedPath, host string, start time.Time, duration time.Duration, forQuery bool) string {
	startUnix := start.Unix()
	endUnix := start.Add(duration).Unix()
	keyTime := fmt.Sprintf("%d;%d", startUnix, endUnix)
	signKey := hmacSHA1Hex([]byte(setting.AccessKeySecret), keyTime)
	canonicalHeaders := "host=" + url.QueryEscape(strings.ToLower(host))
	httpString := strings.ToLower(method) + "\n" + escapedPath + "\n\n" + canonicalHeaders + "\n"
	hash := sha1.Sum([]byte(httpString))
	stringToSign := "sha1\n" + keyTime + "\n" + hex.EncodeToString(hash[:]) + "\n"
	signature := hmacSHA1Hex([]byte(signKey), stringToSign)
	values := url.Values{"q-sign-algorithm": {"sha1"}, "q-ak": {setting.AccessKeyID}, "q-sign-time": {keyTime}, "q-key-time": {keyTime}, "q-header-list": {"host"}, "q-url-param-list": {""}, "q-signature": {signature}}
	if forQuery {
		return values.Encode()
	}
	// Authorization 请求头中的 KeyTime 使用原始分号；仅预签名 URL 的查询参数需要百分号编码。
	return strings.Join([]string{"q-sign-algorithm=sha1", "q-ak=" + setting.AccessKeyID, "q-sign-time=" + keyTime, "q-key-time=" + keyTime, "q-header-list=host", "q-url-param-list=", "q-signature=" + signature}, "&")
}
func hmacSHA1Hex(key []byte, value string) string {
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}
