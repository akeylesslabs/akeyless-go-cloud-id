package alibaba

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const testTimestamp = "2026-05-11T10:00:00Z"

func TestGetCloudIDDefaultRegion(t *testing.T) {
	got := mustGetDecodedCloudID(t, testCloudIDOptions(""))

	if got.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", got.Method, http.MethodPost)
	}
	if !strings.HasPrefix(got.URL, "https://sts.aliyuncs.com/?") {
		t.Errorf("url = %q, want STS global endpoint", got.URL)
	}
	if got.Body != "" {
		t.Errorf("body = %q, want empty body", got.Body)
	}

	parsedURL, err := url.Parse(got.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	query := parsedURL.Query()
	if query.Get("RegionId") != defaultRegion {
		t.Errorf("RegionId = %q, want %q", query.Get("RegionId"), defaultRegion)
	}
	if query.Get("Action") != stsAPIAction {
		t.Errorf("Action = %q", query.Get("Action"))
	}
	if query.Get("Version") != stsAPIVersion {
		t.Errorf("Version = %q", query.Get("Version"))
	}
	if query.Get("Signature") == "" {
		t.Fatal("missing Signature query parameter")
	}
}

func TestGetCloudIDConfiguredRegion(t *testing.T) {
	got := mustGetDecodedCloudID(t, testCloudIDOptions("cn-beijing"))

	parsedURL, err := url.Parse(got.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if parsedURL.Query().Get("RegionId") != "cn-beijing" {
		t.Errorf("RegionId = %q, want cn-beijing", parsedURL.Query().Get("RegionId"))
	}
}

func TestGetCloudIDIncludesSecurityToken(t *testing.T) {
	opts := testCloudIDOptions("cn-hangzhou")
	opts.creds.SecurityToken = "SESSION"

	got := mustGetDecodedCloudID(t, opts)

	parsedURL, err := url.Parse(got.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	if parsedURL.Query().Get("SecurityToken") != "SESSION" {
		t.Errorf("SecurityToken = %q", parsedURL.Query().Get("SecurityToken"))
	}
}

func TestGetCloudIDPayloadCompatibility(t *testing.T) {
	got := mustGetDecodedCloudID(t, testCloudIDOptions("cn-hangzhou"))

	if got.Method == "" || got.URL == "" {
		t.Fatalf("decoded cloud id has empty request fields: %+v", got)
	}
	if got.Headers.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", got.Headers.Get("Content-Type"))
	}
	if got.Headers.Get("x-acs-action") != stsAPIAction {
		t.Errorf("x-acs-action = %q", got.Headers.Get("x-acs-action"))
	}
	if got.Headers.Get("x-acs-version") != stsAPIVersion {
		t.Errorf("x-acs-version = %q", got.Headers.Get("x-acs-version"))
	}
}

func TestBuildRPCStringToSignDeterministic(t *testing.T) {
	queryParams := map[string]string{
		"AccessKeyId":      "AKID",
		"Action":           stsAPIAction,
		"Format":           stsAPIFormat,
		"RegionId":         defaultRegion,
		"SignatureMethod":  signatureMethod,
		"SignatureNonce":   "fixed-nonce",
		"SignatureType":    "",
		"SignatureVersion": "1.0",
		"Timestamp":        testTimestamp,
		"Version":          stsAPIVersion,
	}

	stringToSign := buildRPCStringToSign(http.MethodPost, queryParams, nil)
	signature := shaHmac1(stringToSign, "SECRET&")

	wantStringToSign := "POST&%2F&AccessKeyId%3DAKID%26Action%3DGetCallerIdentity%26Format%3DJSON%26RegionId%3Dcn-hangzhou%26SignatureMethod%3DHMAC-SHA1%26SignatureNonce%3Dfixed-nonce%26SignatureType%3D%26SignatureVersion%3D1.0%26Timestamp%3D2026-05-11T10%253A00%253A00Z%26Version%3D2015-04-01"
	if stringToSign != wantStringToSign {
		t.Errorf("stringToSign = %q, want %q", stringToSign, wantStringToSign)
	}
	if signature != "dSCqL2sSKYDmcOcAj2Grhpar/wE=" {
		t.Errorf("signature = %q", signature)
	}

	queryParams["Signature"] = signature
	if encodeQueryParams(queryParams) == "" {
		t.Fatal("expected encoded query string")
	}
}

type decodedCloudID struct {
	Method  string
	URL     string
	Body    string
	Headers http.Header
}

func mustGetDecodedCloudID(t *testing.T, opts cloudIDOptions) decodedCloudID {
	t.Helper()

	cloudID, err := getCloudID(context.Background(), opts)
	if err != nil {
		t.Fatalf("getCloudID returned error: %v", err)
	}

	rawCloudID, err := base64.StdEncoding.DecodeString(cloudID)
	if err != nil {
		t.Fatalf("cloud id is not base64: %v", err)
	}

	var payload map[string]string
	if err := json.Unmarshal(rawCloudID, &payload); err != nil {
		t.Fatalf("cloud id is not JSON: %v", err)
	}

	requestURL := mustDecodeBase64Field(t, payload, "sts_request_url")
	requestBody := mustDecodeBase64Field(t, payload, "sts_request_body")
	rawHeaders := mustDecodeBase64Field(t, payload, "sts_request_headers")

	var headers http.Header
	if err := json.Unmarshal([]byte(rawHeaders), &headers); err != nil {
		t.Fatalf("headers are not JSON: %v", err)
	}

	return decodedCloudID{
		Method:  payload["sts_request_method"],
		URL:     requestURL,
		Body:    requestBody,
		Headers: headers,
	}
}

func mustDecodeBase64Field(t *testing.T, payload map[string]string, key string) string {
	t.Helper()

	value, ok := payload[key]
	if !ok {
		t.Fatalf("missing %q from cloud id", key)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatalf("%s is not base64: %v", key, err)
	}
	return string(decoded)
}

func testCloudIDOptions(region string) cloudIDOptions {
	return cloudIDOptions{
		region:    region,
		timestamp: testTimestamp,
		nonce:     "fixed-nonce",
		creds: alibabaCredentials{
			AccessKeyID:     "AKID",
			AccessKeySecret: "SECRET",
		},
	}
}
