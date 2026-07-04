package serve

import (
	"fmt"
	"os"

	"github.com/k0u3h1k/bare-metal/pkg/model"
	"github.com/k0u3h1k/bare-metal/pkg/server"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound serve` command.
var Cmd = &cobra.Command{
	Use:   "serve <model-name>",
	Short: "Start a headless OpenAI-compatible API server",
	Long: `Loads a model and starts a local HTTP server that exposes
an OpenAI-compatible API. Compatible with any OpenAI SDK, LangChain,
curl, and other LLM tools.

The API server proxies requests to a local llama-server instance running
in the background.

Examples:
  unbound serve llama3.2:1b
  unbound serve mistral:7b --port 8081
  unbound serve qwen2:7b --host 0.0.0.0
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")

		fmt.Printf("🔧 Unbound — serving model: %s\n", modelName)

		mgr := model.NewManager()

		// Load the model (download if needed, start llama-server)
		if err := mgr.Load(modelName, port); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
			return err
		}

		// Get the inference URL
		inferenceURL := mgr.GetInferenceURL()
		if inferenceURL == "" {
			return fmt.Errorf("inference server not available")
		}

		fmt.Printf("🌐 API endpoint: http://%s:%d/v1\n", host, port)
		fmt.Printf("📎 Proxying to internal llama-server at: %s\n", inferenceURL)
		fmt.Println("   Compatible with: OpenAI SDK, LangChain, curl, etc.")
		fmt.Println("\n📋 Press Ctrl+C to stop the server.")

		// Start the proxy server
		return server.Start(modelName, host, port, inferenceURL)
	},
}

func init() {
	Cmd.Flags().IntP("port", "p", 8080, "Port to serve on")
	Cmd.Flags().StringP("host", "H", "127.0.0.1", "Host to bind to")
}
