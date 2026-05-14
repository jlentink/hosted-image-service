package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jlentink/image-service/pkg/jwt"
)

var (
	tokenSecret string
	tokenDomain string
	tokenExpiry time.Duration
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generate a JWT token for testing",
	RunE: func(cmd *cobra.Command, args []string) error {
		if tokenSecret == "" {
			return fmt.Errorf("--secret is required")
		}
		if tokenDomain == "" {
			return fmt.Errorf("--domain is required")
		}

		token, err := jwt.GenerateToken(tokenSecret, tokenDomain, tokenExpiry)
		if err != nil {
			return err
		}

		fmt.Println(token)
		return nil
	},
}

func init() {
	tokenCmd.Flags().StringVar(&tokenSecret, "secret", "", "JWT shared secret (required)")
	tokenCmd.Flags().StringVar(&tokenDomain, "domain", "", "Domain claim to embed in the token (required)")
	tokenCmd.Flags().DurationVar(&tokenExpiry, "expiry", 5*time.Minute, "Token expiry duration")
	rootCmd.AddCommand(tokenCmd)
}
