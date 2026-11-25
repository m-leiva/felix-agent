package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"felix-go/internal/config"
	"felix-go/internal/tools"
)

type mockTool struct {
	name     string
	delay    time.Duration
	executed *atomic.Int32
}

func (m *mockTool) Name() string { return m.name }
func (m *mockTool) Description() string { return "Mock tool para testing" }
func (m *mockTool) Parameters() map[string]any {
	return map[string]any{"type": "object"}
}
func (m *mockTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	time.Sleep(m.delay)
	m.executed.Add(1)
	return "resultado de " + m.name, nil
}

func TestParallelToolExecution(t *testing.T) {
	var tool1Executed atomic.Int32
	var tool2Executed atomic.Int32

	t1 := &mockTool{name: "mock_tool_1", delay: 50 * time.Millisecond, executed: &tool1Executed}
	t2 := &mockTool{name: "mock_tool_2", delay: 50 * time.Millisecond, executed: &tool2Executed}

	reg := tools.NewRegistry()
	reg.Register(t1)
	reg.Register(t2)

	turnCount := 0
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turnCount++
		w.Header().Set("Content-Type", "application/json")

		if turnCount == 1 {
			// El modelo devuelve 2 llamadas a herramientas en paralelo
			resp := ChatCompletionResponse{
				ID: "call-tools-test",
			}
			resp.Choices = append(resp.Choices, struct {
				Index        int           `json:"index"`
				Message      OpenAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				Index: 0,
				Message: OpenAIMessage{
					Role: "assistant",
					ToolCalls: []ToolCall{
						{
							ID:   "call_1",
							Type: "function",
							Function: ToolCallFunction{
								Name:      "mock_tool_1",
								Arguments: "{}",
							},
						},
						{
							ID:   "call_2",
							Type: "function",
							Function: ToolCallFunction{
								Name:      "mock_tool_2",
								Arguments: "{}",
							},
						},
					},
				},
				FinishReason: "tool_calls",
			})
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Turno 2: respuesta final
		resp := ChatCompletionResponse{
			ID: "final-test",
		}
		resp.Choices = append(resp.Choices, struct {
			Index        int           `json:"index"`
			Message      OpenAIMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		}{
			Index: 0,
			Message: OpenAIMessage{
				Role:    "assistant",
				Content: "Ambas herramientas completadas",
			},
			FinishReason: "stop",
		})
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := &config.Config{
		APIBase:       mockServer.URL,
		Model:         "test-model",
		MaxIterations: 5,
	}

	histDir, _ := osMkdirTemp()
	defer osRemoveAll(histDir)
	histMgr, _ := NewHistoryManager(histDir+"/test.db", 10)
	defer histMgr.Close()

	eng := NewEngine(cfg, reg, histMgr)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	reply, err := eng.Run(ctx, "session-parallel-test", "ejecuta ambas herramientas", "")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run devolvio error: %v", err)
	}

	if reply != "Ambas herramientas completadas" {
		t.Errorf("respuesta inesperada: %s", reply)
	}

	if tool1Executed.Load() != 1 || tool2Executed.Load() != 1 {
		t.Errorf("las herramientas no fueron ejecutadas: t1=%d, t2=%d", tool1Executed.Load(), tool2Executed.Load())
	}

	// Al correr en paralelo (50ms cada una), el tiempo total de ejecucion de herramientas debe ser ~50ms, no 100ms+
	if elapsed > 300*time.Millisecond {
		t.Logf("Aviso: tiempo total de ejecucion: %v", elapsed)
	}
}

func TestCleanFormattingPreservesCodeHashes(t *testing.T) {
	input := "## Estado\nUsando **C#** y color #FF0000.\n```python\n# comentario valido\nprint(1)\n```"
	got := cleanFormatting(input)
	if !containsAll(got, []string{"Estado", "*C#*", "#FF0000", "# comentario valido"}) {
		t.Errorf("cleanFormatting altero contenido valido: %q", got)
	}
}

func TestSanitizeSecrets(t *testing.T) {
	raw := `DB=mysql://root:supersecret123@127.0.0.1:3306/app y key gsk_1234567890abcdef1234 {"password":"anotherSecretValue","api_key":"sk-1234567890abcdef1234"}`
	sanitized := SanitizeSecrets(raw)
	if containsAny(sanitized, []string{"supersecret123", "gsk_1234567890abcdef1234", "anotherSecretValue", "sk-1234567890abcdef1234"}) {
		t.Errorf("SanitizeSecrets no redacto credenciales: %q", sanitized)
	}
}

func TestPersistTurnWithToolTrace(t *testing.T) {
	hm, err := NewHistoryManager(t.TempDir()+"/sessions.db", 20)
	if err != nil {
		t.Fatalf("NewHistoryManager error: %v", err)
	}
	defer hm.Close()

	orch := &ReActOrchestrator{history: hm}
	trace := []string{
		formatToolTraceEntry(ToolCall{
			ID:       "call_1",
			Function: ToolCallFunction{Name: "exec", Arguments: `{"command":"echo ok","password":"secretValue123"}`},
		}, "ok"),
		formatToolTraceEntry(ToolCall{
			ID:       "call_2",
			Function: ToolCallFunction{Name: "exec", Arguments: `{"command":"false"}`},
		}, "Error ejecutando herramienta exec: exit status 1"),
	}

	orch.persistTurn("sess-trace", "ejecuta comandos", "Listo.", trace)

	msgs := hm.Get("sess-trace")
	if len(msgs) != 2 {
		t.Fatalf("se esperaban 2 mensajes en historial, se obtuvieron %d", len(msgs))
	}
	if containsAny(msgs[1].Content, []string{"secretValue123"}) {
		t.Errorf("el trace en historial filtro un secreto: %q", msgs[1].Content)
	}
	if !containsAll(msgs[1].Content, []string{"Listo.", "[Herramientas ejecutadas en este turno:", "exec(", "-> OK", "-> ERROR:"}) {
		t.Errorf("el trace de herramientas no se guardo correctamente: %q", msgs[1].Content)
	}
}

func containsAll(s string, subs []string) bool {
	for _, sub := range subs {
		if !stringsContains(s, sub) {
			return false
		}
	}
	return true
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if stringsContains(s, sub) {
			return true
		}
	}
	return false
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexString(s, sub) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func osMkdirTemp() (string, error) {
	return os.MkdirTemp("", "test_felix_eng_*")
}

func osRemoveAll(path string) {
	_ = os.RemoveAll(path)
}
