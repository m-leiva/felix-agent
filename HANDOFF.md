# Handoff y Panorama Tecnico del Proyecto Felix

Fecha de actualizacion: 2026-11-25

---

## 1. Panorama de Proyectos y Repositorios

El ecosistema de Felix se divide en dos directorios principales:

1. `/home/matias/felix-open-source` (Repositorio Git / Uso Universal)
   - Remoto: `https://github.com/m-leiva/felix-agent.git` (`origin`)
   - Rama activa: `pre-botfactory` (tambien existe `main`)
   - Proposito: Version open-source portable para cualquier distribucion Linux, sin rutas fijas, credenciales ni acoplamientos a un entorno particular.
   - Compilacion: `go build -o bin/felix ./cmd/felix` (tiene multiples archivos en `cmd/felix/`).
   - Estado actual: Tiene mejoras de estabilidad y universalidad aplicadas en el arbol de trabajo listas para commit/push (`git status -s`).

2. `/home/matias/felix-server` (Instancia de Produccion / PM2)
   - Proceso PM2: `felix-server` (ID `10`, ejecuta `./bin/felix server`).
   - Proposito: Instancia activa en produccion. Incluye modulos adicionales especificos no presentes en `felix-open-source` (`internal/botfactory`, `internal/cli`, `internal/tools/bot_tool.go`, `internal/tools/wikimed_tool.go`).
   - Compilacion: `go build -o bin/felix cmd/felix/main.go`
   - Configuracion local: `config.json` (mantiene el `system_prompt` y credenciales especificas del despliegue).
   - Datos y logs: `data/*.db` y `/home/matias/.pm2/logs/felix-server-*.log`.

---

## 2. Arquitectura Tecnica Actual

- **Omnicanal (`internal/channel/`, `internal/whatsapp/`)**: Transporte desacoplado para WhatsApp (WhatsMeow) y Telegram (Bot API), con pipeline multimodal (`internal/channel/multimodal.go`) para audio (Whisper), imagenes y documentos.
- **Motor Agente ReAct (`internal/agent/`)**:
  - `context_assembler.go`: Construye el historial de mensajes (`Assemble`).
  - `orchestrator.go`: Bucle ReAct, control de iteraciones, degradacion elegante y compuertas de ejecucion.
  - `execution_gate.go`: Reglas deterministicas anti-alucinacion y deteccion de evasion/errores.
  - `provider.go`: Failover automatico primario -> respaldo (ej. Groq/OpenAI) con circuit breaker, cooldown de 30s, sonda de recuperacion y rotacion de `KeyPool` ante HTTP 429.
  - `compactor.go`: Compactacion asincrona de historial en `session_summaries`.
  - `history.go`: Persistencia SQLite WAL en `sessions.db`.
  - `subagent.go`: Orquestacion de subagentes concurrentes (limite 3 en paralelo) con reportes en `workspace/reports/`.
- **Motor de Grafos y Time-Travel (`internal/graph/`, `internal/checkpoint/`)**: Grafos de estado inmutables con checkpoints SQLite WAL (`checkpoints.db`) y reanudacion ante fallos.
- **Arnes de Desarrollo (`internal/harness/`)**: Repomap AST (`repomap/`), parches fuzzy con Levenshtein (`patcher/`), sandbox Git (`sandbox/`) y ejecutor de tests (`runner/`).
- **Fast-Path (`internal/fastpath/`)**: Enrutador determinista de latencia minima para saludos, hora/fecha (`time.Local`), telemetria, memoria, disco, IP y servicios sin consumir tokens LLM.
- **Catalogo de Herramientas (`internal/tools/`)**: `gating.go` entrega todas las herramientas al administrador y filtra por categorias (`core`, `infra`, `code_files`, `web`, `media`, `admin`) para otros usuarios.
- **Auto-curacion y Salud (`internal/healer/`, `internal/health/`)**: Diagnostico automatico de puertos en uso / procesos caidos y servidor HTTP en `:8055`.

---

## 3. Auditoria y Correcciones Recientes (2026-11-25)

A partir de fallos observados en sesiones reales y logs de ejecucion, se corrigieron los siguientes problemas:

