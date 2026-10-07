package product

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mircearoata/wwise-cli/lib/wwise/client"
	"github.com/spf13/viper"
)

func offlineVersion(t *testing.T, cacheDir string) *WwiseProductVersion {
	t.Helper()
	viper.Set("cache-dir", cacheDir)
	t.Cleanup(func() { viper.Set("cache-dir", "") })

	version, err := NewWwiseProduct(client.NewOfflineClient(), "unrealintegration").GetVersion("2023.1.14.3555")
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	return version
}

func TestOfflineGetInfoReadsTheCachedManifest(t *testing.T) {
	version := offlineVersion(t, t.TempDir())
	manifest := `{"data":{"id":"unrealintegration.2023_1_14_3555","name":"Unreal Integration","files":[{"id":"f1","name":"UE56.tar.xz","groups":[{"groupId":"DeploymentPlatforms","groupValueId":"UE56"}]}]}}`
	if err := os.WriteFile(filepath.Join(version.Dir, manifestFileName), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	info, err := version.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo offline with a cached manifest: %v", err)
	}
	if info.ID != "unrealintegration.2023_1_14_3555" || len(info.Files) != 1 || info.Files[0].Name != "UE56.tar.xz" {
		t.Fatalf("manifest not round-tripped: %+v", info)
	}
}

func TestOfflineGetInfoWithoutAManifestIsErrOffline(t *testing.T) {
	version := offlineVersion(t, t.TempDir())

	_, err := version.GetInfo()
	if !errors.Is(err, client.ErrOffline) {
		t.Fatalf("expected ErrOffline, got %v", err)
	}
}

func TestOfflineDownloadOfAnUncachedFileIsErrOffline(t *testing.T) {
	version := offlineVersion(t, t.TempDir())

	err := version.DownloadOrCache(File{Name: "UE56.tar.xz", URL: "https://example.invalid/UE56.tar.xz"})
	if !errors.Is(err, client.ErrOffline) {
		t.Fatalf("expected ErrOffline, got %v", err)
	}
}

func TestOfflineDownloadOfACachedFileIsANoOp(t *testing.T) {
	version := offlineVersion(t, t.TempDir())
	version.downloadedInfo.Files = []string{"UE56.tar.xz"}

	if err := version.DownloadOrCache(File{Name: "UE56.tar.xz", URL: "https://example.invalid/UE56.tar.xz"}); err != nil {
		t.Fatalf("a cached file must not need the network: %v", err)
	}
}

func TestOfflineClientRefusesTheAPI(t *testing.T) {
	_, err := client.NewOfflineClient().SendRequest("GET", "/products/versions/?category=wwise", nil)
	if !errors.Is(err, client.ErrOffline) {
		t.Fatalf("expected ErrOffline, got %v", err)
	}
}
