package main

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/aws/aws-lambda-go/lambda"

	akl_cloud_id "github.com/akeylesslabs/akeyless-go-cloud-id"
	akl "github.com/akeylesslabs/akeyless-go-sdk"
)

type MyEvent struct {
	Name string `json:"name"`
}

func HandleRequest() (string, error) {

	cloud_id, err := akl_cloud_id.GetCloudId()
	if err != nil {
		fmt.Println("GetCloudId error:", err.Error())
		panic(err.Error())
	}

	cfg := akl.NewConfiguration()
	cfg.BasePath = "http://<api-gateway-host>:<port>"
	client := akl.NewAPIClient(cfg)
	api := client.DefaultApi
	aklsCtx := context.Background()

	accessId := "<auth-method-access-id>"
	secretName := "<secret-name>"

	// Authenticate to the service and returns an access token
	authReplyObj, _, err := api.Auth(aklsCtx, accessId, &akl.DefaultApiAuthOpts{
		AccessType: optional.NewString("aws_iam"),
		CloudId:    optional.NewString(cloud_id),
	})
	if err != nil {
		fmt.Println("Auth error:", err.Error())
		panic("Auth failed")
	}

	token := authReplyObj.Token

	getValReplyObj, _, err := api.GetSecretValue(aklsCtx, secretName, token)
	if err != nil {
		fmt.Println("GetSecretValue error:", err.Error())
		panic("GetSecretValue failed")
	}
	fmt.Println("Secret value:", getValReplyObj.Response)

	return "", nil
}

func main() {
	lambda.Start(HandleRequest)
}
