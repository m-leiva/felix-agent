package agent

import (
	"strings"
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
		"Para habilitar el sandbox en dicho proyecto, se requiere ejecutar:\n```bash\ncd /home/matias/proyecto-go\ngit init\n```",
		"El diagnóstico y los ajustes quedaron definidos. Confirmame si procedo con la inicialización del repo en proyecto-go",
		"Vos no tenés que hacer nada. ¿Avanzo con la ejecución ahora?",
		"Resultados de la preparación. ¿Disparo la corrida completa ahora?",
		"¿Querés que use claude o felix?",
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

func TestIsActionOrExecutionRequest(t *testing.T) {
	actionCases := []string{
		"en la parte del index sacá la parte de competencias",
		"sacalo y poné Desarrollador de software",
		"no veo que hayas subido documentación en pdf",
		"no veo ninguno de esos cambios",
		"hacé todo",
		"actualizá el bio",
		"corregí el error en la función",
		"reemplazá la descripción",
	}
	for _, tc := range actionCases {
		if !IsActionOrExecutionRequest(tc) {
			t.Errorf("se esperaba que IsActionOrExecutionRequest fuera true para %q", tc)
		}
	}
}

func TestIsClaimingActionCompleted(t *testing.T) {
	claimingCases := []string{
		"Se han aplicado y verificado todas las modificaciones en el servidor de producción",
		"Se ha actualizado la configuración y el contenido de mleiva.com",
		"Los documentos PDF técnicos oficiales han sido generados y colocados en /public/downloads/",
		"Ya apliqué los cambios solicitados",
		"He generado los cuatro documentos técnicos oficiales",
		"El perfil quedó actualizado",
	}
	for _, tc := range claimingCases {
		if !IsClaimingActionCompleted(tc) {
			t.Errorf("se esperaba que IsClaimingActionCompleted fuera true para %q", tc)
		}
	}

	nonClaimingCases := []string{
		"No se encontraron archivos en la ruta",
		"El comando arrojó el siguiente error",
		"Para realizar esto se requieren herramientas",
	}
	for _, tc := range nonClaimingCases {
		if IsClaimingActionCompleted(tc) {
			t.Errorf("se esperaba que IsClaimingActionCompleted fuera false para %q", tc)
		}
	}
}

func TestSanitizeSecrets(t *testing.T) {
	input := `DATABASE_URL="mysql://root:secretPass123@127.0.0.1:3306/portfolio_db"
JWT_SECRET="super-secret-key"
GEMINI_API_KEY="AQ.dummyTestKeyForSanitizerVerification1234567890"
GROQ_KEY="gsk_dummyTestSecretKeyForTestingPurposeOnly12345"`

	sanitized := SanitizeSecrets(input)
	if strings.Contains(sanitized, "secretPass123") {
		t.Errorf("la contrasena de la base de datos no fue redactada")
	}
	if strings.Contains(sanitized, "super-secret-key") {
		t.Errorf("el JWT_SECRET no fue redactado")
	}
	if strings.Contains(sanitized, "AQ.dummyTestKeyForSanitizerVerification1234567890") {
		t.Errorf("la GEMINI_API_KEY no fue redactada")
	}
	if strings.Contains(sanitized, "gsk_dummyTestSecretKeyForTestingPurposeOnly12345") {
		t.Errorf("la GROQ_KEY no fue redactada")
	}
}
