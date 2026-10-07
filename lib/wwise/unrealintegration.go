package wwise

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mircearoata/wwise-cli/lib/unrealengine"
	"github.com/mircearoata/wwise-cli/lib/wwise/client"
	"github.com/mircearoata/wwise-cli/lib/wwise/product"
	"github.com/pkg/errors"

	cp "github.com/otiai10/copy"
)

// UEDeploymentPlatform turns an engine version such as "5.6" into the
// DeploymentPlatforms group value the Wwise manifest files it under ("UE56").
func UEDeploymentPlatform(engineVersion string) (string, error) {
	parts := strings.Split(engineVersion, ".")
	if len(parts) != 2 {
		return "", errors.New("engine version must be <major>.<minor>: " + engineVersion)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", errors.Wrap(err, "invalid engine major version")
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", errors.Wrap(err, "invalid engine minor version")
	}
	return fmt.Sprintf("UE%d%d", major, minor), nil
}

// FetchUnrealIntegration makes sure the Unreal integration package for the
// given engine deployment platform is in the cache, and returns the cached
// version together with its manifest. It is the download half of
// IntegrateWwiseUnreal: nothing is copied into a project.
func FetchUnrealIntegration(integrationVersion string, ueDeploymentPlatform string, wwiseClient *client.WwiseClient) (*product.WwiseProductVersion, product.ProductVersionInfo, error) {
	ueIntegrationProduct := product.NewWwiseProduct(wwiseClient, "unrealintegration")

	ueIntegrationVersion, err := ueIntegrationProduct.GetVersion(integrationVersion)
	if err != nil {
		return nil, product.ProductVersionInfo{}, errors.Wrap(err, "failed to get unreal integration version")
	}

	versionInfo, err := ueIntegrationVersion.GetInfo()
	if err != nil {
		return nil, product.ProductVersionInfo{}, errors.Wrap(err, "failed to get wwise manifest")
	}

	integrationFiles := versionInfo.FindFilesByGroups([]product.GroupFilter{
		{GroupID: "DeploymentPlatforms", GroupValues: []string{ueDeploymentPlatform}},
		{GroupID: "Packages", GroupValues: []string{"Unreal"}},
	})

	if len(integrationFiles) == 0 {
		return nil, product.ProductVersionInfo{}, errors.New("failed to find integration file for " + ueDeploymentPlatform)
	}

	if len(integrationFiles) > 1 {
		return nil, product.ProductVersionInfo{}, errors.New("found more than one integration file for " + ueDeploymentPlatform)
	}

	err = ueIntegrationVersion.DownloadOrCache(integrationFiles[0])
	if err != nil {
		return nil, product.ProductVersionInfo{}, errors.Wrap(err, "failed to download integration file")
	}

	return ueIntegrationVersion, versionInfo, nil
}

