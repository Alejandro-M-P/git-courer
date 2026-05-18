# git-courer v2 — Implementación detallada del nuevo flujo de commits

## Contexto del código actual

Antes de tocar nada, hay que entender qué existe ya y qué NO hay que romper.

### Lo que ya funciona y NO se toca

- `BgJob`, `BgRunning/Done/Failed`, `sync.Map` en `handlers.go` → el sistema de jobs en memoria ya existe
- `handlePreview`, `handleStatus`, `handleApply`, `handleAbort`, `handleRegenerate` en `handlers.go` → la estructura del handler ya existe
- `previewTimeout = 45 * time.Second` → el timer de 45s ya existe
- El select con `time.After(previewTimeout)` en `handlePreview` → ya existe
- El goroutine que sigue vivo cuando el MCP corta → ya existe
- `workflow.Run`, `workflow.Apply`, `workflow.Abort` → no se tocan
- `CommitService`, `executeSync`, `ExecuteFromPlan` → no se tocan (todavía, ver abajo)
- Los prompts de `commit_message.md` y `commit_message.txt` → se actualizan para incluir WHY

---

## Problema central que resuelve esta implementación

El flujo actual divide los staged en múltiples commits (chunker decide). El nuevo flujo hace siempre UN commit por llamada PREVIEW, usando git plumbing (`git write-tree` + `git commit-tree`) para capturar el estado del staging en el momento del PREVIEW y commitearlo más tarde en el APPLY, sin que el agente tenga que esperar bloqueado.

---

## Cambios por archivo

### 1. `internal/core/ports/git.go`

**Qué hacer:** Añadir tres métodos nuevos a la interfaz `Git`.

Los tres métodos van justo después de `Commit(message string) (string, error)` en la sección `--- Write ---`:

```go
// WriteTree captures the current staging area as an immutable tree object.
// Returns the tree hash (SHA1). Fails if staging is empty.
WriteTree() (string, error)

// CommitTree creates a commit object pointing to treeHash with parentHash as parent.
// Returns the new commit hash. parentHash is typically the current HEAD.
CommitTree(treeHash, parentHash, message string) (string, error)

// UpdateRef moves a ref (e.g. "HEAD") to point to commitHash.
UpdateRef(ref, commitHash string) (string, error)
```

**Por qué:** La interfaz define el contrato. El adapter implementa. El workflow llama por la interfaz. Sin esto nada compila.

---

### 2. `internal/adapters/git/exec_write_commit.go`

**Qué hacer:** Implementar los tres métodos nuevos en `ExecAdapter`.

Añadir al final del archivo, después de `RemoteRemove`:

**`WriteTree`**: llama a `a.runGit("write-tree")` y hace `strings.TrimSpace` del output. Si el output está vacío devuelve error `"nothing to commit, staging area is empty"`. Este error es el que el handler usa para fallar inmediatamente antes de crear el job.

**`CommitTree`**: llama a `a.runGit("commit-tree", treeHash, "-p", parentHash, "-m", message)` y hace `strings.TrimSpace` del output. El output es el hash del nuevo commit.

**`UpdateRef`**: llama a `a.runGit("update-ref", ref, commitHash)`. No hay output relevante, solo error o nil.

**Importante:** `runGit` ya existe en `exec.go` y maneja el workDir correctamente. No hay que hacer nada especial.

---

### 3. `internal/delivery/mcp/core/handlers.go`

Este es el archivo más importante. Hay tres cambios aquí.

#### 3a. Añadir `TreeHash` y `Done chan struct{}` al struct `BgJob`

El struct actual es:
```go
type BgJob struct {
    ID     string
    Status BgJobStatus
    Error  string
}
```

Añadir dos campos:
```go
type BgJob struct {
    ID       string
    Status   BgJobStatus
    Error    string
    TreeHash string      // capturado con git write-tree antes de lanzar el goroutine
    Done     chan struct{} // cerrado cuando el goroutine termina (éxito o fallo)
}
```

