package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
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

func TestGetCloudIdWithRegionOverridesEnvironmentRegion(t *testing.T) {
	setStaticAWSEnv(t, "eu-west-1")

	cloudID, err := GetCloudIdWithRegion("us-east-1")
	if err != nil {
		t.Fatalf("GetCloudIdWithRegion returned error: %v", err)
	}
	got := decodeCloudID(t, cloudID)

	if got.URL != "https://sts.us-east-1.amazonaws.com/" {
		t.Errorf("url = %q, want us-east-1 regional STS endpoint", got.URL)
	}
	assertSignedForRegion(t, got.Headers, "us-east-1")
}

func TestGetCloudIdWithEmptyRegionUsesSDKRegion(t *testing.T) {
	setStaticAWSEnv(t, "eu-west-1")

	for name, getCloudId := range map[string]func() (string, error){
		"GetCloudId":           GetCloudId,
		"GetCloudIdWithRegion": func() (string, error) { return GetCloudIdWithRegion("") },
	} {
		cloudID, err := getCloudId()
		if err != nil {
			t.Fatalf("%s returned error: %v", name, err)
		}
		got := decodeCloudID(t, cloudID)

		if got.URL != "https://sts.eu-west-1.amazonaws.com/" {
			t.Errorf("%s: url = %q, want eu-west-1 regional STS endpoint", name, got.URL)
		}
		assertSignedForRegion(t, got.Headers, "eu-west-1")
	}
}

// setStaticAWSEnv makes config.LoadDefaultConfig resolve static credentials and the given
// region from the environment, without reading the developer's AWS files or IMDS.
func setStaticAWSEnv(t *testing.T, region string) {
	t.Helper()

	emptyFile := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(emptyFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", emptyFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", emptyFile)
	t.Setenv("AWS_PROFILE", "")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_STS", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_REGION", region)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKID")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "SECRET")
	t.Setenv("AWS_SESSION_TOKEN", "SESSION")
}

func mustGetDecodedCloudID(t *testing.T, cfg awssdk.Config) decodedCloudID {
	t.Helper()

	cloudID, err := getCloudID(context.Background(), cfg, testSigningTime)
	if err != nil {
		t.Fatalf("getCloudID returned error: %v", err)
	}
	return decodeCloudID(t, cloudID)
}

func decodeCloudID(t *testing.T, cloudID string) decodedCloudID {
	t.Helper()

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
