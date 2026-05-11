# akeyless-go-cloud-id

Retrieves cloud identity. Currently AWS, Azure and GCP are supported.

## AWS cloud environments

The AWS cloud identity helper uses AWS SDK for Go v2 and signs an STS `GetCallerIdentity` request without sending it. Credentials are resolved through the standard AWS SDK chain, including environment variables, shared config/profile files, Lambda execution roles, ECS/Fargate task roles, and EC2 instance profiles.

Region is read from the AWS SDK configuration (`AWS_REGION`, `AWS_DEFAULT_REGION`, shared config, or other SDK-supported sources). If no region is configured, the helper falls back to `us-east-1`. For AWS China, configure a China region such as `cn-north-1` or `cn-northwest-1`; the helper signs against the matching STS endpoint under `amazonaws.com.cn`.

Import: `github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/aws`; use `aws.GetCloudId()`.

## Azure cloud environments

The Azure cloud identity helpers pick the correct Resource Manager audience (public, US Government, or China). Resolution runs once per process.

1. **Environment variables** (optional override; checked in this order):
   - `AZURE_ENVIRONMENT`
   - `AZURE_CLOUD`

   If the first variable is set but not a supported name, the second is tried. Values are matched case-insensitively. Supported names:

   | Value | Cloud |
   | ------- | -------- |
   | `AzureCloud` or `AzurePublicCloud` | Public Azure |
   | `AzureUSGovernment` or `AzureUSGovernmentCloud` | Azure US Government |
   | `AzureChinaCloud` or `AzureChinaCloud21Vianet` | Azure China (21Vianet) |

2. **Automatic detection on Azure VMs**: if neither variable yields a known cloud, the library reads instance metadata (`compute.azEnvironment`) from the Azure IMDS. Typical values are `AzurePublicCloud`, `AzureUSGovernment`, and `AzureChinaCloud`.

3. **Default**: if metadata is unavailable (for example, local development), behavior falls back to **public** Azure (`https://management.azure.com/`).

Import: `github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/azure` — use `azure.GetCloudId(...)`.

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
  
    akl_cloud_id "github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/aws"
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
