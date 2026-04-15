package azure

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

const (
	envAzureEnvironment = "AZURE_ENVIRONMENT"
	envAzureCloud       = "AZURE_CLOUD"
)

// instanceMetadataAPIVersion is used to read compute.azEnvironment from IMDS.
const instanceMetadataAPIVersion = "2021-02-01"

// imdsHost is the link-local address for the Azure Instance Metadata Service (IMDS).
// Microsoft documents this literal IP (not a hostname) for VMs and many Azure-hosted services;
// see https://learn.microsoft.com/en-us/azure/virtual-machines/instance-metadata-service
const imdsHost = "169.254.169.254"

// imdsHTTPClient is reused for IMDS requests (timeouts, connection reuse).
var imdsHTTPClient = &http.Client{Timeout: 5 * time.Second}

type azureCloudConfig struct {
	resourceURL string
	scope       string
	cloud       cloud.Configuration
}

var publicAzureCloud = azureCloudConfig{
	resourceURL: "https://management.azure.com/",
	scope:       "https://management.azure.com/.default",
	cloud:       cloud.AzurePublic,
}

var usGovAzureCloud = azureCloudConfig{
	resourceURL: "https://management.usgovcloudapi.net/",
	scope:       "https://management.usgovcloudapi.net/.default",
	cloud:       cloud.AzureGovernment,
}

var chinaAzureCloud = azureCloudConfig{
	resourceURL: "https://management.chinacloudapi.cn/",
	scope:       "https://management.chinacloudapi.cn/.default",
	cloud:       cloud.AzureChina,
}

var (
	resolvedCloud    azureCloudConfig
	resolveCloudOnce sync.Once
)

func resolvedAzureCloud() azureCloudConfig {
	resolveCloudOnce.Do(func() {
		resolvedCloud = newAzureCloudDetector().detect()
	})
	return resolvedCloud
}

type azureCloudDetector struct {
	lookupEnv func(key string) (string, bool)
	fetchMeta func(ctx context.Context) ([]byte, error)
}

func newAzureCloudDetector() *azureCloudDetector {
	return &azureCloudDetector{
		lookupEnv: os.LookupEnv,
		fetchMeta: defaultFetchInstanceMetadata,
	}
}

func (d *azureCloudDetector) detect() azureCloudConfig {
	if d.lookupEnv == nil {
		d.lookupEnv = os.LookupEnv
	}
	if d.fetchMeta == nil {
		d.fetchMeta = defaultFetchInstanceMetadata
	}
	if cfg, ok := cloudFromEnv(d.lookupEnv); ok {
		return cfg
	}
	body, err := d.fetchMeta(context.Background())
	if err != nil {
		return publicAzureCloud
	}
	if azEnv, ok := parseAzEnvironmentFromInstanceMetadata(body); ok {
		if cfg, ok := cloudConfigFromAzEnvironment(azEnv); ok {
			return cfg
		}
	}
	return publicAzureCloud
}

func cloudFromEnv(lookup func(string) (string, bool)) (azureCloudConfig, bool) {
	for _, key := range []string{envAzureEnvironment, envAzureCloud} {
		v, ok := lookup(key)
		if !ok || strings.TrimSpace(v) == "" {
			continue
		}
		if cfg, ok := cloudConfigFromAzEnvironment(v); ok {
			return cfg, true
		}
	}
	return azureCloudConfig{}, false
}

func cloudConfigFromAzEnvironment(raw string) (azureCloudConfig, bool) {
	key := strings.ToLower(strings.TrimSpace(raw))
	switch key {
	case "azurecloud", "azurepubliccloud":
		return publicAzureCloud, true
	case "azureusgovernment", "azureusgovernmentcloud":
		return usGovAzureCloud, true
	case "azurechinacloud", "azurechinacloud21vianet":
		return chinaAzureCloud, true
	default:
		return azureCloudConfig{}, false
	}
}

func parseAzEnvironmentFromInstanceMetadata(body []byte) (string, bool) {
	var doc struct {
		Compute struct {
			AzEnvironment string `json:"azEnvironment"`
		} `json:"compute"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return "", false
	}
	if strings.TrimSpace(doc.Compute.AzEnvironment) == "" {
		return "", false
	}
	return doc.Compute.AzEnvironment, true
}

func defaultFetchInstanceMetadata(ctx context.Context) ([]byte, error) {
	u := "http://" + imdsHost + "/metadata/instance?api-version=" + instanceMetadataAPIVersion + "&format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata", "true")

	resp, err := imdsHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, errIMDSNotOK
	}
	return io.ReadAll(resp.Body)
}

var errIMDSNotOK = errors.New("instance metadata non-200")
