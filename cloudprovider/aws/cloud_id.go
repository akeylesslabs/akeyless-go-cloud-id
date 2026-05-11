package aws

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const (
	defaultSTSRegion = "us-east-1"
	stsServiceName   = "sts"
)

func GetCloudId() (string, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return "", err
	}
	return getCloudID(context.Background(), cfg, time.Now())
}

func getCloudID(ctx context.Context, cfg awssdk.Config, signingTime time.Time) (string, error) {
	region := cfg.Region
	if region == "" {
		region = defaultSTSRegion
	}

	endpointURL, err := resolveSTSEndpoint(ctx, cfg, region)
	if err != nil {
		return "", err
	}

	requestBody := []byte(url.Values{
		"Action":  {"GetCallerIdentity"},
		"Version": {"2011-06-15"},
	}.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL.String(), bytes.NewReader(requestBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return "", err
	}

	payloadHash := sha256.Sum256(requestBody)
	signer := v4.NewSigner()
	if err := signer.SignHTTP(ctx, credentials, req, hex.EncodeToString(payloadHash[:]), stsServiceName, region, signingTime); err != nil {
		return "", err
	}

	headersJson, err := json.Marshal(req.Header)
	if err != nil {
		return "", err
	}

	awsData := make(map[string]string)
	awsData["sts_request_method"] = req.Method
	awsData["sts_request_url"] = base64.StdEncoding.EncodeToString([]byte(req.URL.String()))
	awsData["sts_request_body"] = base64.StdEncoding.EncodeToString(requestBody)
	awsData["sts_request_headers"] = base64.StdEncoding.EncodeToString(headersJson)
	awsDataDump, err := json.Marshal(awsData)

	if err != nil {
		return "", err
	}

	cloudId := base64.StdEncoding.EncodeToString(awsDataDump)
	return cloudId, nil
}

func resolveSTSEndpoint(ctx context.Context, cfg awssdk.Config, region string) (url.URL, error) {
	useFIPS := false
	useDualStack := false
	useGlobalEndpoint := false

	endpoint, err := sts.NewDefaultEndpointResolverV2().ResolveEndpoint(ctx, sts.EndpointParameters{
		Region:            &region,
		UseFIPS:           &useFIPS,
		UseDualStack:      &useDualStack,
		Endpoint:          cfg.BaseEndpoint,
		UseGlobalEndpoint: &useGlobalEndpoint,
	})
	if err != nil {
		return url.URL{}, err
	}

	endpointURL := endpoint.URI
	if endpointURL.Path == "" {
		endpointURL.Path = "/"
	}
	return endpointURL, nil
}
