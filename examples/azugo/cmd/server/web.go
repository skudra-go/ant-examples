package main

import (
	"azugo.io/azugo/server"

	"github.com/spf13/cobra"

	app "github.com/skudra-go/ant-examples/playground"
	"github.com/skudra-go/ant-examples/playground/router"
)

// webCmd represents the web command
var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start web server",
	Long: `Web server is the only thing you need to run,
and it takes care of all the other things for you`,
	RunE: runWeb,
}

func runWeb(cmd *cobra.Command, args []string) error {
	a, err := app.NewApp(cmd)
	if err != nil {
		return err
	}

	server.Run(router.RegisterRoutes(a))

	return nil
}

func init() {
	initRootCmd()
	RootCmd.AddCommand(webCmd)
}