`Done` es un channel sin buffer. Se crea con `make(chan struct{})` cuando se crea el job. El goroutine lo cierra con `close(job.Done)` cuando termina, tanto en éxito como en fallo. El `handleApply` espera en `<-job.Done` si el job todavía está en `BgRunning`.

#### 3b. Modificar `handlePreview`

El `handlePreview` actual lanza el goroutine del workflow directamente. Hay que añadir ANTES de lanzar el goroutine:

1. Llamar a `h.git.WriteTree()` síncronamente.
2. Si devuelve error → return error inmediato, NO crear job. El staging está vacío.
3. Si va bien → guardar el `treeHash`.
4. Crear el job con ese `treeHash` y `Done: make(chan struct{})`.

El goroutine que ya existe no cambia — cuando termina (éxito o fallo) además de actualizar `Status` tiene que cerrar `job.Done` con `close(j.Done)`.

Actualmente el goroutine del slow path hace:
```go
go func() {
    res := <-ch
    bgJob, _ := h.bgJobs.Load(jobID)
    if bgJob != nil {
        j := bgJob.(*BgJob)
        if res.err != nil {
            j.Status = BgFailed
            j.Error = res.err.Error()
            return
        }
        j.Status = BgDone
    }
}()
```

Añadir `close(j.Done)` antes del `return` y al final del goroutine. En AMBAS ramas (failed y done).

También hay que añadir el `treeHash` al mensaje de respuesta del slow path para que el apply pueda encontrarlo. El job se guarda en `bgJobs` con el treeHash ya incluido, así que el apply lo lee de ahí.

#### 3c. Modificar `handleApply`

El `handleApply` actual llama a `h.reviewWorkflow.Apply(ctx)` que internamente llama a `s.git.Commit(msg)`. Hay que cambiar esto.

El nuevo flujo del `handleApply`:

1. Leer el job_id de los params (nuevo param `job_id` opcional).
2. Si hay job_id → buscar el job en `bgJobs`.
   - Si `Status == BgRunning` → esperar en `<-job.Done` (bloqueante, el agente decidió esperar).
   - Si `Status == BgFailed` → devolver error.
   - Si `Status == BgDone` → continuar con el treeHash del job.
3. Obtener el `treeHash` del job.
4. Llamar a `h.reviewWorkflow.Apply(ctx)` para obtener el mensaje generado por el LLM (el Apply existente ya hace esto correctamente, solo que luego llama a `git commit`).
5. En vez de que el workflow ejecute el `git commit`, el handler toma el control:
   - Obtener HEAD actual: `headHash, err := h.git.Log(1, "")` o mejor añadir un método `Head() (string, error)` al port que haga `git rev-parse HEAD`.
   - Llamar a `h.git.CommitTree(treeHash, headHash, message)` → obtiene `commitHash`.
   - Llamar a `h.git.UpdateRef("HEAD", commitHash)` → mueve HEAD.
   - Llamar a `h.git.Reset("HEAD", ".")` → limpia el staging. **Esta función ya existe en el port y en el adapter.**
6. Devolver resultado con el hash del commit y el mensaje.

**Problema:** El workflow.Apply actualmente ejecuta el commit internamente. Hay dos opciones:

**Opción A (más limpia):** Añadir un método `GenerateMessage(ctx) (string, error)` al workflow o al CommitService que solo genera el mensaje sin ejecutar el commit. El handler ejecuta el plumbing.

**Opción B (más rápida):** Añadir un flag `DryRun bool` al Apply que genera el mensaje pero no ejecuta el commit, y devuelve el mensaje en `Result.Output`. El handler lee el mensaje de ahí y ejecuta el plumbing.

Recomendación: **Opción A** porque es más limpio y no contamina el Apply con flags de dry run.

---

### 4. `internal/adapters/git/exec_write_commit.go` (añadir `Head`)

Añadir un método `Head() (string, error)` que ejecuta `git rev-parse HEAD` y devuelve el hash limpio. Añadirlo también al port en `ports/git.go`.

