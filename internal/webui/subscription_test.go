package webui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Lichas/maxclaw/internal/agent"
	"github.com/Lichas/maxclaw/internal/bus"
	"github.com/Lichas/maxclaw/internal/config"
	"github.com/Lichas/maxclaw/internal/providers"
	"github.com/Lichas/maxclaw/pkg/tools"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	program := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if os.Getenv("MAXCLAW_FAKE_SUBSCRIPTION") == "1" && (program == "codex" || program == "claude") {
		os.Exit(fakeSubscriptionCLI(program, os.Args[1:]))
	}
	os.Exit(m.Run())
}

func fakeSubscriptionCLI(program string, args []string) int {
	if os.Getenv("OPENAI_API_KEY") != "" || os.Getenv("ANTHROPIC_API_KEY") != "" {
		return 2
	}
	if args[0] == "login" || args[0] == "auth" {
		if program == "codex" {
			if os.Getenv("MAXCLAW_FAKE_API_AUTH") == "1" {
				fmt.Println("Logged in using an API key")
			} else {
				fmt.Println("Logged in using ChatGPT")
			}
		} else {
			method := "claude.ai"
			if os.Getenv("MAXCLAW_FAKE_API_AUTH") == "1" {
				method = "api_key"
			}
			fmt.Printf("{\"loggedIn\":true,\"authMethod\":%q}\n", method)
		}
		return 0
	}
	prompt, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 2
	}
	reply := []byte(`{"text":"UNEXPECTED_TEST_PROMPT","tool_calls":[]}`)
	if bytes.Contains(prompt, []byte("SUBSCRIPTION_FILE_CONTENT")) {
		reply = []byte(`{"text":"SUBSCRIPTION_PONG","tool_calls":[]}`)
	}
	if bytes.Contains(prompt, []byte("read subscription fixture")) && !bytes.Contains(prompt, []byte("SUBSCRIPTION_FILE_CONTENT")) {
		arguments, _ := json.Marshal(map[string]string{"path": os.Getenv("MAXCLAW_FAKE_FILE")})
		reply, _ = json.Marshal(map[string]any{
			"text":       "",
			"tool_calls": []map[string]string{{"name": "read_file", "arguments_json": string(arguments)}},
		})
	}
	if program == "claude" {
		fmt.Printf("{\"is_error\":false,\"structured_output\":%s}\n", reply)
		return 0
	}
	for i, arg := range args {
		if arg == "--output-last-message" && i+1 < len(args) && os.WriteFile(args[i+1], reply, 0600) == nil {
			return 0
		}
	}
	return 2
}

func TestSubscriptionGatewayStatusConfigAndToolRoundTrip(t *testing.T) {
	binDir := t.TempDir()
	testExe, err := os.Executable()
	require.NoError(t, err)
	content, err := os.ReadFile(testExe)
	require.NoError(t, err)
	for _, binary := range []string{"codex", "claude"} {
		if runtime.GOOS == "windows" {
			binary += ".exe"
		}
		require.NoError(t, os.WriteFile(filepath.Join(binDir, binary), content, 0700))
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MAXCLAW_FAKE_SUBSCRIPTION", "1")
	t.Setenv("OPENAI_API_KEY", "test-api-key-not-for-subscriptions")
	t.Setenv("ANTHROPIC_API_KEY", "test-api-key-not-for-subscriptions")

	for _, route := range []string{providers.CodexCLI, providers.ClaudeCode} {
		t.Run(route, func(t *testing.T) {
			configHome := t.TempDir()
			t.Setenv("MAXCLAW_HOME", configHome)
			cfg := config.DefaultConfig()
			require.NoError(t, os.MkdirAll(cfg.Agents.Defaults.Workspace, 0700))
			file := filepath.Join(cfg.Agents.Defaults.Workspace, "fixture.txt")
			require.NoError(t, os.WriteFile(file, []byte("SUBSCRIPTION_FILE_CONTENT"), 0600))
			t.Setenv("MAXCLAW_FAKE_FILE", file)
			require.NoError(t, config.SaveConfig(cfg))

			loop := agent.NewAgentLoop(bus.NewMessageBus(10), nil, cfg.Agents.Defaults.Workspace,
				cfg.Agents.Defaults.Model, 3, "", tools.WebFetchOptions{}, cfg.Tools.Exec, true, nil, nil, false)
			defer loop.Close()
			server := &Server{cfg: cfg, agentLoop: loop}
			request := func(handler http.HandlerFunc, method, path string, body any) *httptest.ResponseRecorder {
				data, err := json.Marshal(body)
				require.NoError(t, err)
				recorder := httptest.NewRecorder()
				handler(recorder, httptest.NewRequest(method, path, bytes.NewReader(data)))
				return recorder
			}

			t.Setenv("MAXCLAW_FAKE_API_AUTH", "1")
			probe := map[string]string{"apiFormat": route}
			require.Equal(t, http.StatusBadRequest, request(server.handleTestProvider, http.MethodPost, "/api/providers/test", probe).Code)
			t.Setenv("MAXCLAW_FAKE_API_AUTH", "")
			require.Equal(t, http.StatusOK, request(server.handleTestProvider, http.MethodPost, "/api/providers/test", probe).Code)

			model := route + "/default"
			cfg.Agents.Defaults.Model = model
			update := map[string]any{
				"agents": cfg.Agents,
				"providers": map[string]config.ProviderConfig{
					route:    {APIFormat: route, Models: []config.ProviderModelConfig{{ID: model, Enabled: true}}},
					"openai": {APIKey: "another-configured-api-key"},
				},
			}
			save := request(server.handleConfig, http.MethodPut, "/api/config", update)
			require.Equal(t, http.StatusOK, save.Code, save.Body.String())
			reloaded, err := config.LoadConfig()
			require.NoError(t, err)
			require.Equal(t, model, reloaded.Agents.Defaults.Model)
			require.Empty(t, reloaded.GetAPIKey(model))
			require.False(t, reloaded.SupportsImageInput(model))

			post := request(server.handleMessage, http.MethodPost, "/api/message", map[string]string{
				"sessionKey": "webui:subscription", "content": "read subscription fixture",
			})
			require.Equal(t, http.StatusOK, post.Code, post.Body.String())
			require.Contains(t, post.Body.String(), "SUBSCRIPTION_PONG")
			cfg.Agents.Defaults.Model = ""
			require.Equal(t, http.StatusOK, request(server.handleConfig, http.MethodPut, "/api/config", map[string]any{
				"agents": cfg.Agents, "providers": map[string]any{},
			}).Code)
			post = request(server.handleMessage, http.MethodPost, "/api/message", map[string]string{
				"sessionKey": "webui:removed-subscription", "content": "try the removed subscription",
			})
			require.Contains(t, post.Body.String(), "select a model")
		})
	}
}
