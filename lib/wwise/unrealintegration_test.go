package wwise

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mircearoata/wwise-cli/lib/wwise/client"
	"github.com/mircearoata/wwise-cli/lib/wwise/product"
	"github.com/spf13/viper"
)

const (
	testIntegrationVersion = "2023.1.14.3555"
	testManifest           = `{"data":{"id":"unrealintegration.2023_1_14_3555","name":"Unreal Integration","files":[` +
		`{"id":"f56","name":"UE56.tar.xz","groups":[{"groupId":"DeploymentPlatforms","groupValueId":"UE56"},{"groupId":"Packages","groupValueId":"Unreal"}]},` +
		`{"id":"f55","name":"UE55.tar.xz","groups":[{"groupId":"DeploymentPlatforms","groupValueId":"UE55"},{"groupId":"Packages","groupValueId":"Unreal"}]}` +
		`]}}`
)

// seedCachedIntegration lays out an offline cache dir holding the manifest
// and, when downloaded is true, an info.json marking UE56.tar.xz as fetched.
func seedCachedIntegration(t *testing.T, downloaded bool) {
	t.Helper()
	cacheDir := t.TempDir()
	viper.Set("cache-dir", cacheDir)
	t.Cleanup(func() { viper.Set("cache-dir", "") })

	version, err := product.NewWwiseProduct(client.NewOfflineClient(), "unrealintegration").GetVersion(testIntegrationVersion)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if err := os.WriteFile(filepath.Join(version.Dir, "version-manifest.json"), []byte(testManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if downloaded {
		if err := os.WriteFile(filepath.Join(version.Dir, "info.json"), []byte(`{"files":["UE56.tar.xz"],"groups":[]}`), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestUEDeploymentPlatformJoinsMajorAndMinor(t *testing.T) {
	got, err := UEDeploymentPlatform("5.6")
	if err != nil {
		t.Fatalf("UEDeploymentPlatform(5.6): %v", err)
	}
	if got != "UE56" {
		t.Fatalf("want UE56, got %q", got)
	}
}

func TestUEDeploymentPlatformRejectsAnythingButMajorDotMinor(t *testing.T) {
	for _, bad := range []string{"5", "5.6.1", "UE56", "5.x", ""} {
		if _, err := UEDeploymentPlatform(bad); err == nil {
			t.Errorf("UEDeploymentPlatform(%q) accepted", bad)
		}
	}
}

func TestFetchUnrealIntegrationReturnsTheCachedVersion(t *testing.T) {
	seedCachedIntegration(t, true)

	version, info, err := FetchUnrealIntegration(testIntegrationVersion, "UE56", client.NewOfflineClient())
	if err != nil {
		t.Fatalf("FetchUnrealIntegration from a complete cache: %v", err)
	}
	if version.VersionId != testIntegrationVersion {
		t.Fatalf("want version %s, got %s", testIntegrationVersion, version.VersionId)
	}
	if info.ID != "unrealintegration.2023_1_14_3555" {
		t.Fatalf("manifest not returned: %+v", info)
	}
}

func TestFetchUnrealIntegrationOfflineWithoutTheFileIsErrOffline(t *testing.T) {
	seedCachedIntegration(t, false)

	_, _, err := FetchUnrealIntegration(testIntegrationVersion, "UE56", client.NewOfflineClient())
	if !errors.Is(err, client.ErrOffline) {
		t.Fatalf("want ErrOffline, got %v", err)
	}
}

func TestFetchUnrealIntegrationWithNoFileForThePlatformFails(t *testing.T) {
	seedCachedIntegration(t, true)

	_, _, err := FetchUnrealIntegration(testIntegrationVersion, "UE42", client.NewOfflineClient())
	if err == nil || errors.Is(err, client.ErrOffline) {
		t.Fatalf("want a 'no integration file' error, got %v", err)
	}
}