---

### 5. `internal/workflow/commit.go` o nuevo archivo

**Si se elige la Opción A:** Añadir un método `GenerateCommitMessage(ctx context.Context, instruction string) (string, error)` al `CommitService`. Este método hace exactamente lo que hace `Execute` pero en vez de llamar a `s.git.Commit(msg)` al final, devuelve el mensaje.

Internamente llama a `s.prepareStages(instruction)` para obtener los chunks, luego llama al LLM para generar el mensaje, pero no ejecuta nada de git. Devuelve el mensaje como string.

Este método es el que el workflow.Apply usará en modo "preview only", o el que el handler llamará directamente si se pasa el `treeHash`.

---

### 6. `internal/delivery/mcp/core/registration.go`

**Qué hacer:** Actualizar la descripción del tool `commit` y añadir `job_id` al APPLY.

La descripción actual del tool `commit`:
```
"3-phase commit pipeline: PREVIEW parses AST and groups files by dependency graph into atomic commits, APPLY executes them..."
```

Nueva descripción:
```
"Async commit pipeline using local LLM. PREVIEW captures the current staging area as an immutable snapshot and generates a commit message — always ONE commit per PREVIEW. If LLM finishes under 45s, returns message + job_id directly. If it exceeds 45s, returns job_id immediately and continues in background.

APPLY executes the commit using the captured snapshot — files appear unstaged in worktree after APPLY, this is EXPECTED, the commit is in the log. Do NOT re-stage or re-commit. Do NOT use git bash as a fallback while a job_id is active.

You CAN and SHOULD continue working on unrelated tasks between PREVIEW and APPLY."
```

Añadir el parámetro `job_id` al tool para que el APPLY pueda recibirlo:
```go
mcpgo.WithString("job_id", mcpgo.Description("Job ID from a slow PREVIEW (>45s). Pass to APPLY to commit that specific snapshot. Pass to STATUS to poll progress."))
```

---

### 7. `internal/shared/prompts/md/commit_message.md` y `.txt`

**Qué hacer:** Actualizar los prompts para incluir el WHY del agente.

Añadir al template una nueva variable `{{.Why}}` que se inyecta antes del diff:

```
{{if .Why}}
Developer's reason for this change (expand this into a rich WHY in the body):
{{.Why}}
{{end}}
```

Y actualizar las reglas para que el body incluya:
- `WHY:` expandido con contexto técnico basado en el diff y el `Why` del agente
- `WHAT:` bullets de los cambios por archivo

Si `.Why` está vacío, el modelo puede omitir el WHY o intentar inferirlo marcándolo como inferido.

---

### 8. `internal/adapters/llm/openai_standard/adapter_commit.go`

**Qué hacer:** Añadir el campo `Why` al struct que se pasa al template de commit.

Buscar el struct que se usa para renderizar el template `commit_message.md` (probablemente algo como `CommitPromptData` o similar). Añadir campo `Why string`.

El `Why` llega desde el `instruction` que pasa el agente en el PREVIEW. El handler lo extrae del param `instruction` y lo pasa al CommitService, que lo pasa al LLM adapter.

---

## Orden de implementación

1. **`ports/git.go`** — Añadir `WriteTree`, `CommitTree`, `UpdateRef`, `Head` a la interfaz. El código no compila hasta que el adapter los implemente.

2. **`adapters/git/exec_write_commit.go`** — Implementar los cuatro métodos. Compilar y verificar que no hay errores.

3. **`handlers.go` — `BgJob`** — Añadir `TreeHash` y `Done chan struct{}`. No rompe nada todavía.

4. **`handlers.go` — `handlePreview`** — Añadir el `WriteTree` síncrono antes del goroutine. Añadir `close(j.Done)` al goroutine. Asegurarse de que el `treeHash` queda en el job.

5. **`workflow/commit.go`** — Añadir `GenerateCommitMessage` (Opción A). Este método solo genera el mensaje, no ejecuta git.

