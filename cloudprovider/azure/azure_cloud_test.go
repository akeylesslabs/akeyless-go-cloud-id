package azure

import (
	"context"
	"testing"
)

func TestCloudConfigFromAzEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantRes string
		wantScp string
	}{
		{"public AzureCloud", "AzureCloud", "https://management.azure.com/", "https://management.azure.com/.default"},
		{"public AzurePublicCloud", "AzurePublicCloud", "https://management.azure.com/", "https://management.azure.com/.default"},
		{"public case", "azurepubliccloud", "https://management.azure.com/", "https://management.azure.com/.default"},
		{"gov", "AzureUSGovernment", "https://management.usgovcloudapi.net/", "https://management.usgovcloudapi.net/.default"},
		{"gov alias", "AzureUSGovernmentCloud", "https://management.usgovcloudapi.net/", "https://management.usgovcloudapi.net/.default"},
		{"china", "AzureChinaCloud", "https://management.chinacloudapi.cn/", "https://management.chinacloudapi.cn/.default"},
		{"china alias", "AzureChinaCloud21Vianet", "https://management.chinacloudapi.cn/", "https://management.chinacloudapi.cn/.default"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := cloudConfigFromAzEnvironment(tt.input)
			if !ok {
				t.Fatal("expected known environment")
			}
			if got.resourceURL != tt.wantRes {
				t.Errorf("resourceURL = %q, want %q", got.resourceURL, tt.wantRes)
			}
			if got.scope != tt.wantScp {
				t.Errorf("scope = %q, want %q", got.scope, tt.wantScp)
			}
		})
	}
}

func TestCloudConfigFromAzEnvironment_unknown(t *testing.T) {
	_, ok := cloudConfigFromAzEnvironment("UnknownCloud")
	if ok {
		t.Fatal("expected unknown")
	}
}

func TestParseAzEnvironmentFromInstanceMetadata(t *testing.T) {
	body := []byte(`{"compute":{"azEnvironment":"AzureUSGovernment"}}`)
	got, ok := parseAzEnvironmentFromInstanceMetadata(body)
	if !ok || got != "AzureUSGovernment" {
		t.Fatalf("got %q, ok=%v", got, ok)
	}
}

func TestParseAzEnvironmentFromInstanceMetadata_empty(t *testing.T) {
	_, ok := parseAzEnvironmentFromInstanceMetadata([]byte(`{"compute":{}}`))
	if ok {
		t.Fatal("expected false")
	}
	_, ok = parseAzEnvironmentFromInstanceMetadata([]byte(`not json`))
	if ok {
		t.Fatal("expected false for invalid json")
	}
}

func TestCloudFromEnv_AZURE_CLOUD(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == envAzureCloud {
			return "AzureUSGovernment", true
		}
		return "", false
	}
	got, ok := cloudFromEnv(lookup)
	if !ok {
		t.Fatal("expected env match")
	}
	if got.resourceURL != usGovAzureCloud.resourceURL {
		t.Errorf("got %q", got.resourceURL)
	}
}

func TestAzureCloudDetector_envWins(t *testing.T) {
	d := &azureCloudDetector{
		lookupEnv: func(key string) (string, bool) {
			if key == envAzureEnvironment {
				return "AzureChinaCloud", true
			}
			return "", false
		},
		fetchMeta: func(context.Context) ([]byte, error) {
			t.Error("IMDS should not be called when env is set")
			return nil, errIMDSNotOK
		},
	}
	got := d.detect()
	if got.resourceURL != chinaAzureCloud.resourceURL {
		t.Errorf("got %q", got.resourceURL)
	}
}

func TestAzureCloudDetector_imds(t *testing.T) {
	meta := []byte(`{"compute":{"azEnvironment":"AzurePublicCloud"}}`)
	d := &azureCloudDetector{
		lookupEnv: func(string) (string, bool) { return "", false },
		fetchMeta: func(context.Context) ([]byte, error) { return meta, nil },
	}
	got := d.detect()
	if got.resourceURL != publicAzureCloud.resourceURL {
		t.Errorf("got %q", got.resourceURL)
	}
}

func TestAzureCloudDetector_fallbackPublic(t *testing.T) {
	d := &azureCloudDetector{
		lookupEnv: func(string) (string, bool) { return "", false },
		fetchMeta: func(context.Context) ([]byte, error) { return nil, errIMDSNotOK },
	}
	got := d.detect()
	if got.resourceURL != publicAzureCloud.resourceURL {
		t.Errorf("got %q", got.resourceURL)
	}
}

func TestAzureCloudDetector_imdsUnknownUsesPublic(t *testing.T) {
	meta := []byte(`{"compute":{"azEnvironment":"AzureStackHub"}}`)
	d := &azureCloudDetector{
		lookupEnv: func(string) (string, bool) { return "", false },
		fetchMeta: func(context.Context) ([]byte, error) { return meta, nil },
	}
	got := d.detect()
	if got.resourceURL != publicAzureCloud.resourceURL {
		t.Errorf("got %q", got.resourceURL)
	}
}
