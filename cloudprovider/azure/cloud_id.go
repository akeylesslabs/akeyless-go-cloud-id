package azure

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// AzureADDefResource is the public Azure Resource Manager audience (legacy default).
const AzureADDefResource = "https://management.azure.com/"

// AzureADManagementScope is the public Azure Resource Manager OAuth scope (legacy default).
const AzureADManagementScope = "https://management.azure.com/.default"

const AzureADDefApiVersion = "2018-02-01"

func GetCloudId(objectId string) (string, error) {
	if objectId == "" {
		token, err := getCloudId(context.TODO())
		if err != nil {
			return "", err
		}
		return base64.StdEncoding.EncodeToString([]byte(token)), nil
	}

	cfg := resolvedAzureCloud()
	var errMsg string
	for retry := 1; retry < 6; retry++ {
		req, err := http.NewRequest("GET", "http://"+imdsHost+"/metadata/identity/oauth2/token", nil)
		if err != nil {
			return "", err
		}

		q := req.URL.Query()
		q.Add("api-version", AzureADDefApiVersion)
		q.Add("resource", cfg.resourceURL)

		if objectId != "" {
			q.Add("object_id", objectId)
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Metadata", "true")
		req.Header.Set("User-Agent", "AKEYLESS")

		resp, err := imdsHTTPClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to fetch azure-ad identity metadata. Error: %v", err.Error())
		}

		if resp == nil {
			return "", fmt.Errorf("failed to fetch azure-ad identity metadata. Error: empty response")
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("failed to read azure-ad identity metadata response. Error: %v", err.Error())
		}

		if resp.StatusCode != http.StatusOK {
			errMsg = fmt.Sprintf("failed to read azure-ad identity metadata response. "+
				"Error: invalid status code - %v body: %v", resp.StatusCode, string(body))

			//retry policy: https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token#error-handling
			if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
				time.Sleep(time.Duration(retry) * time.Second)
				continue
			} else {
				return "", errors.New(errMsg)
			}
		}

		var identity struct {
			AccessToken string `json:"access_token"`
		}
		err = json.Unmarshal(body, &identity)
		if err != nil {
			return "", fmt.Errorf("failed to unmarshal azure-ad identity metadata response. Error: %v %v", err, string(body))
		}
		cloudId := base64.StdEncoding.EncodeToString([]byte(identity.AccessToken))
		return cloudId, nil
	}

	return "", errors.New(errMsg)
}

func getCloudId(ctx context.Context) (string, error) {
	cfg := resolvedAzureCloud()
	cred, err := azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{
		ClientOptions: policy.ClientOptions{Cloud: cfg.cloud},
	})
	if err != nil {
		return "", fmt.Errorf("failed to get default Azure credential, Error: %v", err)
	}

	accessToken, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{cfg.scope}})
	if err != nil {
		return "", fmt.Errorf("failed to get Azure token, Error: %v", err)
	}
	return accessToken.Token, nil
}