6. **`handlers.go` — `handleApply`** — Cambiar para usar el plumbing. Leer job_id → treeHash → GenerateCommitMessage → CommitTree → UpdateRef → Reset.

7. **`registration.go`** — Actualizar descripción del tool y añadir `job_id` al schema.

8. **Prompts** — Actualizar `commit_message.md` y `.txt` con el campo `Why`. Actualizar el adapter que renderiza el template.

---

## Casos de error que hay que manejar

| Situación | Dónde | Qué hacer |
|---|---|---|
| `WriteTree` falla (staging vacío) | `handlePreview` | Error inmediato, NO crear job |
| `WriteTree` falla (no es repo git) | `handlePreview` | Error inmediato, NOT crear job |
| Ollama no responde | goroutine del LLM | Job → `BgFailed`, cerrar `Done` |
| `CommitTree` falla | `handleApply` | Error con contexto, treeHash ya fue capturado, el staging sigue intacto |
| `UpdateRef` falla | `handleApply` | Error — el commit object existe en `.git/objects` pero HEAD no apunta a él. Loggear el commitHash para que el usuario pueda recuperarlo manualmente |
| job_id no existe en `bgJobs` | `handleApply` | Error `job_id not found — it may have expired if the process restarted` |
| Apply sin job_id | `handleApply` | Comportamiento actual (sin plumbing) — mantener compatibilidad hacia atrás |

---

## Lo que NO hay que implementar ahora

- MCP Tasks nativo (futuro v1.2)
- Cancelación de jobs
- Persistencia de jobs en disco — si el proceso muere los jobs mueren, asumido
- Cola FIFO de applies — el agente llama en el orden que quiere
- Múltiples commits por PREVIEW — siempre uno

---

## Verificación final

Después de implementar, el flujo end-to-end tiene que funcionar así:

```
Agente: commit PREVIEW instruction="why: add refresh token rotation"
git-courer: git write-tree → hash:4b825d
git-courer: lanza goroutine LLM
git-courer (si <45s): devuelve {"message":"feat(auth):...","job_id":"commit-1234"}
git-courer (si >45s): devuelve {"status":"processing","job_id":"commit-1234"}

Agente: [sigue trabajando en otras cosas]

Agente: commit APPLY job_id="commit-1234"
git-courer: espera job.Done si todavía está procesando
git-courer: git commit-tree 4b825d -p HEAD -m "feat(auth): implement JWT refresh token rotation\n\nWHY:..."
git-courer: git update-ref HEAD <nuevo_hash>
git-courer: git reset HEAD .
git-courer: devuelve {"status":"committed","hash":"a1b2c3","message":"feat(auth):..."}

git log: muestra el commit con el mensaje completo
git status: archivos en worktree sin stagear — CORRECTO
```
## Orden de sub-SDDs

El QUEHACER se divide en 5 sub-SDDs independientes, cada uno compilable y testeable por separado:

| # | Sub-SDD | Qué cubre | Archivos principales | Depende de |
|---|---------|-----------|-----------------------|------------|
| A | **git-plumbing** | Puertos + adaptador (WriteTree, CommitTree, UpdateRef, Head) | `ports/git.go`, `adapters/git/exec_write_commit.go` | — |
| B | **job-snapshot** | BgJob struct (TreeHash, Done) + handlePreview (WriteTree síncrono) | `delivery/mcp/core/handlers.go` | A |
| C | **generate-commit-message** | GenerateCommitMessage en workflow/service (Opción A) | `workflow/commit.go` o nuevo archivo, `adapters/llm/` | — |
| D | **apply-plumbing** | handleApply con git plumbing en vez de workflow.Apply | `delivery/mcp/core/handlers.go` | A, B, C |
| E | **tool-description-why** | Registration tool desc, prompts WHY, adapter LLM | `registration.go`, `commit_message.md/txt`, `adapters/llm/` | C |

---

# git-courer v2 — Guía de implementación en humano

---