1. **Error HTTP 400 en Failover (`No user query found in messages`)**:
   - *Causa*: `ContextAssembler.Assemble` inyectaba el bloque `SCRATCHPAD` como un mensaje `Role: "system"` despues del mensaje `user`, rompiendo templates Jinja estrictos. Ademas `pruneRequestForFallback` podia cortar la ventana dejando un mensaje `tool` huerfano o sin mensaje `user`.
   - *Solucion*: `Assemble` consolida `SystemPrompt`, `extraSystemContext`, resumen y `SCRATCHPAD` en un unico mensaje `conv[0]` (`Role: "system"`), asegurando que la lista termine en `user`. `pruneRequestForFallback` preserva pares `assistant(tool_calls) -> tool` y garantiza presencia de un mensaje `user`.

2. **Alucinacion post-ejecucion y evasion de herramientas**:
   - *Causa*: El agente afirmaba exito cuando `exec` habia devuelto error/timeout, o respondia con bloques `bash` crudos y placeholders (`"task ready"`) sin ejecutar herramientas.
   - *Solucion*: En `execution_gate.go` y `orchestrator.go` se agrego deteccion objetiva de errores en salida de herramientas (`HasToolErrorOutput`), deteccion de bloques shell delegados y placeholders (`IsDelegatingShellOrPlaceholder`), y bloqueo en `iter < 4` si el modelo afirma exito cuando `modifyingToolsExecuted == 0` o `lastToolHadError == true`. Tambien se reemplazo el mensaje falso de cierre por un reporte explicito cuando se agota el limite de iteraciones.

3. **Destruccion de `#` en codigo dentro de `cleanFormatting`**:
   - *Causa*: `strings.ReplaceAll(text, "#", "")` eliminaba comentarios Python/Bash, `C#` y colores hexadecimales.
   - *Solucion*: En `engine.go`, `cleanFormatting` ahora elimina encabezados Markdown (`^#{1,6}\s+`) solo fuera de bloques de codigo ```` ``` ```` y aplica `SanitizeSecrets`.

4. **Universalidad**:
   - *Solucion*: Se removieron rutas y contenedores hardcodeados de `DefaultConfig().SystemPrompt` en `config.go`, se neutralizo el prompt de `compactor.go` y se cambio `fastpath.go` para usar `time.Local` en lugar de una zona horaria fija.

---

## 4. Estado de Pendientes y Proximos Pasos

1. **Publicar cambios en `felix-open-source`** (Completado):
   - Commiteado y pusheado en rama `pre-botfactory` (commit `716975b`) tras ejecutar `go test -count=1 ./...` y `go build -o bin/felix ./cmd/felix`.
2. **Trazabilidad de herramientas en `sessions.db`** (Completado):
   - `orchestrator.go` (`persistTurn` y `formatToolTraceEntry`) adjunta un resumen compacto y sanitizado de las herramientas ejecutadas y errores al turno guardado en `sessions.db`, sin alterar la respuesta limpia enviada al usuario.
3. **Sanitizacion de credenciales en logs y dispatcher** (Completado):
   - `tool_dispatcher.go` aplica `SanitizeSecrets` antes de loguear `[Tool Call]` y `[Tool Call Paralelo]`. `secretLineRegex` en `engine.go` fue ampliado para cubrir pares clave/valor en formato JSON.
4. **Prueba en vivo del failover** (Completado):
   - Validado en `felix-server` contra `https://api.groq.com/openai/v1` (`qwen/qwen3.8-27b`) mediante `go test -tags=live -v ./internal/agent -run TestLiveFailoverWithToolCalls`: completo un turno completo con `tool_calls` y respuesta `tool` sin errores HTTP 400.
5. **Estrategia de sincronizacion** (Definicion pendiente del usuario):
   - Recomendacion: unificar `felix-server` como rama downstream (`server-prod`) sobre `felix-open-source` (o aislar `botfactory`, `cli`, `bot_tool.go` y `wikimed_tool.go` detras de build tags `-tags server`), manteniendo `config.json` y `data/` fuera de Git.

---

## 5. Comandos de Verificacion y Operacion

```bash
# Ejecutar suite completa de tests
go test -count=1 ./...

# Compilar binario (felix-open-source)
go build -o bin/felix ./cmd/felix
```
