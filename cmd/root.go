package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mircearoata/wwise-cli/lib/wwise/client"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var rootCmd = &cobra.Command{
	Use: "wwise-cli",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		viper.SetEnvPrefix("wwise")
		viper.AutomaticEnv()

		// A pre-obtained bearer token (--token / WWISE_TOKEN) bypasses the
		// email/password login. Required for accounts migrated to the new
		// (Cognito) sign-in, where the legacy login endpoint rejects
		// email/password; the token can be taken from an active Wwise
		// Launcher session (the fCRM_jwt cookie).
		if viper.IsSet("token") {
			wwiseClient := client.NewWwiseClientWithToken(viper.GetString("token"))
			cmd.SetContext(NewContextWithClient(cmd.Context(), wwiseClient))
			return nil
		}

		if !viper.IsSet("email") {
			fmt.Print("Enter Wwise email: ")
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Scan()
			email := scanner.Text()
			viper.Set("email", email)
		}
		if !viper.IsSet("password") {
			fmt.Print("Enter Wwise password: ")
			pass, err := term.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				return err
			}
			viper.Set("password", string(pass))
			fmt.Println()
		}

		email := viper.GetString("email")
		password := viper.GetString("password")

		wwiseClient := client.NewWwiseClient()

		err := wwiseClient.Authenticate(email, password)
		if err != nil {
			return errors.Wrap(err, "authentication error. check your Wwise credentials")
		}

		cmd.SetContext(NewContextWithClient(cmd.Context(), wwiseClient))

		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().String("email", "", "Wwise account email")
	rootCmd.PersistentFlags().String("password", "", "Wwise account password")
	rootCmd.PersistentFlags().String("token", "", "Pre-obtained bearer token (skips email/password login)")

	userCache, err := os.UserCacheDir()
	if err != nil {
		userCache = "."
	}
	cacheDir := filepath.Join(userCache, "wwise-cli")
	rootCmd.PersistentFlags().String("cache-dir", cacheDir, "Cache directory")

	_ = viper.BindPFlag("email", rootCmd.PersistentFlags().Lookup("email"))
	_ = viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	_ = viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
	_ = viper.BindPFlag("cache-dir", rootCmd.PersistentFlags().Lookup("cache-dir"))
}
