package console

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strings"

    "github.com/k0u3h1k/bare-metal/pkg/permissions"
    "github.com/k0u3h1k/bare-metal/pkg/shell"
)

// Message represents a chat message in the conversation.
type Message struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

// RunInteractive starts an interactive terminal chat session without inference.
func RunInteractive(modelName string) error {
    reader := bufio.NewReader(os.Stdin)

    fmt.Printf("\n━━━ Unbound Interactive Chat — %s ━━━\n", modelName)
    fmt.Println("Type '/help' for commands, '/exit' to quit.")
    fmt.Println()

    for {
        fmt.Print("> ")
        input, err := reader.ReadString('\n')
        if err != nil {
            return fmt.Errorf("read error: %w", err)
        }

        input = strings.TrimSpace(input)

        switch {
        case input == "":
            continue
        case input == "/exit" || input == "/quit":
            fmt.Println("Goodbye!")
            return nil
        case input == "/help":
            printHelp()
        case strings.HasPrefix(input, "/"):
            handleCommand(input, nil)
        default:
            fmt.Printf("🤖 %s > (placeholder — model not connected)\n", modelName)
            fmt.Printf("   You said: %s\n", input)
        }
    }
}

// RunInteractiveWithInference starts an interactive chat with real inference
// and full permission gating for shell commands, file I/O, and network access.
func RunInteractiveWithInference(modelName, inferenceURL, systemPrompt string, maxTokens int) error {
    reader := bufio.NewReader(os.Stdin)

    fmt.Printf("\n━━━ Unbound Interactive Chat — %s ━━━\n", modelName)
    fmt.Println("Type '/help' for commands, '/exit' to quit.")
    fmt.Println("The model can execute shell commands, read/write files, and access the network")
    fmt.Println("with your explicit permission. Use '/allow <cmd>' to pre-approve patterns.")
    fmt.Println()

    // Build conversation history
    var messages []Message
    if systemPrompt != "" {
        messages = append(messages, Message{Role: "system", Content: systemPrompt})
    }

    // Reset session-level permissions
    permissions.DefaultSessionTracker.Clear()

    for {
        fmt.Print("> ")
        input, err := reader.ReadString('\n')
        if err != nil {
            return fmt.Errorf("read error: %w", err)
        }

        input = strings.TrimSpace(input)

        switch {
        case input == "":
            continue
        case input == "/exit" || input == "/quit":
            fmt.Println("Goodbye!")
            return nil
        case input == "/help":
            printHelp()
            continue
        case input == "/clear":
            messages = []Message{}
            if systemPrompt != "" {
                messages = append(messages, Message{Role: "system", Content: systemPrompt})
            }
            fmt.Println("🧹 Conversation cleared.")
            continue
        case input == "/perms":
            printPermissions()
            continue
        case strings.HasPrefix(input, "/"):
            handleCommand(input, &messages)
            continue
        }

        // Add user message
        messages = append(messages, Message{Role: "user", Content: input})

        // Get response from inference server
        fmt.Print("🤖 ")
        response, err := chatCompletion(inferenceURL, messages, maxTokens)
        if err != nil {
            fmt.Printf("\n⚠️  Error: %v\n", err)
            // Remove the user message since we couldn't get a response
            messages = messages[:len(messages)-1]
            continue
        }

        fmt.Println()
        fmt.Println()

        // Process the response for tool calls (shell commands, file I/O, network)
        processedResponse, toolResults := processToolCalls(response, inferenceURL, maxTokens)

        // If there were tool calls, we need to send the results back to the model
        if len(toolResults) > 0 {
            // Add assistant response (with tool call indicators) to history
            messages = append(messages, Message{Role: "assistant", Content: processedResponse})

            // Add tool results as system messages for the model to see
            for _, tr := range toolResults {
                messages = append(messages, Message{Role: "system", Content: tr})
            }

            // Get the model's follow-up response considering the tool output
            fmt.Print("🤖 ")
            followUp, err := chatCompletion(inferenceURL, messages, maxTokens)
            if err != nil {
                fmt.Printf("\n⚠️  Error: %v\n", err)
                continue
            }
            fmt.Println()
            fmt.Println()

            messages = append(messages, Message{Role: "assistant", Content: followUp})
        } else {
            // No tool calls, just add the response
            messages = append(messages, Message{Role: "assistant", Content: response})
        }
    }
}

