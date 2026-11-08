package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

var testSigningTime = time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC)

type decodedCloudID struct {
	Method  string
	URL     string
	Body    string
	Headers http.Header
}

func TestGetCloudIDDefaultRegion(t *testing.T) {
	got := mustGetDecodedCloudID(t, testAWSConfig(""))

	if got.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", got.Method, http.MethodPost)
	}
	if got.URL != "https://sts.us-east-1.amazonaws.com/" {
		t.Errorf("url = %q, want us-east-1 regional STS endpoint", got.URL)
	}
	if got.Body != "Action=GetCallerIdentity&Version=2011-06-15" {
		t.Errorf("body = %q", got.Body)
	}
	assertSignedForRegion(t, got.Headers, defaultSTSRegion)
}

func TestGetCloudIDChinaRegion(t *testing.T) {
	got := mustGetDecodedCloudID(t, testAWSConfig("cn-north-1"))

	if got.URL != "https://sts.cn-north-1.amazonaws.com.cn/" {
		t.Errorf("url = %q, want China STS endpoint", got.URL)
	}
	assertSignedForRegion(t, got.Headers, "cn-north-1")
}

func TestGetCloudIDPayloadCompatibility(t *testing.T) {
	got := mustGetDecodedCloudID(t, testAWSConfig("us-west-2"))

	if got.Method == "" || got.URL == "" || got.Body == "" {
		t.Fatalf("decoded cloud id has empty request fields: %+v", got)
	}
	if got.Headers.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", got.Headers.Get("Content-Type"))
	}
	if got.Headers.Get("X-Amz-Date") != "20260511T100000Z" {
		t.Errorf("X-Amz-Date = %q", got.Headers.Get("X-Amz-Date"))
	}
	if got.Headers.Get("X-Amz-Security-Token") != "SESSION" {
		t.Errorf("X-Amz-Security-Token = %q", got.Headers.Get("X-Amz-Security-Token"))
	}
}

func mustGetDecodedCloudID(t *testing.T, cfg awssdk.Config) decodedCloudID {
	t.Helper()

	cloudID, err := getCloudID(context.Background(), cfg, testSigningTime)
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

func testAWSConfig(region string) awssdk.Config {
	return awssdk.Config{
		Region:      region,
		Credentials: awssdk.NewCredentialsCache(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "SESSION")),
	}
}

func assertSignedForRegion(t *testing.T, headers http.Header, region string) {
	t.Helper()

	authorization := headers.Get("Authorization")
	if authorization == "" {
		t.Fatal("missing Authorization header")
	}
	wantScope := "/" + region + "/" + stsServiceName + "/aws4_request"
	if !strings.Contains(authorization, wantScope) {
		t.Errorf("Authorization = %q, want credential scope containing %q", authorization, wantScope)
	}
}
