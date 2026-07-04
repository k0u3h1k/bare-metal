package console

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Message represents a chat message in the conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RunInteractive starts an interactive terminal chat session.
// This is a simpler version that doesn't connect to an inference server.
func RunInteractive(modelName string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n━━━ Unbound Interactive Chat — %s ━━━\n", modelName)
	fmt.Println("Type '/help' for commands, '/exit' to quit.")
	fmt.Println()

	history := []string{}

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
			handleCommand(input)
		default:
			history = append(history, input)
			fmt.Printf("🤖 %s > (placeholder — model not connected)\n", modelName)
			fmt.Printf("   You said: %s\n", input)
		}
	}
}

// RunInteractiveWithInference starts an interactive chat with real inference
// via the llama-server API.
func RunInteractiveWithInference(modelName, inferenceURL, systemPrompt string, maxTokens int) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n━━━ Unbound Interactive Chat — %s ━━━\n", modelName)
	fmt.Println("Type '/help' for commands, '/exit' to quit.")
	fmt.Println()

	// Build conversation history
	var messages []Message
	if systemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: systemPrompt})
	}

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
		case input == "/clear":
			messages = []Message{}
			if systemPrompt != "" {
				messages = append(messages, Message{Role: "system", Content: systemPrompt})
			}
			fmt.Println("🧹 Conversation cleared.")
			continue
		case strings.HasPrefix(input, "/"):
			handleCommand(input)
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

		// Add assistant response to history
		messages = append(messages, Message{Role: "assistant", Content: response})
	}
}

// chatCompletion sends a request to the llama-server API and returns the response.
func chatCompletion(inferenceURL string, messages []Message, maxTokens int) (string, error) {
	// Convert messages to API format
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
	fmt.Println("  /exit, /quit    - Exit the chat")
	fmt.Println("  /help           - Show this help")
	fmt.Println("  /clear          - Clear conversation history")
	fmt.Println("  /history        - Show conversation history (TODO)")
	fmt.Println()
}

func handleCommand(input string) {
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch cmd {
	case "/allow":
		fmt.Printf("✅ Pre-approved command pattern: %s\n", arg)
	case "/deny":
		fmt.Printf("❌ Denied command pattern: %s\n", arg)
	case "/clear":
		fmt.Println("🧹 Chat history cleared.")
	case "/history":
		fmt.Println("📝 History display coming soon.")
	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", cmd)
	}
}
