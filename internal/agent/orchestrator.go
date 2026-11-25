package agent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"felix-go/internal/config"
	"felix-go/internal/tools"
)

// Orchestrator define la interfaz del orquestador del ciclo ReAct.
type Orchestrator interface {
	Run(ctx context.Context, sessionID, userInput, extraSystemContext string) (string, error)
}

// ReActOrchestrator coordina el ciclo ReAct conectando el ensamblador de contexto, el proveedor LLM,
// el despachador de herramientas y la compuerta de ejecucion.
type ReActOrchestrator struct {
	cfg        *config.Config
	registry   *tools.Registry
	history    *HistoryManager
	provider   LLMProvider
	assembler  ContextAssembler
	dispatcher ToolDispatcher
	onSuccess  func(sessionID string)
}

// NewReActOrchestrator crea una nueva instancia de ReActOrchestrator con todos sus componentes compartimentados.
func NewReActOrchestrator(
	cfg *config.Config,
	reg *tools.Registry,
	hist *HistoryManager,
	provider LLMProvider,
	assembler ContextAssembler,
	dispatcher ToolDispatcher,
	onSuccess func(sessionID string),
) *ReActOrchestrator {
	return &ReActOrchestrator{
		cfg:        cfg,
		registry:   reg,
		history:    hist,
		provider:   provider,
		assembler:  assembler,
		dispatcher: dispatcher,
		onSuccess:  onSuccess,
	}
}

