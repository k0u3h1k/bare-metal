package console

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RunInteractive starts an interactive terminal chat session with a model.
// This is a TUI placeholder — will be replaced with Bubble Tea or Charm
// for a richer terminal experience.
func RunInteractive(modelName string) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n━━━ Unbound Interactive Chat — %s ━━━\n", modelName)
	fmt.Println("Type '/help' for commands, '/exit' to quit.")
	fmt.Println("Type '/allow ls' to give shell permission, or reply normally to chat.\n")

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
			// TODO: send to model inference and display response
			fmt.Printf("🤖 %s > (placeholder — model not yet connected)\n", modelName)
			fmt.Printf("   You said: %s\n", input)
			fmt.Println("   [Model inference coming soon with llama.cpp bindings]")
		}
	}
}

func printHelp() {
	fmt.Println("\n📖 Commands:")
	fmt.Println("  /exit, /quit    - Exit the chat")
	fmt.Println("  /help           - Show this help")
	fmt.Println("  /allow <cmd>    - Pre-approve a shell command pattern")
	fmt.Println("  /deny <cmd>     - Deny a shell command pattern")
	fmt.Println("  /clear          - Clear chat history")
	fmt.Println("  /history        - Show recent chat history")
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
