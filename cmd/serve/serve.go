package serve

import (
	"fmt"

	"github.com/k0u3h1k/bare-metal/pkg/server"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound serve` command.
var Cmd = &cobra.Command{
	Use:   "serve <model-name>",
	Short: "Start a headless OpenAI-compatible API server",
	Long: `Starts a local HTTP server that exposes an OpenAI-compatible API
on the configured host and port. Useful for integrating Unbound models into
other tools (LangChain, custom UIs, scripts, etc.).

Example:
  unbound serve llama3.2:1b
  unbound serve mistral:7b --port 8081
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")

		fmt.Printf("🔧 Unbound — serving model: %s\n", modelName)
		fmt.Printf("🌐 API endpoint: http://%s:%d/v1\n", host, port)
		fmt.Println("   Compatible with: OpenAI SDK, LangChain, curl, etc.")

		// TODO: load model and start API server
		fmt.Println("⚠️  API server is a placeholder — model loading not yet implemented")

		return server.Start(modelName, host, port)
	},
}

func init() {
	Cmd.Flags().IntP("port", "p", 8080, "Port to serve on")
	Cmd.Flags().StringP("host", "H", "127.0.0.1", "Host to bind to")
}
