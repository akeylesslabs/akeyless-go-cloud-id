package alibaba

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	alicredentials "github.com/aliyun/credentials-go/credentials"
)

const (
	defaultRegion   = "cn-hangzhou"
	stsDomain       = "sts.aliyuncs.com"
	stsAPIVersion   = "2015-04-01"
	stsAPIAction    = "GetCallerIdentity"
	stsAPIFormat    = "JSON"
	signatureMethod = "HMAC-SHA1"
)

type alibabaCredentials struct {
	AccessKeyID     string
	AccessKeySecret string
	SecurityToken   string
}

type cloudIDOptions struct {
	region    string
	timestamp string
	nonce     string
	creds     alibabaCredentials
}

// GetCloudId returns an Alibaba Cloud identity proof by signing an STS
// GetCallerIdentity request without sending it.
func GetCloudId() (string, error) {
	credProvider, err := alicredentials.NewCredential(nil)
	if err != nil {
		return "", err
	}

	model, err := credProvider.GetCredential()
	if err != nil {
		return "", err
	}

	return getCloudID(context.Background(), cloudIDOptions{
		region:    resolveRegion(),
		timestamp: formatTimestamp(time.Now().UTC()),
		nonce:     randomNonce(),
		creds: alibabaCredentials{
			AccessKeyID:     derefString(model.AccessKeyId),
			AccessKeySecret: derefString(model.AccessKeySecret),
			SecurityToken:   derefString(model.SecurityToken),
		},
	})
}

func getCloudID(ctx context.Context, opts cloudIDOptions) (string, error) {
	region := opts.region
	if region == "" {
		region = defaultRegion
	}
	if opts.creds.AccessKeyID == "" || opts.creds.AccessKeySecret == "" {
		return "", fmt.Errorf("alibaba credentials are missing access key id or secret")
	}

	req, requestBody, err := buildSignedSTSRequest(ctx, opts, region)
	if err != nil {
		return "", err
	}

	headersJSON, err := json.Marshal(req.Header)
	if err != nil {
		return "", err
	}

	alibabaData := map[string]string{
		"sts_request_method":  req.Method,
		"sts_request_url":     base64.StdEncoding.EncodeToString([]byte(req.URL.String())),
		"sts_request_body":    base64.StdEncoding.EncodeToString(requestBody),
		"sts_request_headers": base64.StdEncoding.EncodeToString(headersJSON),
	}
	alibabaDataDump, err := json.Marshal(alibabaData)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(alibabaDataDump), nil
}

func buildSignedSTSRequest(ctx context.Context, opts cloudIDOptions, region string) (*http.Request, []byte, error) {
	queryParams := map[string]string{
		"AccessKeyId":      opts.creds.AccessKeyID,
		"Action":           stsAPIAction,
		"Format":           stsAPIFormat,
		"RegionId":         region,
		"SignatureMethod":  signatureMethod,
		"SignatureNonce":   opts.nonce,
		"SignatureType":    "",
		"SignatureVersion": "1.0",
		"Timestamp":        opts.timestamp,
		"Version":          stsAPIVersion,
	}
	if opts.creds.SecurityToken != "" {
		queryParams["SecurityToken"] = opts.creds.SecurityToken
	}

	stringToSign := buildRPCStringToSign(http.MethodPost, queryParams, nil)
	signature := shaHmac1(stringToSign, opts.creds.AccessKeySecret+"&")
	queryParams["Signature"] = signature

	requestURL := url.URL{
		Scheme:   "https",
		Host:     stsDomain,
		Path:     "/",
		RawQuery: encodeQueryParams(queryParams),
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-acs-action", stsAPIAction)
	req.Header.Set("x-acs-version", stsAPIVersion)

	return req, nil, nil
}

func buildRPCStringToSign(method string, queryParams, formParams map[string]string) string {
	signParams := make(map[string]string, len(queryParams)+len(formParams))
	for key, value := range queryParams {
		signParams[key] = value
	}
	for key, value := range formParams {
		signParams[key] = value
	}

	stringToSign := encodeQueryParams(signParams)
	stringToSign = strings.ReplaceAll(stringToSign, "+", "%20")
	stringToSign = strings.ReplaceAll(stringToSign, "*", "%2A")
	stringToSign = strings.ReplaceAll(stringToSign, "%7E", "~")
	stringToSign = url.QueryEscape(stringToSign)

	return method + "&%2F&" + stringToSign
}

func encodeQueryParams(params map[string]string) string {
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values.Encode()
}

func shaHmac1(source, secret string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(source))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func resolveRegion() string {
	for _, key := range []string{
		"ALIBABA_CLOUD_REGION_ID",
		"ALIBABA_CLOUD_REGION",
		"REGION_ID",
	} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func formatTimestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05Z")
}

func randomNonce() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
