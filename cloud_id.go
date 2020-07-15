package akeyless_go_cloud_id

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"

	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func GetCloudId() (string, error) {
	awsCfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return "", err
	}

	// Endpoint https://sts.amazonaws.com is available only in single region: us-east-1. 
	// So, caller identity request can be only us-east-1. Default call brings region where caller is
	awsCfg.Region = "us-east-1"

	svc := sts.New(awsCfg)
	input := &sts.GetCallerIdentityInput{}
	req := svc.GetCallerIdentityRequest(input)

	err = req.Sign()
	if err != nil {
		return "", err
	}

	headersJson, err := json.Marshal(req.HTTPRequest.Header)
	if err != nil {
		return "", err
	}
	requestBody, err := ioutil.ReadAll(req.HTTPRequest.Body)
	if err != nil {
		return "", err
	}

	awsData := make(map[string]string)
	awsData["sts_request_method"] = req.HTTPRequest.Method
	awsData["sts_request_url"] = base64.StdEncoding.EncodeToString([]byte(req.HTTPRequest.URL.String()))
	awsData["sts_request_body"] = base64.StdEncoding.EncodeToString(requestBody)
	awsData["sts_request_headers"] = base64.StdEncoding.EncodeToString(headersJson)
	awsDataDump, err := json.Marshal(awsData)

	if err != nil {
		return "", err
	}

	cloud_id := base64.StdEncoding.EncodeToString(awsDataDump)
	return cloud_id, nil
}
