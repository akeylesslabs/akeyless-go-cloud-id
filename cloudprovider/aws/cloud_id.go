package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
)

const (
	AWS_STS_REQUEST_METHOD  = "sts_request_method"
	AWS_STS_REQUEST_URL     = "sts_request_url"
	AWS_STS_REQUEST_BODY    = "sts_request_body"
	AWS_STS_REQUEST_HEADERS = "sts_request_headers"
)

func GetCloudId() (string, error) {
	// Endpoint https://sts.amazonaws.com is available only in single region: us-east-1.
	// So, caller identity request can be only us-east-1. Default call brings region where caller is
	region := "us-east-1"

	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return "", err
	}
	stsClient := sts.NewFromConfig(cfg)

	captureReq := &captureSingedRequest{}

	// we don't actually call GetCallerIdentity, we just want the signed request, so we add a middleware
	// after Finalize to capture it
	_, err = stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(o *sts.Options) {
		o.APIOptions = append(o.APIOptions, func(s *middleware.Stack) error {
			return s.Finalize.Add(captureReq, middleware.After)
		})
	})

	if err != nil {
		return "", err
	}

	req := captureReq.req

	headersJson, err := json.Marshal(req.Header)
	if err != nil {
		return "", err
	}
	requestBody, err := io.ReadAll(req.GetStream())
	if err != nil {
		return "", err
	}

	awsData := make(map[string]string)
	awsData[AWS_STS_REQUEST_METHOD] = req.Method
	awsData[AWS_STS_REQUEST_URL] = base64.StdEncoding.EncodeToString([]byte(req.URL.String()))
	awsData[AWS_STS_REQUEST_HEADERS] = base64.StdEncoding.EncodeToString(headersJson)
	awsDataDump, err := json.Marshal(awsData)
	awsData[AWS_STS_REQUEST_BODY] = base64.StdEncoding.EncodeToString(requestBody)

	if err != nil {
		return "", err
	}

	cloudId := base64.StdEncoding.EncodeToString(awsDataDump)
	return cloudId, nil
}
