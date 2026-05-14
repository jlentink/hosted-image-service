package cmd

import (
	log "github.com/jlentink/yaglogger"
	"github.com/spf13/cobra"

	"github.com/jlentink/image-service/internal/config"
	"github.com/jlentink/image-service/internal/server"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the image processing HTTP server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(getConfigFile())
		if err != nil {
			return err
		}

		log.SetLevelByString(cfg.Logging.Level)
		log.Info("Starting image-service %s on %s:%d", Version, cfg.Server.Host, cfg.Server.Port)

		srv := server.New(cfg)
		return srv.Run()
	},
}