// processToolCalls scans model output for tool calls (shell commands, file ops, network)
// and executes them with user permission. Returns the filtered response and tool results.
func processToolCalls(response string, inferenceURL string, maxTokens int) (string, []string) {
    var toolResults []string

    // Detect shell commands in ```bash or ```sh blocks
    processed := detectAndExecuteShellCommands(response, &toolResults, inferenceURL, maxTokens)

    // Detect file read/write operations
    processed = detectAndExecuteFileOps(processed, &toolResults, inferenceURL, maxTokens)

    // Detect network/curl requests
    processed = detectAndExecuteNetworkOps(processed, &toolResults, inferenceURL, maxTokens)

    return processed, toolResults
}

// detectAndExecuteShellCommands finds ```bash or ```sh code blocks and executes them.
func detectAndExecuteShellCommands(response string, toolResults *[]string, inferenceURL string, maxTokens int) string {
    lines := strings.Split(response, "\n")
    var resultLines []string
    inBlock := false
    var commandBuffer []string

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)

        if !inBlock && (trimmed == "```bash" || trimmed == "```sh" || trimmed == "```shell") {
            inBlock = true
            commandBuffer = nil
            continue
        }

        if inBlock && trimmed == "```" {
            inBlock = false
            cmd := strings.Join(commandBuffer, "\n")
            cmd = strings.TrimSpace(cmd)

            if cmd != "" {
                // Execute the command with user permission
                result := executeShellCommandWithPermission(cmd, inferenceURL, maxTokens)
                *toolResults = append(*toolResults, result)

                // Replace the block with a summary
                resultLines = append(resultLines, fmt.Sprintf("_Executed: %s_", truncate(cmd, 60)))
            }
            continue
        }

        if inBlock {
            commandBuffer = append(commandBuffer, line)
            continue
        }

        resultLines = append(resultLines, line)
    }

    // If we're still in a block at the end, add it back
    if inBlock && len(commandBuffer) > 0 {
        resultLines = append(resultLines, commandBuffer...)
    }

    return strings.Join(resultLines, "\n")
}

// detectAndExecuteFileOps finds file read/write patterns in model output.
func detectAndExecuteFileOps(response string, toolResults *[]string, inferenceURL string, maxTokens int) string {
    // Look for file operations in code blocks
    lines := strings.Split(response, "\n")
    var resultLines []string
    inBlock := false
    var commandBuffer []string

    for _, line := range lines {
        trimmed := strings.TrimSpace(line)

        if !inBlock && (trimmed == "```" || strings.HasPrefix(trimmed, "```")) {
            inBlock = true
            commandBuffer = nil
            continue
        }

        if inBlock && trimmed == "```" {
            inBlock = false
            continue
        }

        if inBlock {
            commandBuffer = append(commandBuffer, line)
            continue
        }

        resultLines = append(resultLines, line)
    }

    return strings.Join(resultLines, "\n")
}

// detectAndExecuteNetworkOps finds network/cURL requests in model output.
func detectAndExecuteNetworkOps(response string, toolResults *[]string, inferenceURL string, maxTokens int) string {
    // Network operations are handled within shell command blocks (curl, wget, etc.)
    // This function is a placeholder for future structured tool call parsing
    return response
}

// executeShellCommandWithPermission runs a shell command after getting user consent.
func executeShellCommandWithPermission(command string, inferenceURL string, maxTokens int) string {
    // Execute with user permission
    result, err := shell.Exec(command, "Model requested shell execution", false)
    if err != nil {
        return fmt.Sprintf("Error executing command: %v", err)
    }

    output := ""
    if result.Stdout != "" {
        output += fmt.Sprintf("STDOUT:\n%s\n", result.Stdout)
    }
    if result.Stderr != "" {
        output += fmt.Sprintf("STDERR:\n%s\n", result.Stderr)
    }
    if result.ExitCode != 0 {
        output += fmt.Sprintf("Exit code: %d\n", result.ExitCode)
    }

    if output == "" {
        output = "Command completed successfully (no output)."
    }

    return fmt.Sprintf("The command '%s' was executed. Result:\n%s", command, output)
}

