package agent

import (
	"regexp"
	"strings"
)

var affirmativePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*(ok|okay|okey|dale|hacelo|mandale|metele|ejecuta|ejecutalo|procede|adelante|si|confirmo|confirmado|avanza|claro|por supuesto|hace todo|hacelo de una vez|dame los resultados|avanzar|iniciar)\b`),
	regexp.MustCompile(`(?i)\b(hacelo|ejecuta todo|hace todo|mandale|procede|adelante|confirmado|hacelo de una vez|dame los resultados de una vez)\b`),
}

var executionCommandVerbs = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(ejecuta|ejecutar|ejecutalo|proba|probar|crea|crear|hace|hacer|hacelo|compila|compilar|instala|instalar|corre|correr|testea|testear|actualiza|actualizar|inicializa|inicializar|inicia|iniciar|borra|borrar|elimina|eliminar|reinicia|reiniciar|saca|sacar|sacalo|pone|poner|ponelo|cambia|cambiar|cambialo|reemplaza|reemplazar|reemplazalo|quita|quitar|quitalo|subi|subir|agrega|agregar|corregi|corregir|arregla|arreglar|desplega|desplegar)\b`),
}

var actionRequestPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(no veo (ninguno|los|nada|que|esos)|no cambio nada|sigue igual|no veo los cambios|no veo absolutamente nada|falta|hace todo|hacelo todo|entonces\?|y lo demas|y el resto)\b`),
	regexp.MustCompile(`(?i)\b(sacalo y pone|sacala y pone|cambialo por|reemplazalo por|pone en|saca de|saca la parte|en la parte de)\b`),
}

var claimingActionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(se ha(n)? (aplicado|actualizado|generado|creado|modificado|eliminado|corregido|subido|guardado|instalado|configurado|compilado|vinculado|procesado))\b`),
	regexp.MustCompile(`(?i)\b(han sido (generados|actualizados|creados|modificados|eliminados|subidos|guardados|instalados|compilados|vinculados))\b`),
	regexp.MustCompile(`(?i)\b(ya (aplique|actualice|genere|cree|modifique|elimine|corregi|subi|guarde|instale|compile|quedo))\b`),
	regexp.MustCompile(`(?i)\b(he (aplicado|actualizado|generado|creado|modificado|eliminado|corregido|subido|guardado|instalado|compilado))\b`),
	regexp.MustCompile(`(?i)\b(quedo actualizado|quedo modificado|se encuentra actualizado|se encuentran actualizados|fueron aplicados|fueron actualizados)\b`),
	regexp.MustCompile(`(?i)\b(aplicadas y verificadas|aplicados y verificados|modificaciones en el servidor|operaciones tecnicas ejecutadas)\b`),
}

var hesitationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)¿\s*(procedo|avanzo|lo hago|queres que|desea que|confirmo|ejecuto|hago|disparo|arranco|inicio)\b`),
	regexp.MustCompile(`(?i)\b(confirmame si|confirmame para|avisame si|avisame para|aguardo tu confirmacion|aguardo confirmacion|espero tu confirmacion|si estas de acuerdo|cuando me des el visto bueno|cuando me confirmes|indicame si procedo|decime si avanzo|decime si procedo)\b`),
	regexp.MustCompile(`(?i)se requiere ejecutar\s*:?\s*` + "```"),
}

func normalizeLower(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "á", "a")
	s = strings.ReplaceAll(s, "é", "e")
	s = strings.ReplaceAll(s, "í", "i")
	s = strings.ReplaceAll(s, "ó", "o")
	s = strings.ReplaceAll(s, "ú", "u")
	return s
}

func IsAffirmativeOrExecutionCommand(userInput string) bool {
	norm := normalizeLower(strings.TrimSpace(userInput))
	if norm == "" {
		return false
	}

	for _, p := range affirmativePatterns {
		if p.MatchString(norm) {
			return true
		}
	}

	for _, p := range executionCommandVerbs {
		if p.MatchString(norm) {
			return true
		}
	}

	return false
}

func IsActionOrExecutionRequest(userInput string) bool {
	norm := normalizeLower(strings.TrimSpace(userInput))
	if norm == "" {
		return false
	}

	for _, p := range actionRequestPatterns {
		if p.MatchString(norm) {
			return true
		}
	}

	for _, p := range executionCommandVerbs {
		if p.MatchString(norm) {
			return true
		}
	}

	return false
}

func IsClaimingActionCompleted(reply string) bool {
	norm := normalizeLower(strings.TrimSpace(reply))
	if norm == "" {
		return false
	}

	for _, p := range claimingActionPatterns {
		if p.MatchString(norm) {
			return true
		}
	}

	return false
}

func IsHesitatingOrAskingConfirmation(reply string) bool {
	norm := normalizeLower(strings.TrimSpace(reply))
	if norm == "" {
		return false
	}

	for _, p := range hesitationPatterns {
		if p.MatchString(norm) {
			return true
		}
	}

	return false
}
