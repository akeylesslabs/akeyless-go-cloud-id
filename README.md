# akeyless-go-cloud-id
Retrieves cloud identity. Currently only AWS cloud supported. 

## Installation

Install the following dependencies:

```shell
go get github.com/akeylesslabs/akeyless-go-sdk
go get github.com/akeylesslabs/akeyless-go-cloud-id
```

## Getting Started (example with lambda function)

Please follow the [installation procedure](#installation) and then run the following:

```golang
package main

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/aws/aws-lambda-go/lambda"
  
	akl_cloud_id "github.com/akeylesslabs/akeyless-go-cloud-id/go/src/aws"
	akl_sdk "github.com/akeylesslabs/akeyless-go-sdk"
)

type MyEvent struct {
	Name string `json:"name"`
}

func main() {
	cloud_id, err := akl_cloud_id.GetCloudId()
	if err != nil {
		fmt.Println("GetCloudId error:", err.Error())
		panic(err.Error())
	}

	cfg := akl_sdk.NewConfiguration()
	cfg.BasePath = "http://<api-gateway-host>:<port>"
	client := akl_sdk.NewAPIClient(cfg)
	api := client.DefaultApi
	aklsCtx := context.Background()

	accessId := "<your-access-id>"

	fmt.Println("Before auth")

	// Authenticate to the service and returns an access token
	authReplyObj, _, err := api.Auth(aklsCtx, accessId, &akl_sdk.DefaultApiAuthOpts{
		AccessType: optional.NewString("aws_iam"),
		CloudId:    optional.NewString(cloud_id),
	})
  
	if authReplyObj.Status != "success" {
		fmt.Println("Auth error:", authReplyObj.Status)
		panic("Auth failed")
	}

	token := authReplyObj.Token

	secretName := "<secret-name>"
  
	getValReplyObj, _, err := api.GetSecretValue(aklsCtx, secretName, token)
	if getValReplyObj.Status != "success" {
		fmt.Println("GetSecretValue error:", getValReplyObj.Status)
		panic("GetSecretValue failed")
	}
	fmt.Println(getValReplyObj.Response)


	return token, nil
}

func main() {
	lambda.Start(HandleRequest)
}
```
