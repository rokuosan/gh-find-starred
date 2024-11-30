package cmd

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hugo",
	Short: "Hugo is a very fast static site generator",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("hi world, this is the gh-find-starred extension!")
		client, err := api.DefaultRESTClient()
		if err != nil {
			fmt.Println(err)
			return
		}
		response := struct{ Login string }{}
		err = client.Get("user", &response)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("running as %s\n", response.Login)
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
}
