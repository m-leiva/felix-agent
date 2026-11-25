package agent

import (
	"context"
	"fmt"

	"felix-go/internal/config"
	"felix-go/internal/tools"
)

// ContextAssembler define el contrato para la construccion determinista del contexto conversacional.
type ContextAssembler interface {
	Assemble(ctx context.Context, sessionID, userInput, extraSystemContext string) ([]OpenAIMessage, string, error)
}

// DefaultContextAssembler implementa ContextAssembler coordinando prompts, memoria RAG, resumenes y scratchpad.
type DefaultContextAssembler struct {
	cfg        *config.Config
	history    *HistoryManager
	scratchpad *tools.ScratchpadManager
}

// NewContextAssembler crea un nuevo ensamblador de contexto.
func NewContextAssembler(cfg *config.Config, hist *HistoryManager, sm *tools.ScratchpadManager) *DefaultContextAssembler {
	return &DefaultContextAssembler{
		cfg:        cfg,
		history:    hist,
		scratchpad: sm,
	}
}

// SetScratchpadManager asigna el gestor de memoria de trabajo activa.
func (ca *DefaultContextAssembler) SetScratchpadManager(sm *tools.ScratchpadManager) {
	ca.scratchpad = sm
}

// Assemble construye la lista ordenada de mensajes para enviar al LLM.
// Consolida todas las instrucciones de sistema (prompt, resumen y scratchpad) en un unico mensaje
// inicial para preservar la secuencia valida user/assistant y compatibilidad con templates estrictos.
func (ca *DefaultContextAssembler) Assemble(ctx context.Context, sessionID, userInput, extraSystemContext string) ([]OpenAIMessage, string, error) {
	systemPrompt := ca.cfg.SystemPrompt
	if extraSystemContext != "" {
		systemPrompt = systemPrompt + "\n\n" + extraSystemContext
	}

	if ca.history != nil {
		if summary, _, _ := ca.history.GetSummary(sessionID); summary != "" {
			systemPrompt += "\n\n[RESUMEN CONSOLIDADO DE SESIONES PREVIAS]:\n" + summary
		}
	}

	activeGoal := ""
	if ca.scratchpad != nil {
		if sp, err := ca.scratchpad.Get(sessionID); err == nil && sp != nil && sp.Goal != "" {
			activeGoal = sp.Goal
			spBlock := fmt.Sprintf("### MEMORIA DE TRABAJO ACTIVA (SCRATCHPAD):\n- Objetivo en curso: %s\n- Estado: %s (Actualizado: %s)",
				sp.Goal, sp.Status, sp.UpdatedAt.Format("15:04:05"))
			if len(sp.Variables) > 0 {
				spBlock += "\n- Variables tecnicas:"
				for k, v := range sp.Variables {
					spBlock += fmt.Sprintf(" %s=%s;", k, v)
				}
			}
			systemPrompt += "\n\n" + spBlock
		}
	}

	var conversation []OpenAIMessage
	conversation = append(conversation, OpenAIMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	if ca.history != nil {
		existingHistory := ca.history.Get(sessionID)
		conversation = append(conversation, existingHistory...)
	}

	conversation = append(conversation, OpenAIMessage{
		Role:    "user",
		Content: userInput,
	})

	return conversation, activeGoal, nil
}
