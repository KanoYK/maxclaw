package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	CodexCLI   = "codex-cli"
	ClaudeCode = "claude-code"
)

var subscriptionModelID = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$")

// SubscriptionRoute requires an explicit prefix so API models never use CLI billing.
func SubscriptionRoute(model string) string {
	route, id, ok := strings.Cut(strings.TrimSpace(model), "/")
	if ok && id != "" && (route == CodexCLI || route == ClaudeCode) {
		return route
	}
	return ""
}

type SubscriptionProvider struct {
	model string
}

func (p *SubscriptionProvider) GetDefaultModel() string        { return p.model }
func (p *SubscriptionProvider) SupportsImageInput(string) bool { return false }

type subscriptionCLI struct {
	route string
	path  string
}

func findSubscriptionCLI(route string) (subscriptionCLI, error) {
	binary := "codex"
	if route == ClaudeCode {
		binary = "claude"
	}
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	path, err := exec.LookPath(binary)
	if err != nil {
		return subscriptionCLI{}, fmt.Errorf("%s not found on PATH. Install the official native CLI and restart MaxClaw", binary)
	}
	return subscriptionCLI{route: route, path: path}, nil
}

func (c subscriptionCLI) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, c.path, args...)
	hideSubscriptionConsole(cmd)
	// Preserve the CLI's own login store and OS environment. Explicit credentials
	// and custom provider settings can route a CLI request to usage-based billing.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "OPENAI_API_KEY", "OPENAI_BASE_URL", "CODEX_API_KEY", "CODEX_ACCESS_TOKEN",
			"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "ANTHROPIC_BASE_URL", "CLAUDE_CODE_OAUTH_TOKEN",
			"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY":
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

const codexAccountSetting = "forced_login_method=\"chatgpt\""

func (c subscriptionCLI) checkLogin(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if c.route == CodexCLI {
		status, err := c.command(ctx, "login", "-c", codexAccountSetting, "status").CombinedOutput()
		if err == nil && strings.Contains(strings.ToLower(string(status)), "logged in using chatgpt") {
			return nil
		}
		return fmt.Errorf("ChatGPT sign-in is required. Run codex login and choose ChatGPT")
	}

	status, err := c.command(ctx, "auth", "status", "--json").Output()
	var account struct {
		LoggedIn   bool   `json:"loggedIn"`
		AuthMethod string `json:"authMethod"`
	}
	if err == nil && json.Unmarshal(status, &account) == nil && account.LoggedIn && account.AuthMethod == "claude.ai" {
		return nil
	}
	return fmt.Errorf("Claude account sign-in is required. Run claude auth login and choose your Claude subscription")
}

// CheckSubscriptionLogin checks the vendor login without returning any account data.
func CheckSubscriptionLogin(ctx context.Context, route string) error {
	if route != CodexCLI && route != ClaudeCode {
		return fmt.Errorf("unknown subscription provider")
	}
	cli, err := findSubscriptionCLI(route)
	if err != nil {
		return err
	}
	return cli.checkLogin(ctx)
}

const subscriptionSchema = `{
  "type":"object",
  "properties":{
    "text":{"type":"string"},
    "tool_calls":{"type":"array","items":{
      "type":"object",
      "properties":{"name":{"type":"string"},"arguments_json":{"type":"string"}},
      "required":["name","arguments_json"],
      "additionalProperties":false
    }}
  },
  "required":["text","tool_calls"],
  "additionalProperties":false
}`

const subscriptionInstructions = `You are providing one turn of a MaxClaw conversation.
The JSON input contains the conversation and the MaxClaw tool definitions. Follow the system messages, then respond to the conversation.
To ask MaxClaw to run a listed tool, return its exact name and encode its argument object as the arguments_json string. MaxClaw will run the tools and send back their results.
Return text with an empty tool_calls array when no tool is needed. Never claim a tool ran without its result. Do not use tools internal to the coding CLI.`