// Run orquesta el ciclo de razonamiento y ejecucion de herramientas paso a paso.
func (o *ReActOrchestrator) Run(ctx context.Context, sessionID, userInput, extraSystemContext string) (string, error) {
	// 1. Ensamblado determinista del contexto
	conversation, activeGoal, err := o.assembler.Assemble(ctx, sessionID, userInput, extraSystemContext)
	if err != nil {
		return "", fmt.Errorf("error ensamblando contexto: %w", err)
	}

	isAdmin := true
	if val, ok := ctx.Value("is_admin").(bool); ok {
		isAdmin = val
	}

	// 2. Tool Gating Dinamico: Seleccionar exclusivamente las herramientas pertinentes
	schemas := o.registry.SelectTools(userInput, activeGoal, isAdmin)
	forceToolsNextIter := false
	isActionReq := IsActionRequest(userInput)
	toolsExecutedCount := 0
	modifyingToolsExecuted := 0
	lastToolHadError := false
	lastToolErrorSample := ""
	var toolTrace []string

	maxIters := o.cfg.MaxIterations
	if maxIters <= 0 {
		maxIters = 10
	}

	for iter := 0; iter < maxIters; iter++ {
		var currentTools []tools.OpenAIToolSchema
		var toolChoice string

		// Control preventivo de tiempo de turno restante
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 15*time.Second {
			log.Printf("[Orchestrator] Tiempo de turno restante critico (%v < 15s). Forzando sintesis final directa.", time.Until(deadline))
			currentTools = nil
			toolChoice = "none"
			conversation = append(conversation, OpenAIMessage{
				Role:    "user",
				Content: "[AVISO DE SISTEMA]: El tiempo disponible para este turno esta por finalizar. No invoques mas herramientas; redacta inmediatamente la respuesta final completa y concisa.",
			})
		} else if iter >= maxIters-2 {
			// En las ultimas dos iteraciones, deshabilitamos herramientas para forzar redaccion final
			currentTools = nil
			toolChoice = "none"
			conversation = append(conversation, OpenAIMessage{
				Role:    "user",
				Content: "[AVISO DE SISTEMA]: Ya dispones de toda la informacion. No invoques mas herramientas; redacta de inmediato la respuesta final clara y completa para el usuario.",
			})
		} else if forceToolsNextIter && len(schemas) > 0 {
			currentTools = schemas
			toolChoice = "required"
			forceToolsNextIter = false
		} else {
			currentTools = schemas
			toolChoice = "auto"
		}

		reqBody := ChatCompletionRequest{
			Model:      o.cfg.Model,
			Messages:   conversation,
			Tools:      currentTools,
			ToolChoice: toolChoice,
		}

		// Otorgar presupuesto de tiempo garantizado (Grace Period) para la inferencia
		execCtx := ctx
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 30*time.Second {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), 45*time.Second)
			defer cancel()
		}

		chatResp, err := o.provider.Execute(execCtx, reqBody)
		if err != nil {
			// Degradacion elegante: Si se ejecutaron herramientas durante el turno,
			// rescatar sus salidas tecnicas en lugar de abortar abruptamente.
			if fallbackSummary := summarizeExecutedTools(conversation); fallbackSummary != "" {
				log.Printf("[Orchestrator Degradacion Elegante] Fallo llamada LLM (%v), pero existen herramientas ejecutadas con exito. Retornando reporte tecnico.", err)
				o.persistTurn(sessionID, userInput, fallbackSummary, toolTrace)
				return fallbackSummary, nil
			}
			return "", fmt.Errorf("error llamando al LLM: %w", err)
		}

		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("respuesta sin opciones desde proveedor LLM")
		}

		choice := chatResp.Choices[0]
		assistantMsg := choice.Message
		conversation = append(conversation, assistantMsg)

		// 3. Si el modelo solicita ejecutar herramientas
		if len(assistantMsg.ToolCalls) > 0 {
			outputs, err := o.dispatcher.Dispatch(ctx, sessionID, isAdmin, assistantMsg.ToolCalls)
			if err != nil {
				return "", fmt.Errorf("error despachando herramientas: %w", err)
			}

			turnHadError := false
			turnErrorSample := ""
			for i, tc := range assistantMsg.ToolCalls {
				toolsExecutedCount++
				switch tc.Function.Name {
				case "exec", "file_write", "file_edit", "apply_patch", "harness_develop", "git_tool", "docker", "homelab", "infra_healer", "bot_factory":
					modifyingToolsExecuted++
				}

				output := outputs[i]
				if HasToolErrorOutput(output) {
					turnHadError = true
					sample := strings.TrimSpace(output)
					if len(sample) > 400 {
						sample = sample[:400] + "..."
					}
					turnErrorSample = sample
				}
				toolTrace = append(toolTrace, formatToolTraceEntry(tc, output))
				if len(output) > 3000 {
					output = output[:3000] + "\n... [salida truncada en memoria de trabajo]"
				}
				conversation = append(conversation, OpenAIMessage{
					Role:       "tool",
					Content:    output,
					ToolCallID: tc.ID,
				})
			}
			lastToolHadError = turnHadError
			if turnHadError {
				lastToolErrorSample = turnErrorSample
			} else {
				lastToolErrorSample = ""
			}
			continue
		}

		// 4. Si el modelo devuelve texto final
		finalReply := strings.TrimSpace(assistantMsg.Content)
		if finalReply != "" {
			claimsDone := IsClaimingActionCompleted(finalReply)
			hesitating := IsHesitatingOrAskingConfirmation(finalReply)
			delegating := IsDelegatingShellOrPlaceholder(finalReply)

			// Compuerta de Ejecucion y Evidencia Real (Grounding Gate):
			// Impide que el modelo alucine exito sin herramientas, oculte un error real de tool,
			// devuelva bloques bash crudos para que los corra el usuario o pida confirmacion redundante.
			if iter < 4 && len(schemas) > 0 {
				if isActionReq && (toolsExecutedCount == 0 || hesitating || delegating) {
					log.Printf("[Execution Gate] Interceptada evasion/delegacion en iter %d (tools=%d, hesitating=%v, delegating=%v). Forzando ejecucion...", iter, toolsExecutedCount, hesitating, delegating)
					conversation = append(conversation, OpenAIMessage{
						Role:    "user",
						Content: fmt.Sprintf("[DIRECTIVA DEL SISTEMA - EJECUCION OBLIGATORIA]: El usuario solicito una accion o diagnostico ('%s'). No devuelvas comandos para que los ejecute el usuario ni respuestas vacias o evasivas. Invoca tus herramientas reales ahora.", userInput),
					})
					forceToolsNextIter = true
					continue
				}
				if claimsDone && modifyingToolsExecuted == 0 {
					log.Printf("[Grounding Gate] Bloqueada afirmacion sin herramientas modificadoras en iter %d.", iter)
					conversation = append(conversation, OpenAIMessage{
						Role:    "user",
						Content: "[DIRECTIVA CRITICA - PROHIBIDO ALUCINAR]: Afirmaste haber completado o verificado cambios sin ejecutar herramientas de modificacion en este turno. Ejecuta las herramientas reales y verifica el resultado antes de responder.",
					})
					forceToolsNextIter = true
					continue
				}
				if claimsDone && lastToolHadError {
					log.Printf("[Grounding Gate] Bloqueada afirmacion de exito tras error real de herramienta en iter %d.", iter)
					conversation = append(conversation, OpenAIMessage{
						Role:    "user",
						Content: fmt.Sprintf("[DIRECTIVA CRITICA - ERROR EN HERRAMIENTA]: Tu ultima ejecucion devolvio un error real (%s). Queda PROHIBIDO afirmar al usuario que la operacion funciono sin errores. Corrige el fallo o reporta con exactitud tecnica el error observado.", lastToolErrorSample),
					})
					continue
				}
			}

			finalReply = cleanFormatting(finalReply)
			o.persistTurn(sessionID, userInput, finalReply, toolTrace)
			return finalReply, nil
		}
	}

	// 5. Sintesis forzada de cierre si se agotaron iteraciones
	synthPrompt := "Resume y responde de inmediato al usuario con toda la informacion recopilada hasta este punto sin invocar mas herramientas."
	if isActionReq && (modifyingToolsExecuted == 0 || lastToolHadError) {
		synthPrompt = "Se alcanzo el limite de iteraciones. Reporta con precision tecnica que se ejecuto y que fallo o quedo pendiente, SIN afirmar falsamente que se completaron los cambios."
	}
	synthConv := append(conversation, OpenAIMessage{
		Role:    "user",
		Content: synthPrompt,
	})
	synthReq := ChatCompletionRequest{
		Model:      o.cfg.Model,
		Messages:   synthConv,
		ToolChoice: "none",
	}

	synthCtx, synthCancel := context.WithTimeout(context.WithoutCancel(ctx), 35*time.Second)
	defer synthCancel()

	if chatResp, err := o.provider.Execute(synthCtx, synthReq); err == nil && len(chatResp.Choices) > 0 {
		reply := cleanFormatting(chatResp.Choices[0].Message.Content)
		if reply != "" {
			o.persistTurn(sessionID, userInput, reply, toolTrace)
			return reply, nil
		}
	}

	if fallbackSummary := summarizeExecutedTools(conversation); fallbackSummary != "" {
		fallbackSummary = cleanFormatting(fallbackSummary)
		o.persistTurn(sessionID, userInput, fallbackSummary, toolTrace)
		return fallbackSummary, nil
	}

	return "No se pudo completar la sintesis final dentro del limite de iteraciones.", nil
}

