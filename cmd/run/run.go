package run

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"net"

	"github.com/k0u3h1k/bare-metal/pkg/console"
	"github.com/k0u3h1k/bare-metal/pkg/model"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound run` command.
var Cmd = &cobra.Command{
	Use:   "run <model-name>",
	Short: "Download (if needed) and start an interactive chat with a model",
	Long: `Downloads a model from Hugging Face if not already cached,
then starts an interactive terminal chat session with real inference via llama.cpp.
Models can be granted full system permissions (shell, file I/O, internet)
with explicit consent.

Examples:
  unbound run llama3.2:1b
  unbound run mistral:7b
  unbound run hugging-quants/Meta-Llama-3.1-8B-Instruct-Q4_K_M-GGUF
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		autoApprove, _ := cmd.Flags().GetBool("auto-approve")
		systemPrompt, _ := cmd.Flags().GetString("system-prompt")
		maxTokens, _ := cmd.Flags().GetInt("max-tokens")
		port, _ := cmd.Flags().GetInt("port")

		fmt.Printf("🔧 Unbound — running model: %s\n\n", modelName)

		mgr := model.NewManager()

		// Load the model (download if needed, start llama-server)
		loadPort := port
		if loadPort == 0 {
			loadPort = 8080
		}

		if err := mgr.Load(modelName, loadPort); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading model: %v\n", err)
			return err
		}

		// Get the inference URL
		inferenceURL := mgr.GetInferenceURL()
		if inferenceURL == "" {
			return fmt.Errorf("inference server not available")
		}

		fmt.Printf("💬 Starting interactive chat (inference at %s)\n", inferenceURL)
		if autoApprove {
			fmt.Println("🔓 Auto-approving all permission requests (--auto-approve)")
			previous, existed := os.LookupEnv("UNBOUND_ALLOW_ALL")
			if err := os.Setenv("UNBOUND_ALLOW_ALL", "1"); err != nil {
				return fmt.Errorf("enabling unrestricted mode: %w", err)
			}
			defer func() {
				if existed { _ = os.Setenv("UNBOUND_ALLOW_ALL", previous) } else { _ = os.Unsetenv("UNBOUND_ALLOW_ALL") }
			}()
		}
		if systemPrompt != "" {
			fmt.Printf("📝 System prompt: %s\n", systemPrompt)
		}

		// Run the interactive console with real inference
		err := console.RunInteractiveWithInference(modelName, inferenceURL, systemPrompt, maxTokens)

		// Clean up
		fmt.Println("\n🛑 Stopping model...")
		if unloadErr := mgr.Unload(modelName); unloadErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: error unloading model: %v\n", unloadErr)
		}

		return err
	},
}

func init() {
	Cmd.Flags().BoolP("auto-approve", "a", false, "Skip permission prompts (auto-approve all actions for this session)")
	Cmd.Flags().StringP("system-prompt", "s", "", "Custom system prompt for the model")
	Cmd.Flags().IntP("max-tokens", "m", 2048, "Maximum tokens per response")
	Cmd.Flags().IntP("port", "p", 0, "Port for llama-server (default: 0 = auto-select)")
}

func freePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return 0, err }
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