## Qué estamos construyendo y por qué

El problema actual es que el agente de nube llama a git-courer para hacer un commit y se queda esperando bloqueado hasta que el LLM local termina de generar el mensaje. Si tarda más de lo que el agente aguanta, tira por bash y hace el commit mal o no lo hace. Perdemos todo el valor de git-courer.

La solución es separar el commit en dos momentos:

**Momento 1 — PREVIEW:** El agente dice "quiero hacer un commit de lo que tengo staged ahora". Git-courer fotografía el estado del staging en ese instante y empieza a generar el mensaje. Si termina rápido, devuelve todo junto. Si tarda, devuelve solo un identificador (job_id) y el agente puede irse a hacer otras cosas.

**Momento 2 — APPLY:** Cuando el agente quiera cerrar ese checkpoint, dice "aplica el commit con ese job_id". Git-courer usa la fotografía que tomó antes y el mensaje ya generado para hacer el commit real. El agente nunca estuvo bloqueado.

---

## La fotografía — qué es git write-tree

Cuando el agente tiene archivos en staging y llama a PREVIEW, lo primero que hace git-courer antes de tocar nada más es ejecutar un comando de git llamado `write-tree`.

Lo que hace `write-tree` es tomar exactamente lo que hay en el staging area en ese momento y guardarlo como un objeto inmutable dentro de la carpeta `.git`. Git le asigna un identificador único (un hash). Ese objeto nunca cambia, nunca desaparece mientras el proceso esté vivo.

A partir de ese momento, da igual lo que haga el agente — puede modificar los mismos archivos, puede stagear cosas nuevas, puede hacer lo que quiera. La fotografía ya está tomada y es permanente. Cada PREVIEW tiene su propia fotografía independiente.

Si el staging está vacío cuando llega el PREVIEW, `write-tree` falla y git-courer devuelve error inmediatamente sin crear ningún job. Esto es correcto — no tiene sentido fotografiar un staging vacío.

---

## El job en memoria

Después de tomar la fotografía, git-courer crea un "job" — una estructura pequeña que vive en memoria RAM mientras el proceso esté corriendo.

El job guarda:
- Su identificador único (job_id)
- El hash de la fotografía (tree hash)
- El estado actual: procesando, listo, o fallido
- El mensaje de commit cuando el LLM lo genera
- Un canal de comunicación interno para saber cuándo el LLM termina

Los jobs no tienen tiempo de expiración. Viven exactamente mientras el proceso de git-courer esté vivo. Cuando el proceso muere — porque el usuario cierra el MCP, reinicia el sistema, o lo que sea — los jobs desaparecen automáticamente con él. El sistema operativo libera la memoria. Sin timers, sin goroutines de limpieza, sin configuración.

Un job desaparece explícitamente solo en dos situaciones: cuando el agente llama a ABORT, o cuando el APPLY termina con éxito. En cualquier otro caso el job sigue en memoria hasta que el proceso muera.

El contrato con el agente es claro: el job_id es válido mientras esta sesión del MCP esté activa. Si el MCP se reinicia, los job_ids anteriores ya no existen y hay que hacer un nuevo PREVIEW — lo cual tiene sentido porque el contexto del repo puede haber cambiado.

---

## El timer de 45 segundos

Cuando el agente llama a PREVIEW, git-courer hace dos cosas a la vez:

Por un lado, lanza el LLM local para que empiece a generar el mensaje de commit. El LLM trabaja en segundo plano.

Por otro lado, pone un reloj de 45 segundos.

Si el LLM termina antes de que lleguen los 45 segundos, git-courer devuelve al agente el mensaje completo más el job_id en la misma llamada. El agente puede hacer APPLY inmediatamente si quiere.

Si llegan los 45 segundos y el LLM todavía está trabajando, el servidor MCP corta la respuesta y devuelve solo el job_id al agente. El LLM sigue trabajando en background — no se cancela, no se interrumpe. Cuando termina, actualiza el job en memoria con el mensaje. El agente ya está libre para hacer otras cosas.