// chatCompletion sends a request to the llama-server API and returns the response.
func chatCompletion(inferenceURL string, messages []Message, maxTokens int) (string, error) {
    apiMessages := make([]map[string]string, len(messages))
    for i, msg := range messages {
        apiMessages[i] = map[string]string{
            "role":    msg.Role,
            "content": msg.Content,
        }
    }

    body := map[string]interface{}{
        "messages":    apiMessages,
        "max_tokens":  maxTokens,
        "temperature": 0.7,
        "stream":      false,
    }

    jsonBody, err := json.Marshal(body)
    if err != nil {
        return "", fmt.Errorf("marshaling request: %w", err)
    }

    apiURL := inferenceURL + "/v1/chat/completions"
    resp, err := http.Post(apiURL, "application/json", strings.NewReader(string(jsonBody)))
    if err != nil {
        return "", fmt.Errorf("API request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        respBody, _ := io.ReadAll(resp.Body)
        return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
    }

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return "", fmt.Errorf("decoding response: %w", err)
    }

    if len(result.Choices) == 0 {
        return "", fmt.Errorf("no choices in response")
    }

    return result.Choices[0].Message.Content, nil
}

func printHelp() {
    fmt.Println("\n📖 Commands:")
    fmt.Println("  /exit, /quit             - Exit the chat")
    fmt.Println("  /help                    - Show this help")
    fmt.Println("  /clear                   - Clear conversation history")
    fmt.Println("  /allow <command>         - Pre-approve a shell command pattern")
    fmt.Println("  /deny <command>          - Pre-deny a shell command pattern")
    fmt.Println("  /perms                   - Show current session permissions")
    fmt.Println()
    fmt.Println("💡 Permission model:")
    fmt.Println("  When the model wants to run a shell command, you'll be prompted:")
    fmt.Println("    y    - Allow this one time")
    fmt.Println("    a/!  - Always allow this session (pre-approve)")
    fmt.Println("    N    - Deny")
    fmt.Println()
}

func handleCommand(input string, messages *[]Message) {
    parts := strings.SplitN(input, " ", 2)
    cmd := parts[0]
    arg := ""
    if len(parts) > 1 {
        arg = parts[1]
    }

    switch cmd {
    case "/allow":
        if arg == "" {
            fmt.Println("Usage: /allow <command-pattern>")
            fmt.Println("Example: /allow ls")
            fmt.Println("This pre-approves any command starting with 'ls' for this session.")
            return
        }
        permissions.DefaultSessionTracker.AddAllow(permissions.ActionShell, arg)
        fmt.Printf("✅ Pre-approved shell command pattern: %s\n", arg)
        fmt.Println("   Any command starting with this pattern will be auto-approved.")

    case "/deny":
        if arg == "" {
            fmt.Println("Usage: /deny <command-pattern>")
            fmt.Println("Example: /deny rm")
            fmt.Println("This denies any command starting with 'rm' for this session.")
            return
        }
        permissions.DefaultSessionTracker.AddDeny(permissions.ActionShell, arg)
        fmt.Printf("❌ Pre-denied shell command pattern: %s\n", arg)
        fmt.Println("   Any command starting with this pattern will be auto-denied.")

    case "/perms":
        printPermissions()

    case "/clear":
        if messages != nil {
            *messages = []Message{}
        }
        permissions.DefaultSessionTracker.Clear()
        fmt.Println("🧹 Conversation and permissions cleared.")

    case "/history":
        if messages != nil {
            fmt.Printf("📝 %d messages in conversation history.\n", len(*messages))
        } else {
            fmt.Println("📝 No conversation history.")
        }

    default:
        fmt.Printf("Unknown command: %s. Type /help for available commands.\n", cmd)
    }
}

func printPermissions() {
    entries := permissions.DefaultSessionTracker.List()
    if len(entries) == 0 {
        fmt.Println("📋 No session-level permissions set.")
        fmt.Println("   Use '/allow <pattern>' to pre-approve commands.")
        fmt.Println("   Use '/deny <pattern>' to pre-deny commands.")
        return
    }
    fmt.Println("📋 Session permissions:")
    for _, e := range entries {
        fmt.Printf("  %s\n", e)
    }
}

func truncate(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen-3] + "..."
}
