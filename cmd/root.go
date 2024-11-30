package cmd

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-find-starred",
	Short: "gh-find-starred is a GitHub CLI extension to find your starred repositories.",
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
