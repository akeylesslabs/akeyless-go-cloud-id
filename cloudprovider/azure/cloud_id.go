package azure

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

const AzureADDefResource = "https://management.azure.com/"
const AzureADDefApiVersion = "2018-02-01"

func GetCloudId(objectId string) (string, error) {
	var errMsg string
	for retry := 1; retry < 6; retry++ {

		req, err := http.NewRequest("GET", "http://169.254.169.254/metadata/identity/oauth2/token", nil)
		if err != nil {
			return "", err
		}

		q := req.URL.Query()
		q.Add("api-version", AzureADDefApiVersion)
		q.Add("resource", AzureADDefResource)

		if objectId != "" {
			q.Add("object_id", objectId)
		}
		req.URL.RawQuery = q.Encode()
		req.Header.Set("Metadata", "true")
		req.Header.Set("User-Agent", "AKEYLESS")

		httpClient := &http.Client{}
		resp, err := httpClient.Do(req)

		if err != nil {
			return "", fmt.Errorf("failed to fetch azure-ad identity metadata. Error: %v", err.Error())
		}

		if resp == nil {
			return "", fmt.Errorf("failed to fetch azure-ad identity metadata. Error: empty response")
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
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
				return "", fmt.Errorf(errMsg)
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

	return "", fmt.Errorf(errMsg)
}
