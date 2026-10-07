package cmd

import (
	"fmt"

	"github.com/mircearoata/wwise-cli/lib/wwise"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fetchUEIntegrationCmd = &cobra.Command{
	Use:   "fetch-ue-integration",
	Short: "Download the Wwise Unreal integration package into the cache without integrating it",
	Long: `Downloads the Unreal integration package for one engine version into the
cache directory and stops there. Together with "download" this fills a cache
that a later "integrate-ue --offline" can run from without logging in.`,
	// integrate-ue binds the same "integration-version" key at init, and viper
	// keeps only the last binding; rebinding here points it at this command's
	// flags for the duration of this run.
	PreRunE: func(cmd *cobra.Command, args []string) error {
		return viper.BindPFlags(cmd.Flags())
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		integrationVersion := viper.GetString("integration-version")
		engineVersion := viper.GetString("ue")

		ueDeploymentPlatform, err := wwise.UEDeploymentPlatform(engineVersion)
		if err != nil {
			return err
		}

		wwiseClient, ok := ClientFromContext(cmd.Context())
		if !ok {
			return errors.New("could not get Wwise client from context")
		}

		fmt.Printf("Fetching Wwise Unreal integration %s for UE %s...\n", integrationVersion, engineVersion)

		version, _, err := wwise.FetchUnrealIntegration(integrationVersion, ueDeploymentPlatform, wwiseClient)
		if err != nil {
			return errors.Wrap(err, "could not fetch the Unreal integration")
		}

		fmt.Printf("Cached at %s\n", version.Dir)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fetchUEIntegrationCmd)

	fetchUEIntegrationCmd.Flags().String("integration-version", "", "Wwise UE integration version to download")
	fetchUEIntegrationCmd.MarkFlagRequired("integration-version")
	fetchUEIntegrationCmd.Flags().String("ue", "", "Unreal Engine version the integration is for, as <major>.<minor> (e.g. 5.6)")
	fetchUEIntegrationCmd.MarkFlagRequired("ue")
}