func IntegrateWwiseUnreal(uprojectFilePath string, integrationVersion string, wwiseClient *client.WwiseClient) error {
	if filepath.Ext(uprojectFilePath) != ".uproject" {
		return errors.New("invalid project path: " + uprojectFilePath)
	}

	if _, err := os.Stat(uprojectFilePath); os.IsNotExist(err) {
		return errors.Wrap(err, "project path does not exist")
	}

	// Get UE version from project file
	engineRoot, err := unrealengine.GetEngineRootFromProject(uprojectFilePath)
	if err != nil {
		return errors.Wrap(err, "failed to get engine root")
	}

	engineBuild, err := unrealengine.GetEngineVersionData(engineRoot)
	if err != nil {
		return errors.Wrap(err, "failed to get engine build")
	}

	wwiseUEDeploymentPlatform := fmt.Sprintf("UE%d%d", engineBuild.MajorVersion, engineBuild.MinorVersion)

	ueIntegrationVersion, versionInfo, err := FetchUnrealIntegration(integrationVersion, wwiseUEDeploymentPlatform, wwiseClient)
	if err != nil {
		return err
	}

	wwiseSDKVersion := fmt.Sprintf("%d.%d.%d.%d", versionInfo.Version.Year, versionInfo.Version.Major, versionInfo.Version.Minor, versionInfo.ProductDependentData.WwiseSdkBuild)

	sdkProduct := product.NewWwiseProduct(wwiseClient, "wwise")
	sdkProductVersion, err := sdkProduct.GetVersion(fmt.Sprintf("wwise.%s", wwiseSDKVersion))
	if err != nil {
		return errors.Wrap(err, "failed to get sdk version")
	}

	type SDKIntegrationAsset struct {
		Source      string
		Destination string
	}

	sdkAssets := []SDKIntegrationAsset{}

	if versionInfo.ProductDependentData.PlatformFolders != nil {
		for _, folder := range versionInfo.ProductDependentData.PlatformFolders.Mandatory {
			if _, err := os.Stat(filepath.Join(sdkProductVersion.Dir, "SDK", folder)); os.IsNotExist(err) {
				return errors.New("failed to find mandatory folder: " + folder)
			}
			sdkAssets = append(sdkAssets, SDKIntegrationAsset{
				Source:      folder,
				Destination: folder,
			})
		}

		for _, folder := range versionInfo.ProductDependentData.PlatformFolders.Optional {
			if _, err := os.Stat(filepath.Join(sdkProductVersion.Dir, "SDK", folder)); !os.IsNotExist(err) {
				sdkAssets = append(sdkAssets, SDKIntegrationAsset{
					Source:      folder,
					Destination: folder,
				})
			}
		}
	} else if versionInfo.ProductDependentData.SdkPlatformFolders != nil {
		for _, platformInfos := range *versionInfo.ProductDependentData.SdkPlatformFolders {
			for _, platformInfo := range platformInfos {
				// TODO: Use FileMatchExpression, but for now it seems like it's always "*"

				if platformInfo.SinceEngine != nil {
					major, err := strconv.Atoi(platformInfo.SinceEngine.Major)
					if err != nil {
						return errors.Wrap(err, "failed to parse major version")
					}
					minor, err := strconv.Atoi(platformInfo.SinceEngine.Minor)
					if err != nil {
						return errors.Wrap(err, "failed to parse minor version")
					}
					if engineBuild.MajorVersion < major || (engineBuild.MajorVersion == major && engineBuild.MinorVersion < minor) {
						continue
					}
				}

				if platformInfo.UntilEngine != nil {
					major, err := strconv.Atoi(platformInfo.UntilEngine.Major)
					if err != nil {
						return errors.Wrap(err, "failed to parse major version")
					}
					minor, err := strconv.Atoi(platformInfo.UntilEngine.Minor)
					if err != nil {
						return errors.Wrap(err, "failed to parse minor version")
					}
					if engineBuild.MajorVersion > major || (engineBuild.MajorVersion == major && engineBuild.MinorVersion > minor) {
						continue
					}
				}

				if platformInfo.Optional {
					if _, err := os.Stat(filepath.Join(sdkProductVersion.Dir, "SDK", platformInfo.Source)); !os.IsNotExist(err) {
						sdkAssets = append(sdkAssets, SDKIntegrationAsset{
							Source:      platformInfo.Source,
							Destination: platformInfo.Destination,
						})
					}
				} else {
					if _, err := os.Stat(filepath.Join(sdkProductVersion.Dir, "SDK", platformInfo.Source)); os.IsNotExist(err) {
						return errors.New("failed to find mandatory folder: " + platformInfo.Source)
					}
					sdkAssets = append(sdkAssets, SDKIntegrationAsset{
						Source:      platformInfo.Source,
						Destination: platformInfo.Destination,
					})
				}
			}
		}
	} else {
		return errors.New("failed to find platform folders")
	}

	integrationAssets, err := os.ReadDir(ueIntegrationVersion.Dir)
	if err != nil {
		return errors.Wrap(err, "failed to read integration cache path")
	}

	projectRoot := filepath.Dir(uprojectFilePath)
	for _, entry := range integrationAssets {
		if entry.IsDir() {
			err = cp.Copy(filepath.Join(ueIntegrationVersion.Dir, entry.Name()), filepath.Join(projectRoot, "Plugins", entry.Name()))
			if err != nil {
				return errors.Wrapf(err, "failed to copy integration asset: %s", entry.Name())
			}
		}
	}

	mainWwisePlugin := filepath.Join(projectRoot, "Plugins", "Wwise")
	for _, sdkAsset := range sdkAssets {
		err = cp.Copy(filepath.Join(sdkProductVersion.Dir, "SDK", sdkAsset.Source), filepath.Join(mainWwisePlugin, "ThirdParty", sdkAsset.Destination))
		if err != nil {
			return errors.Wrap(err, "failed to copy third party files")
		}
	}

	return nil
}
