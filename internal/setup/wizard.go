package setup

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/DevAnimecx/deltacode/internal/config"
	"github.com/DevAnimecx/deltacode/internal/provider"
	"github.com/DevAnimecx/deltacode/pkg/models"
)

func RunWizard(cfg *config.Manager) {
	reader := bufio.NewReader(os.Stdin)

	ClearScreen()
	printLogo()
	fmt.Println()
	printCentered("Welcome to Δ Delta Code")
	printCentered("The Self-Evolving BYOK Coding Agent")
	fmt.Println()
	printLine()

	fmt.Println()
	fmt.Println("  This one-time setup will:")
	fmt.Println("   • Configure your AI provider")
	fmt.Println("   • Set your API key")
	fmt.Println("   • Test the connection")
	fmt.Println()
	fmt.Print("  Press Enter to start...")
	reader.ReadString('\n')

	// Step 1: Choose provider
	ClearScreen()
	printLogo()
	fmt.Println()
	printCentered("Step 1/3: Choose Your AI Provider")
	fmt.Println()
	printLine()
	fmt.Println()
	fmt.Println("  Supported providers:")
	providerList := []struct {
		num int
		name string
		desc string
		url string
		models string
	}{
		{1, "OpenAI", "GPT-4o, GPT-4o-mini", "https://api.openai.com/v1", "gpt-4o,gpt-4o-mini,gpt-4-turbo"},
		{2, "Anthropic", "Claude Sonnet 4, Claude Haiku 3.5", "https://api.anthropic.com/v1", "claude-sonnet-4-20250514,claude-haiku-3-5-20241022"},
		{3, "Google Gemini", "Gemini 2.0 Flash, Gemini 1.5 Pro", "https://generativelanguage.googleapis.com/v1beta", "gemini-2.0-flash,gemini-1.5-pro"},
		{4, "DeepSeek", "DeepSeek Chat, DeepSeek Coder", "https://api.deepseek.com/v1", "deepseek-chat,deepseek-coder"},
		{5, "Ollama (Local)", "Run models locally (free)", "http://localhost:11434", ""},
		{6, "Custom / OpenRouter / Other", "Any OpenAI-compatible endpoint", "", ""},
	}
	for _, p := range providerList {
		fmt.Printf("  %d. %s\n", p.num, p.name)
		fmt.Printf("     %s\n", p.desc)
	}

	fmt.Println()
	fmt.Print("  Select provider [1-6]: ")
	choiceStr, _ := reader.ReadString('\n')
	choiceStr = strings.TrimSpace(choiceStr)

	var selected struct {
		name   string
		pType  models.ProviderType
		url    string
		models string
	}
	idx := 0
	for _, c := range choiceStr {
		if c >= '0' && c <= '9' {
			idx = idx*10 + int(c-'0')
		}
	}
	if idx < 1 || idx > 6 {
		idx = 1
	}
	sel := providerList[idx-1]
	selected.name = sel.name
	selected.models = sel.models
	selected.url = sel.url
	switch idx {
	case 1:
		selected.pType = models.ProviderOpenAI
	case 2:
		selected.pType = models.ProviderAnthropic
	case 3:
		selected.pType = models.ProviderGoogle
	case 4:
		selected.pType = models.ProviderDeepSeek
	case 5:
		selected.pType = models.ProviderOllama
	case 6:
		selected.pType = models.ProviderCustom
	}

	// Step 2: API Key
	ClearScreen()
	printLogo()
	fmt.Println()
	printCentered(fmt.Sprintf("Step 2/3: Enter Your %s API Key", selected.name))
	fmt.Println()
	printLine()
	fmt.Println()

	var apiKey string
	if selected.pType == models.ProviderOllama {
		fmt.Println("  ✓ Ollama runs locally — no API key needed!")
		fmt.Println("  Make sure Ollama is running at http://localhost:11434")
		fmt.Println()
	} else if selected.pType == models.ProviderCustom {
		fmt.Print("  Provider name: ")
		nameInput, _ := reader.ReadString('\n')
		selected.name = strings.TrimSpace(nameInput)
		fmt.Print("  Base URL: ")
		urlInput, _ := reader.ReadString('\n')
		selected.url = strings.TrimSpace(urlInput)
		fmt.Print("  API Key: ")
		apiKeyInput, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKeyInput)
	} else {
		fmt.Printf("  Get your API key from: https://platform.openai.com/api-keys\n")
		if strings.Contains(selected.name, "Anthropic") {
			fmt.Printf("  Get your API key from: https://console.anthropic.com/keys\n")
		}
		if strings.Contains(selected.name, "Gemini") {
			fmt.Printf("  Get your API key from: https://aistudio.google.com/apikey\n")
		}
		if strings.Contains(selected.name, "DeepSeek") {
			fmt.Printf("  Get your API key from: https://platform.deepseek.com/api_keys\n")
		}
		fmt.Println()
		fmt.Print("  Paste your API key and press Enter: ")
		apiKeyInput, _ := reader.ReadString('\n')
		apiKey = strings.TrimSpace(apiKeyInput)
	}

	// Step 3: Test connection
	ClearScreen()
	printLogo()
	fmt.Println()
	printCentered("Step 3/3: Testing Connection")
	fmt.Println()
	printLine()

	provCfg := models.ProviderConfig{
		Name:    strings.ToLower(selected.name),
		Type:    selected.pType,
		BaseURL: selected.url,
		APIKey:  apiKey,
		Models:  strings.Split(selected.models, ","),
	}

	fmt.Println()
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < 20; i++ {
		fmt.Printf("\r  %s Connecting to %s...", spinner[i%len(spinner)], selected.name)
		time.Sleep(80 * time.Millisecond)
	}

	p, err := provider.NewProvider(provCfg)
	if err != nil {
		fmt.Printf("\r  ✗ Connection failed: %v\n", err)
		fmt.Println()
		fmt.Print("  Press Enter to retry or Ctrl+C to quit...")
		reader.ReadString('\n')
		RunWizard(cfg)
		return
	}

	models, err := p.ListModels()
	if err != nil {
		fmt.Printf("\r  ⚠ Connected but couldn't list models: %v\n", err)
	} else {
		fmt.Printf("\r  ✓ Connected to %s — found %d models\n", selected.name, len(models))
	}

	// Save
	cfg.AddProvider(provCfg)
	cfg.SetDefault(provCfg.Name)
	if len(models) > 0 {
		cfg.SetDefault(provCfg.Name)
	}

	fmt.Println()
	printLine()
	fmt.Println()
	printCentered("🎉 Delta Code is ready!")
	fmt.Println()
	fmt.Println("  Try these commands:")
	fmt.Println()
	fmt.Println("    delta run \"write hello world in python\"")
	fmt.Println("    delta doctor")
	fmt.Println("    delta explain \"how does this project work\"")
	fmt.Println("    delta review")
	fmt.Println("    delta")
	fmt.Println()
	fmt.Print("  Press Enter to continue...")
	reader.ReadString('\n')
	ClearScreen()
	printLogo()
	fmt.Println()
	printCentered("Happy coding! Δ")
	fmt.Println()
}