El agente nunca nota diferencia entre los dos casos — siempre recibe el job_id rápido. La única diferencia es si también recibe el mensaje en esa misma llamada o tiene que esperar al APPLY.

---

## El APPLY — usar la fotografía para commitear

Cuando el agente llama a APPLY con un job_id, git-courer busca el job en memoria.

Si el LLM todavía está procesando, el APPLY espera a que termine. Esta espera es intencional — el agente eligió hacer APPLY, así que está dispuesto a esperar ese momento final.

Cuando el mensaje está listo, git-courer ejecuta tres operaciones de git en secuencia:

**Primera operación:** Toma la fotografía (el tree hash del job) y el mensaje generado por el LLM y crea un objeto commit en git apuntando a esa fotografía. Esto devuelve el hash del nuevo commit. En este momento el commit existe en el grafo de git pero HEAD todavía no apunta a él.

**Segunda operación:** Mueve HEAD para que apunte al nuevo commit. Ahora el commit es parte oficial del historial.

**Tercera operación:** Limpia el staging area. Los archivos que estaban staged ahora aparecen en el worktree sin stagear — esto es exactamente el comportamiento esperado de un commit normal. El commit está en el log, los archivos siguen en disco.

Estas tres operaciones juntas son lo que `git commit` hace internamente. La diferencia es que aquí los separamos en el tiempo: la fotografía se tomó en el PREVIEW, el mensaje lo generó el LLM en background, y el commit físico se ejecuta en el APPLY.

---

## Por qué el agente no se frustra

La descripción del tool es la única palanca real que tenemos para controlar el comportamiento del agente. El agente lee esa descripción antes de llamar al tool y toma decisiones basándose en ella.

La descripción tiene que decir explícitamente tres cosas:

Primero, que el PREVIEW puede tardar hasta 45 segundos en responder directamente o puede devolver un job_id si tarda más — en ambos casos el agente tiene lo que necesita.

Segundo, que después del APPLY los archivos aparecen sin stagear en el worktree y eso es correcto — el commit está en el log. Sin esta aclaración el agente pensaría que algo falló y volvería a intentar commitear.

Tercero, que mientras espera el APPLY puede y debe seguir trabajando en otras cosas. Darle permiso explícito para ser productivo mientras espera es lo que evita que se impaciente.

---

## El WHY — de dónde viene y cómo se expande

El agente de nube tiene todo el contexto de la conversación — sabe por qué está haciendo el cambio. El LLM local no tiene ese contexto, solo ve el diff.

Por eso el WHY lo aporta el agente cuando llama al PREVIEW, como parte de la instrucción. Algo tan simple como "add refresh token rotation to prevent replay attacks".

Cuando el LLM local genera el mensaje de commit, recibe ese WHY junto con el diff. Su trabajo es dos cosas: describir qué cambió (el WHAT, que sí puede inferir del diff) y expandir el WHY del agente con contexto técnico real basado en lo que ve en el código.

El resultado final es un cuerpo de commit profesional con WHY expandido y WHAT detallado — no una línea genérica de "update files".

---

## El pipeline del LLM para diffs grandes

Si el diff completo cabe en la ventana de contexto del modelo — una sola llamada al LLM con todo el diff más el WHY del agente. Mensaje perfecto, mínima latencia.

Si el diff es demasiado grande para la ventana — una llamada por archivo. Cada llamada recibe solo el diff de ese archivo más el WHY global del agente, y devuelve una frase de WHAT y una frase de WHY local para ese archivo específico. Estas llamadas van en paralelo si el hardware lo permite, en serie si no.

Luego una llamada de síntesis final que recibe todas las frases por archivo concatenadas (que sí caben en la ventana porque son frases cortas) y genera el mensaje de commit completo con el WHY expandido.

El LLM nunca ve más de un archivo a la vez excepto en la síntesis, donde ya no ve código sino frases.

---

## El modelo recomendado y por qué importa la ventana