// summarizeExecutedTools extrae y consolida las salidas tecnicas de herramientas en caso de falla de sintesis final.
func summarizeExecutedTools(messages []OpenAIMessage) string {
	var relevantOutputs []string
	for _, m := range messages {
		if m.Role != "tool" {
			continue
		}
		raw := strings.TrimSpace(m.Content)
		if raw == "" || raw == "OK" || raw == "(comando ejecutado sin salida)" || raw == "true" {
			continue
		}
		// Truncar salidas extensas para no inundar el canal ni envenenar el historial
		if len(raw) > 250 {
			raw = raw[:250] + "... [salida truncada]"
		}
		relevantOutputs = append(relevantOutputs, raw)
	}

	if len(relevantOutputs) == 0 {
		hasTools := false
		for _, m := range messages {
			if m.Role == "tool" {
				hasTools = true
				break
			}
		}
		if hasTools {
			return "Operaciones tecnicas ejecutadas en el sistema. Aguardando siguiente instruccion."
		}
		return ""
	}

	var b strings.Builder
	b.WriteString("Operaciones tecnicas ejecutadas:\n\n")
	start := 0
	if len(relevantOutputs) > 3 {
		start = len(relevantOutputs) - 3
	}
	for i := start; i < len(relevantOutputs); i++ {
		b.WriteString(relevantOutputs[i])
		if i < len(relevantOutputs)-1 {
			b.WriteString("\n---\n")
		}
	}
	return b.String()
}

func (o *ReActOrchestrator) persistTurn(sessionID, userInput, assistantReply string, toolTrace []string) {
	if o.history != nil {
		storedReply := assistantReply
		if len(toolTrace) > 0 {
			storedReply = fmt.Sprintf("%s\n\n[Herramientas ejecutadas en este turno:\n%s]", assistantReply, strings.Join(toolTrace, "\n"))
		}
		o.history.Append(sessionID, OpenAIMessage{Role: "user", Content: userInput})
		o.history.Append(sessionID, OpenAIMessage{Role: "assistant", Content: storedReply})
	}
	if o.onSuccess != nil {
		o.onSuccess(sessionID)
	}
}

func formatToolTraceEntry(tc ToolCall, output string) string {
	args := strings.Join(strings.Fields(SanitizeSecrets(tc.Function.Arguments)), " ")
	if len(args) > 120 {
		args = args[:120] + "..."
	}
	status := "OK"
	if HasToolErrorOutput(output) {
		errSample := strings.Join(strings.Fields(SanitizeSecrets(output)), " ")
		if len(errSample) > 120 {
			errSample = errSample[:120] + "..."
		}
		status = "ERROR: " + errSample
	}
	return fmt.Sprintf("- %s(%s) -> %s", tc.Function.Name, args, status)
}
