package serve

import (
"fmt"
"net"
"os"
"os/signal"
"syscall"

"github.com/k0u3h1k/bare-metal/pkg/model"
"github.com/k0u3h1k/bare-metal/pkg/server"
"github.com/spf13/cobra"
)

// Cmd represents the `unbound serve` command.
var Cmd = &cobra.Command{
Use: "serve <model-name>", Short: "Start a model with an OpenAI-compatible API",
Args: cobra.ExactArgs(1),
RunE: func(cmd *cobra.Command, args []string) error {
modelName := args[0]
apiPort, _ := cmd.Flags().GetInt("port")
llamaPort, _ := cmd.Flags().GetInt("llama-port")
host, _ := cmd.Flags().GetString("host")
if llamaPort == 0 {
var err error
llamaPort, err = freePort()
if err != nil { return fmt.Errorf("selecting llama port: %w", err) }
}
fmt.Printf("🔧 Unbound — serving model: %s\n\n", modelName)
mgr := model.NewManager()
if err := mgr.Load(modelName, llamaPort); err != nil { return fmt.Errorf("loading model: %w", err) }
inferenceURL := mgr.GetInferenceURL()
if inferenceURL == "" { _ = mgr.Unload(modelName); return fmt.Errorf("inference server not available") }
fmt.Printf("🌐 API endpoint: http://%s:%d/v1\n", host, apiPort)
fmt.Printf("📎 Proxying to internal llama-server at: %s\n", inferenceURL)
stop := make(chan os.Signal, 1)
signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
defer signal.Stop(stop)
go func() { <-stop; _ = mgr.Unload(modelName); os.Exit(0) }()
return server.Start(modelName, host, apiPort, inferenceURL)
},
}

func freePort() (int, error) {
listener, err := net.Listen("tcp", "127.0.0.1:0")
if err != nil { return 0, err }
defer listener.Close()
return listener.Addr().(*net.TCPAddr).Port, nil
}

func init() {
Cmd.Flags().IntP("port", "p", 8080, "API port (default: 8080)")
Cmd.Flags().Int("llama-port", 0, "llama-server port (default: 0 = auto-select)")
Cmd.Flags().StringP("host", "H", "127.0.0.1", "Host to bind to")
}
