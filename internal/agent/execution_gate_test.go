package agent

import (
	"testing"
)

func TestIsAffirmativeOrExecutionCommand(t *testing.T) {
	affirmativeCases := []string{
		"Ok, mandale",
		"Okey, hacelo",
		"ok",
		"dale",
		"hacelo",
		"ejecutá todo",
		"ejecuta",
		"si",
		"sí, dale",
		"confirmo",
		"adelante",
		"hacelo de una vez",
		"OK, DAME LOS RESULTADOS DE UNA VEZ",
		"probá la suite",
		"creá el archivo",
		"inicializá el repo",
		"compilá el proyecto",
	}

	for _, tc := range affirmativeCases {
		if !IsAffirmativeOrExecutionCommand(tc) {
			t.Errorf("se esperaba que IsAffirmativeOrExecutionCommand fuera true para %q", tc)
		}
	}

	negativeCases := []string{
		"¿Cómo estás?",
		"¿Qué herramientas tenés activas?",
		"¿Cuántos tokens tiene tu prompt?",
		"Explicame la diferencia entre Go y Rust",
		"No, no lo hagas",
		"cancelar",
	}

	for _, tc := range negativeCases {
		if IsAffirmativeOrExecutionCommand(tc) {
			t.Errorf("se esperaba que IsAffirmativeOrExecutionCommand fuera false para %q", tc)
		}
	}
}

func TestIsHesitatingOrAskingConfirmation(t *testing.T) {
	hesitantCases := []string{
		"Para habilitar el sandbox en dicho proyecto, se requiere ejecutar:\n```bash\ncd /home/matias/mosiah-go\ngit init\n```",
		"El diagnóstico y los ajustes quedaron definidos. Confirmame si procedo con la inicialización del repo en mosiah-go",
		"Vos no tenés que hacer nada. ¿Avanzo con la ejecución ahora?",
		"Resultados de la preparación. ¿Disparo la corrida completa ahora?",
		"¿Querés que use mosiah o felix?",
		"Aguardo tu confirmación para proceder con el despliegue.",
		"Cuando me confirmes inicio las pruebas.",
	}

	for _, tc := range hesitantCases {
		if !IsHesitatingOrAskingConfirmation(tc) {
			t.Errorf("se esperaba que IsHesitatingOrAskingConfirmation fuera true para %q", tc)
		}
	}

	nonHesitantCases := []string{
		"Resultados de la suite de pruebas: PASS. 3 pruebas ejecutadas en 0.006s.",
		"El servicio felix-go está activo y operando en el puerto 8055.",
		"Archivo creado exitosamente en /home/matias/test.go.",
		"No se encontraron errores en la compilación.",
	}

	for _, tc := range nonHesitantCases {
		if !IsHesitatingOrAskingConfirmation(tc) {
			continue
		}
		t.Errorf("se esperaba que IsHesitatingOrAskingConfirmation fuera false para %q", tc)
	}
}

func TestIsDelegatingShellOrPlaceholder(t *testing.T) {
	delegating := []string{
		"task ready",
		"```bash\npm2 status || systemctl list-units\n```",
		"```\nls -la\n```",
		"",
	}
	for _, tc := range delegating {
		if !IsDelegatingShellOrPlaceholder(tc) {
			t.Errorf("se esperaba true para %q", tc)
		}
	}

	valid := []string{
		"El servicio web esta activo en el puerto 8080.",
		"Ejecute el comando y el resultado fue:\n```bash\nactive (running)\n```\nTodo operativo.",
	}
	for _, tc := range valid {
		if IsDelegatingShellOrPlaceholder(tc) {
			t.Errorf("se esperaba false para %q", tc)
		}
	}
}

func TestHasToolErrorOutput(t *testing.T) {
	errOutputs := []string{
		"Error ejecutando herramienta exec: exit status 1",
		"Error durante sincronizacion: Timeout 30000ms exceeded",
		"Traceback (most recent call last):\n  File \"app.py\"",
	}
	for _, out := range errOutputs {
		if !HasToolErrorOutput(out) {
			t.Errorf("se esperaba deteccion de error en %q", out)
		}
	}

	okOutputs := []string{
		"Sincronizacion completada. Nuevos registros agregados: 14",
		"OK",
	}
	for _, out := range okOutputs {
		if HasToolErrorOutput(out) {
			t.Errorf("no se esperaba error en %q", out)
		}
	}
}