func printLogo() {
	fmt.Println("      ██████╗ ███████╗██╗  ████████╗ █████╗")
	fmt.Println("      ██╔══██╗██╔════╝██║  ╚══██╔══╝██╔══██╗")
	fmt.Println("      ██║  ██║█████╗  ██║     ██║   ███████║")
	fmt.Println("      ██║  ██║██╔══╝  ██║     ██║   ██╔══██║")
	fmt.Println("      ██████╔╝███████╗███████╗██║   ██║  ██║")
	fmt.Println("      ╚═════╝ ╚══════╝╚══════╝╚═╝   ╚═╝  ╚═╝")
	fmt.Println()
}

func printCentered(text string) {
	w := getTerminalWidth()
	padding := (w - len(text)) / 2
	if padding < 0 {
		padding = 0
	}
	fmt.Println(strings.Repeat(" ", padding) + text)
}

func printLine() {
	w := getTerminalWidth()
	fmt.Println(strings.Repeat("─", w))
}

func ClearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func getTerminalWidth() int {
	cmd := exec.Command("mode", "con")
	out, err := cmd.Output()
	if err != nil {
		return 60
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Columns") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				w := 0
				for _, c := range parts[len(parts)-1] {
					if c >= '0' && c <= '9' {
						w = w*10 + int(c-'0')
					}
				}
				if w > 0 {
					return w
				}
			}
		}
	}
	return 60
}

func IsFirstRun(cfg *config.Manager) bool {
	providers := cfg.ListProviders()
	return len(providers) == 0
}