func (p *SubscriptionProvider) Chat(ctx context.Context, messages []Message, tools []map[string]interface{}, model string) (*Response, error) {
	if model == "" {
		model = p.model
	}
	route := SubscriptionRoute(model)
	if route == "" {
		return nil, fmt.Errorf("select a Codex or Claude Code subscription model")
	}
	cli, err := findSubscriptionCLI(route)
	if err != nil {
		return nil, err
	}
	if err := cli.checkLogin(ctx); err != nil {
		return nil, err
	}

	// Only send text and tool history. Attachment paths and URLs are not needed
	// by a text-only provider.
	conversation := append([]Message(nil), messages...)
	for i := range conversation {
		for _, part := range conversation[i].Parts {
			if part.Type == "text" && conversation[i].Content == "" {
				conversation[i].Content = part.Text
			}
		}
		conversation[i].Parts = nil
	}
	input, err := json.Marshal(map[string]any{"messages": conversation, "tools": tools})
	if err != nil {
		return nil, err
	}
	_, selectedModel, _ := strings.Cut(model, "/")
	if !subscriptionModelID.MatchString(selectedModel) {
		return nil, fmt.Errorf("invalid subscription model ID")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	output, err := cli.runTurn(ctx, selectedModel, subscriptionInstructions+"\n\n"+string(input))
	if err != nil {
		return nil, err
	}
	return parseSubscriptionResponse(output, tools)
}

func (c subscriptionCLI) runTurn(ctx context.Context, model, prompt string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "maxclaw-subscription-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	var args []string
	answer := filepath.Join(dir, "answer.json")
	if c.route == CodexCLI {
		schema := filepath.Join(dir, "schema.json")
		if err := os.WriteFile(schema, []byte(subscriptionSchema), 0600); err != nil {
			return nil, err
		}
		args = []string{"-a", "never", "exec", "--sandbox", "read-only", "--ephemeral", "--skip-git-repo-check",
			"--ignore-user-config", "--ignore-rules", "-c", codexAccountSetting, "-c", "model_provider=\"openai\"",
			"-c", "web_search=\"disabled\"", "-c", "mcp_servers={}", "-c", "include_apply_patch_tool=false",
			"--disable", "shell_tool", "--disable", "apps", "--disable", "plugins", "--disable", "browser_use",
			"--disable", "computer_use", "--disable", "multi_agent", "--disable", "code_mode_host",
			"--output-schema", schema, "--output-last-message", answer}
		if runtime.GOOS == "windows" {
			args = append(args, "-c", "windows.sandbox=\"unelevated\"")
		}
	} else {
		args = []string{"-p", "--safe-mode", "--no-session-persistence", "--tools", "", "--no-chrome",
			"--strict-mcp-config", "--permission-mode", "dontAsk", "--output-format", "json", "--json-schema", subscriptionSchema}
	}
	if model != "default" {
		args = append(args, "--model", model)
	}
	if c.route == CodexCLI {
		args = append(args, "-")
	}
	cmd := c.command(ctx, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s request failed. Check the official CLI login, model availability, and subscription limits", c.route)
	}
	if c.route == CodexCLI {
		return os.ReadFile(answer)
	}
	var result struct {
		IsError bool            `json:"is_error"`
		Output  json.RawMessage `json:"structured_output"`
	}
	if err := json.Unmarshal(output, &result); err != nil || result.IsError || len(result.Output) == 0 {
		return nil, fmt.Errorf("Claude Code did not return a structured reply")
	}
	return result.Output, nil
}

func parseSubscriptionResponse(output []byte, tools []map[string]interface{}) (*Response, error) {
	var turn struct {
		Text      string `json:"text"`
		ToolCalls []struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments_json"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal(output, &turn); err != nil {
		return nil, fmt.Errorf("subscription CLI returned an invalid structured reply")
	}
	allowed := make(map[string]bool, len(tools))
	for _, tool := range tools {
		function, _ := tool["function"].(map[string]interface{})
		if name, ok := function["name"].(string); ok {
			allowed[name] = true
		}
	}
	reply := &Response{Content: turn.Text}
	for _, call := range turn.ToolCalls {
		var args map[string]any
		if !allowed[call.Name] || json.Unmarshal([]byte(call.Arguments), &args) != nil || args == nil {
			return nil, fmt.Errorf("subscription CLI proposed an unavailable tool or invalid arguments")
		}
		reply.ToolCalls = append(reply.ToolCalls, ToolCall{
			ID: uuid.NewString(), Type: "function",
			Function: ToolCallFunction{Name: call.Name, Arguments: call.Arguments},
		})
	}
	reply.HasToolCalls = len(reply.ToolCalls) > 0
	if !reply.HasToolCalls && strings.TrimSpace(reply.Content) == "" {
		return nil, fmt.Errorf("subscription CLI returned an empty reply")
	}
	return reply, nil
}

func (p *SubscriptionProvider) ChatStream(ctx context.Context, messages []Message, tools []map[string]interface{}, model string, handler StreamHandler) error {
	reply, err := p.Chat(ctx, messages, tools, model)
	if handler == nil {
		return err
	}
	if err != nil {
		handler.OnError(err)
		return err
	}
	if reply.Content != "" {
		handler.OnContent(reply.Content)
	}
	for _, call := range reply.ToolCalls {
		handler.OnToolCallStart(call.ID, call.Function.Name)
		handler.OnToolCallDelta(call.ID, call.Function.Arguments)
		handler.OnToolCallEnd(call.ID)
	}
	handler.OnComplete()
	return nil
}