La gente asume que más parámetros significa mejor resultado. Para git-courer eso es falso — lo que importa es la ventana de contexto.

Un modelo de 3 mil millones de parámetros con 32.000 tokens de ventana (como Qwen2.5-Coder 3B) es mejor para esta tarea que un modelo de 7 mil millones con 4.000 tokens de ventana. El LLM recibe el diff ya procesado y masticado — no necesita ser brillante, necesita ver todo el contexto de una vez.

Qwen2.5-Coder 3B tiene exactamente 32.000 tokens de ventana. Un archivo de 250 líneas ocupa aproximadamente 3.750 tokens — cabe perfectamente en una llamada de sub-chunk. El fallback de truncar va a ser rarísimo.

Hay que documentar esto claramente para que el usuario no elija un modelo por tamaño sino por ventana de contexto.

---

## Qué hay que cambiar en el código existente

Todo lo que existe ya funciona. Los cambios son quirúrgicos:

**En el contrato de git (el port):** Añadir tres operaciones nuevas — fotografiar el staging, crear un commit desde una fotografía, y mover HEAD a un commit. Son operaciones de git plumbing que ya existen en git, solo hay que exponerlas.

**En el adaptador de git:** Implementar esas tres operaciones llamando a los comandos de git correspondientes. Nada complejo — son comandos de una línea.

**En el handler de PREVIEW:** Antes de lanzar el LLM, tomar la fotografía y guardar el hash en el job. Si falla, error inmediato. El goroutine que ya existe solo necesita cerrar el canal Done cuando termina para que el APPLY pueda esperar en él.

**En el handler de APPLY:** En vez de dejar que el workflow ejecute `git commit` internamente, el handler toma el control después de que el LLM genera el mensaje y ejecuta las tres operaciones de plumbing con la fotografía capturada en el PREVIEW.

**En los prompts del LLM:** Añadir el campo WHY que viene del agente para que el modelo lo expanda en el cuerpo del commit.

**En la descripción del tool:** Actualizar el texto que ve el agente para que entienda el contrato nuevo — qué esperar, qué no hacer, que puede seguir trabajando mientras espera.

---

## Orden en que hay que hacer los cambios

Primero el contrato (el port), porque si no el código no compila. Segundo el adaptador, porque implementa el contrato. Tercero el job con la fotografía en el PREVIEW. Cuarto la separación entre generar el mensaje y ejecutar el commit en el APPLY. Quinto los prompts. Sexto las descripciones de los tools.

Cada paso compila y es testeable antes de pasar al siguiente.

---

## Verificación de que funciona

El flujo correcto de extremo a extremo:

El agente llama a PREVIEW con el WHY. Git-courer fotografía el staging, lanza el LLM. Si termina en menos de 45 segundos devuelve mensaje y job_id juntos. Si no, devuelve solo job_id y el agente sigue trabajando.

El agente llama a APPLY con ese job_id. Si el LLM no ha terminado, espera. Cuando tiene el mensaje, git-courer crea el commit apuntando a la fotografía, mueve HEAD, limpia el staging.

El agente ve los archivos sin stagear en el worktree. Eso es correcto. El `git log` muestra el commit con el mensaje completo y profesional.

---

## Por qué el agente nunca tira por bash

Un agente bloqueado esperando solo tiene dos opciones: seguir esperando hasta frustrarse o tirar por bash. Un agente que tiene trabajo que hacer no mira el reloj.

La descripción del tool le dice explícitamente que puede y debe seguir trabajando mientras espera. Eso lo desbloquea. El agente vuelve a su cola de tareas — escribe más código, lee archivos, lo que sea — y cuando termina esa tarea llama al APPLY de forma natural, como parte del flujo, no como una comprobación ansiosa.

El bash deja de ser una opción no solo porque la descripción lo prohíbe, sino porque el agente genuinamente no tiene motivo para usarlo. El flujo de trabajo nunca se interrumpe. El agente se entretiene haciendo código, no mirando el estado.