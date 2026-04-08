# git-courer — Auditoría Completa

---

## 1. CLEAN ARCHITECTURE: El core NO está ciego

### El problema central

`internal/infra/mcp/server.go` viola hexagonal architecture en múltiples puntos:

```
internal/infra/mcp/server.go
  imports → internal/app/security (usecase)        ← OK
  imports → internal/infra/diff (chunker)           ← VIOLACIÓN: infra importa infra
  imports → internal/infra/git (gitadapter)         ← VIOLACIÓN: infra crea sus propios adapters
  imports → internal/shared/classifier              ← VIOLACIÓN: lógica de dominio en shared
  imports → internal/shared/parser                  ← VIOLACIÓN: lógica de dominio en shared
```

El MCP server actúa como Composition Root PERO también hace routing de negocio (clasificar intents, extraer branch names). Eso no es su trabajo.

`internal/app/security/security_service.go`:
```go
import "github.com/Alejandro-M-P/git-courer/internal/infra/config"   // ← VIOLACIÓN
import "github.com/Alejandro-M-P/git-courer/internal/infra/secrets"  // ← VIOLACIÓN
```
Un usecase de app no puede importar `infra`. Los secrets deben llegar por puerto.

`internal/app/commit/blocking.go`:
```go
import "github.com/Alejandro-M-P/git-courer/internal/infra/config"   // ← VIOLACIÓN
```
Misma violación.

`internal/app/commit/plan.go`:
```go
import "github.com/Alejandro-M-P/git-courer/internal/infra/config"   // ← VIOLACIÓN
```

### Estructura de carpetas correcta

```
internal/
├── core/
│   ├── domain/          ← sin cambios, está bien
│   ├── ports/           ← sin cambios, está bien
│   └── errors/          ← sin cambios, está bien
│
├── app/                 ← usecases, SOLO dependen de core/
│   ├── branch/          ← OK
│   ├── commit/          ← ARREGLAR: sacar config de aquí
│   │   ├── commit.go
│   │   ├── blocking.go  ← sacar config, recibir TTL/paths como parámetros
│   │   └── plan.go      ← sacar config, recibir planFilePath como parámetro
│   ├── operations/      ← OK
│   ├── query/           ← OK
│   ├── remote/          ← OK
│   ├── security/        ← ARREGLAR: sacar infra/config e infra/secrets
│   └── setup/           ← OK (es CLI, no usecase real)
│
├── infra/               ← implementaciones concretas
│   ├── config/          ← OK
│   ├── diff/            ← OK
│   ├── git/             ← OK
│   ├── llm/             ← OK (ollama_adapter)
│   ├── logging/         ← OK
│   ├── mcp/             ← ARREGLAR: sacar routing de negocio
│   └── secrets/         ← OK
│
└── shared/
    ├── classifier/      ← MOVER a app/ o core/domain/
    ├── formatter/       ← OK
    ├── parser/          ← MOVER a app/ o core/domain/
    ├── prompts/         ← OK
    └── stats/           ← OK
```

### Qué cambia y dónde

**`internal/app/commit/blocking.go`**
- Línea 16: `import "github.com/.../infra/config"` → ELIMINAR
- Línea 57: `cfg *config.Config` → cambiar a campos sueltos
- Constructor `NewBlockingManager(git ports.Git, cfg *config.Config)` → cambiar a:
  ```go
  func NewBlockingManager(git ports.Git, lockFile, planFile string, ttl time.Duration) *BlockingManager
  ```
- El `planStore` recibe `planFilePath string` directamente en vez de `*config.Config`

**`internal/app/commit/plan.go`**
- Línea 9: `import "github.com/.../infra/config"` → ELIMINAR
- `newPlanStore(cfg *config.Config)` → cambiar a:
  ```go
  func newPlanStore(planFilePath string) *planStore
  ```

**`internal/app/security/security_service.go`**
- Línea 7: `import "github.com/.../infra/config"` → ELIMINAR
- Línea 8: `import "github.com/.../infra/secrets"` → ELIMINAR
- La detección de secrets debe llegar como `ports.SecretDetector` (nuevo puerto)
- La config debe llegar como parámetros sueltos: `modelSize domain.ModelSize, llmScanOverride string`
- Constructor:
  ```go
  func NewSecurityService(modelSize domain.ModelSize, llmScanOverride string, detector ports.SecretDetector) *securityService
  ```

**Nuevo puerto `internal/core/ports/secret_detector.go`:**
```go
package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

type SecretDetector interface {
    Detect(files []string) ([]domain.SecretDetection, error)
    IsBinary(path string) bool
    IsBlacklistedFolder(path string) bool
    IsBlacklistedName(name string) bool
}
```

**`internal/infra/secrets/`** → crear `adapter.go` que implemente `ports.SecretDetector`

**`internal/infra/mcp/server.go`**
- Líneas 12-13: sacar imports de `infra/diff` y `infra/git` → estos adapters se crean en `cmd/main.go` y se inyectan
- Líneas 14-15: `shared/classifier` y `shared/parser` → mover a `internal/app/routing/` o inyectar como dependencia
- `NewServer` no debe crear adapters. Solo recibir puertos ya construidos.
- El routing de intents (switch en `handleGitLocalTask`) puede quedarse pero debe recibir el classifier inyectado

**`cmd/main.go`** (Composition Root real) debe crear:
```go
// Todos los adapters
gitAdapter := git.NewExecAdapter(cfg.Git.WorkDir)
chunker := diff.NewChunker()
secretDetector := secrets.NewAdapter()
securitySvc := security.NewSecurityService(modelSize, cfg.Secrets.UseLLMSecurityScan, secretDetector)
commitSvc := commit.NewService(gitAdapter, ollamaAdapter, chunker, securitySvc)
// etc.
// Y pasarlos a mcp.NewServer — que NO crea nada
```

---

## 2. BUGS Y FALLOS

### Bug crítico: Race condition en `handleGitWriteReview`

**Archivo:** `internal/infra/mcp/server.go`, función `handleGitWriteReview`

```go
// Acquire lock and wait for user confirmation
if err := srv.gitWriteCommit.AcquireLock(); err != nil {
    return mcp.NewToolResultError("Another operation is in progress."), nil
}
defer srv.gitWriteCommit.ReleaseLock()

// Signal that we have a pending operation
srv.gitWriteCommit.Approve()          // ← APRUEBA antes de esperar

// Wait for user confirmation
if !srv.gitWriteCommit.WaitForConfirmation() {  // ← espera DESPUÉS de haber aprobado
```

Esto es un deadlock lógico / auto-aprobación. Se llama `Approve()` y luego `WaitForConfirmation()` en el mismo goroutine. `WaitForConfirmation()` en `GitWriteCommitAdapter` usa `cond.Wait()` que solo se despierta cuando alguien llama `Signal()`, pero `Approve()` ya lo hizo antes. El resultado depende del orden de ejecución del mutex. En la implementación actual `confirmed = true` se setea antes del Wait, así que el Wait sale inmediatamente siempre → **la confirmación del usuario nunca se espera realmente**. El `git_write_review` ejecuta sin esperar al usuario.

### Bug: `ReadPlan()` en `GitWriteCommitAdapter` no parsea JSON

**Archivo:** `internal/infra/git/git_write_commit_adapter.go`, línea ~100

```go
func (a *GitWriteCommitAdapter) ReadPlan() (*ports.CommitPlan, error) {
    data, err := os.ReadFile(a.planFile)
    // ...
    plan := &ports.CommitPlan{}
    _ = data // TODO: proper JSON unmarshal   ← NUNCA se parsea
    return plan, nil
}
```

Siempre devuelve un plan vacío. `COMMIT_STATUS` en el MCP server muestra datos incorrectos. Si algo depende de `plan.Commits` para hacer rollback, nunca habrá rollback correcto.

### Bug: `WritePlan()` en `GitWriteCommitAdapter` serializa JSON manualmente (roto)

**Archivo:** `internal/infra/git/git_write_commit_adapter.go`, línea ~75

```go
data := fmt.Sprintf(`{"files":%v,"message":"%s",...}`, plan.Files, plan.Message, ...)
```

`%v` en un `[]string` produce `[elem1 elem2]` (sin comillas, sin comas JSON-válidas). Si `plan.Message` contiene comillas, el JSON se rompe. Usar `json.Marshal`.

### Bug: `context.Background()` sin timeout en `runGit`

**Archivo:** `internal/infra/git/exec_adapter.go`, línea 32

```go
ctx := context.Background()
cmd := exec.CommandContext(ctx, "git", args...)
```

Ningún timeout. Si git cuelga (e.g., `git clone` de repo grande, `git pull` con red lenta), el proceso se queda bloqueado para siempre. El MCP server no puede responder. Usar timeout configurable, mínimo 30s para operaciones locales, 120s para remote.

### Bug: `append` sobre slice compartido en `runGit`

**Archivo:** `internal/infra/git/exec_adapter.go` — varios métodos como `Add` y `Remove`:

```go
args := append([]string{"add"}, paths...)
```

Esto es correcto aquí porque crea nuevo slice. Pero en `Push()`:

```go
func (a *ExecAdapter) Push() (string, error) {
    out, err := a.runGit("push")
    // ...
    a.runGit("fetch", "origin")           // error ignorado
    pullOut, pullErr := a.runGit("pull", "--rebase")
    // ...
    out, err = a.runGit("push")
```

El error de `fetch` se ignora silenciosamente. Si fetch falla (sin red), el pull puede devolver datos stale. Capturar y loguear al menos.

### Bug: Double-lock en `AcquireLock` de `BlockingManager`

**Archivo:** `internal/app/commit/blocking.go`, función `AcquireLock`, línea ~133

```go
func (bm *BlockingManager) AcquireLock() error {
    bm.mu.Lock()         // ← toma el mutex
    defer bm.mu.Unlock()

    // ...
    file, err := os.OpenFile(bm.lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
```

Y `IsLockStale()`:
```go
func (bm *BlockingManager) IsLockStale() (bool, error) {
    info, err := os.Stat(bm.lockFile)
```

`IsLockStale` se llama desde `CleanupAndReset` que YA tiene `bm.mu.Lock()` → deadlock si `bm.mu` es un `sync.Mutex` (no RWMutex). Específicamente en `CheckAndCleanupStaleResources`:

```go
func (bm *BlockingManager) CheckAndCleanupStaleResources() error {
    bm.mu.Lock()           // ← toma el lock
    defer bm.mu.Unlock()
    
    isStale, err := bm.IsLockStale()  // ← IsLockStale también hace bm.mu.Lock() → DEADLOCK
```

### Bug: `CleanupAndReset` silencia errores con shadowing

**Archivo:** `internal/app/commit/blocking.go`, función `CleanupAndReset`

```go
var cleanupErr error

isStale, err := bm.IsLockStale()  // err declarado aquí
if err != nil {
    cleanupErr = fmt.Errorf(...)
} else if isStale { ... }

plan, err := bm.planStore.ReadPlan()  // err redeclarado, cleanupErr sobrescrito
if err != nil {
    cleanupErr = fmt.Errorf(...)      // ← borra el error anterior
}
```

Cada asignación a `cleanupErr` borra la anterior. Si hay múltiples errores, solo se reporta el último. Usar `errors.Join` (Go 1.20+) o acumular en slice.

### Bug: `IsPlanExpired` llamado con lock del caller en `CheckAndCleanupStaleResources`

Mismo problema que el deadlock anterior. `IsPlanExpired` es una función libre que no toca mutex, pero `CheckAndCleanupStaleResources` ya tiene el lock y llama a métodos de `planStore` que también podrían intentar tomar locks.

### Bug: `RotatingLogWriter` no es thread-safe

**Archivo:** `internal/infra/logging/rotating.go`

```go
func (w *RotatingLogWriter) Write(p []byte) (n int, err error) {
    w.lines = append(w.lines, string(p))  // ← sin mutex
    // ...
    os.WriteFile(w.path, ...)
```

`log.Printf` puede llamarse desde múltiples goroutines (el background goroutine en `executeBackground` lo hace). Race condition en `w.lines`. Añadir `sync.Mutex`.

### Bug: Goroutine leak en `executeBackground`

**Archivo:** `internal/app/commit/commit.go`, función `executeBackground`

```go
go func() {
    // ...
    resultChan := make(chan chunkResult, len(chunks))
    
    go func() {  // ← inner goroutine
        for i, chunk := range chunks {
            message, err := s.llm.GenerateChunkMessage(chunk)
            resultChan <- chunkResult{...}
        }
        close(resultChan)
    }()
    
    for result := range resultChan { ... }
}()
```

Si el outer goroutine termina antes (por ejemplo, si la aplicación recibe SIGTERM), el inner goroutine queda bloqueado intentando enviar a `resultChan`. No hay context propagation ni cancelación. El `ctx` del MCP handler no llega hasta aquí.

### Bug: `executeSync` tiene el mismo problema de goroutine

**Archivo:** `internal/app/commit/commit.go`, función `executeSync`

```go
go func() {
    for i, chunk := range chunks {
        message, err := s.llm.GenerateChunkMessage(chunk)
        resultChan <- chunkResult{...}
    }
    close(resultChan)
}()

for result := range resultChan { ... }
```

Si el committer loop sale por error antes de agotar el canal (actualmente no lo hace pero si se añade un break), el worker goroutine queda bloqueado en el send. Pasar un `ctx` con cancel.

### Bug lógico: Status parsing de porcelain ignora archivos renombrados con flecha

**Archivo:** `internal/infra/git/exec_adapter.go`, función `Status`

`git status --porcelain` para archivos renombrados produce: `R  old-name -> new-name`

```go
path := line[3:]  // ← captura "old-name -> new-name" completo
```

El path queda como `"old-name -> new-name"` en lugar de separar ambos nombres. Si luego se hace `git add "old-name -> new-name"`, git falla. Para renombrados, `line[3:]` puede contener `->` que hay que separar.

### Bug: `ResetHard` en `GitWriteReviewAdapter` pasa `"hard"` en vez de `"--hard"`

**Archivo:** `internal/infra/git/git_write_review_adapter.go`, línea ~75

```go
func (a *GitWriteReviewAdapter) ResetHard(target string) (string, error) {
    return a.exec.Reset("hard", target)
}
```

`ExecAdapter.Reset` hace `runGit("reset", mode, commit)` → `git reset hard <target>`. Git espera `--hard`. Resultado: `git reset hard HEAD` falla con "unknown option". Debe ser `"--hard"`.

### Bug: `IsRepo()` no funciona para repos bare o worktrees

**Archivo:** `internal/infra/git/exec_adapter.go`

```go
func (a *ExecAdapter) IsRepo() bool {
    _, err := os.Stat(fmt.Sprintf("%s/.git", a.workDir))
    return err == nil
}
```

En git worktrees y repos bare `.git` no es un directorio sino un archivo o no existe en ese path. Mejor usar `git rev-parse --git-dir` que funciona en todos los casos.

### Bug: `COMMIT_START` con `preview=true` tiene lógica invertida

**Archivo:** `internal/infra/mcp/server.go`, `handleGitWriteCommit`

```go
case git_write_commit.COMMIT_START:
    // ...
    if preview {
        if err := srv.gitWriteCommit.AcquireLock(); err != nil { ... }
        defer srv.gitWriteCommit.ReleaseLock()
        
        srv.gitWriteCommit.Approve()   // ← auto-aprueba ANTES de esperar
        
        if !srv.gitWriteCommit.WaitForConfirmation() {  // ← espera DESPUÉS
```

Mismo problema que `handleGitWriteReview`: se auto-aprueba y luego espera. La confirmación nunca bloquea realmente al usuario. El flujo de preview está roto.

### Warning: `fmt.Sscanf` sin verificar retorno en `handleGitWriteReview`

```go
commits := 1
if subCommand != "" {
    fmt.Sscanf(subCommand, "%d", &commits)  // ← si falla, commits queda en 1 silenciosamente
}
```

Menor, pero si el usuario pasa `"two"` esperando 2 commits, se hace reset de 1 sin warning.

### Warning: Doble registro de `var _ ports.LLM = (*Adapter)(nil)` 

**Archivo:** `internal/infra/llm/ollama_adapter.go`

Línea ~220 y línea ~330: la interface check está duplicada. No causa error de compilación pero es ruido.

### Warning: `CleanupAndReset` e `IsLockStale` están en el mismo `BlockingManager` pero `IsLockStale` ya está expuesto público

`IsLockStale` tiene `bm.mu.Lock()` implícito dentro y `CleanupAndReset` también. Leer la sección de deadlocks arriba.

---

## 3. EVALUACIÓN DE COMMITS

### Commit: `fix: remove debug logging from Ollama adapter`

```
Removed file I/O overhead from GenerateChunkMessage and generateWithThink.
Debug logging was blocking execution and is no longer needed.
```

**Evaluación: Bien escrito, pero el cuerpo necesita verificación**

El mensaje sigue exactamente el template de `generate_message.txt` del propio proyecto (lo cual tiene sentido, fue generado por el mismo tool). El formato es correcto: tipo + descripción corta imperativa + body con WHY.

Sin embargo hay un problema conceptual: el commit message dice "file I/O overhead" y "blocking execution", pero en el código actual de `ollama_adapter.go` que tengo, `generateWithThink` y `GenerateChunkMessage` **no tienen debug logging visible**. Esto significa una de dos cosas:
- El commit ya se aplicó correctamente y lo que veo es el estado post-fix (correcto)
- El body describe algo que no existía (el logging nunca fue tan problemático como dice)

Si el logging realmente hacía I/O en el hot path (cada llamada a Ollama → write a disco), entonces el fix es legítimo y el mensaje es preciso. Si solo era `log.Printf` a stderr, decir "blocking execution" es exagerado.

**Veredicto:** El mensaje es técnicamente correcto en formato. El "blocking" puede ser hiperbólico si era solo stderr logging. Está bien para un commit de limpieza.

### Commit: `feat: add GitWriteReviewAdapter to implement ports.GitWriteReviewPort`

```
Delegates branch, tag, merge, and reset operations to underlying ExecAdapter.
Ensures adapter conforms to GitWriteReviewPort interface for dependency injection.
```

**Evaluación: Cuerpo es puro relleno**

El body no dice nada que no esté en el título. "Delegates operations to ExecAdapter" — obvio, es un adapter. "Ensures adapter conforms to interface" — es lo que hace `var _ ports.GitWriteReviewPort = (*GitWriteReviewAdapter)(nil)`, no necesita body.

Además contiene el bug de `ResetHard` que pasa `"hard"` en vez de `"--hard"`. El commit introduce un bug silencioso.

Un body útil habría sido: "Wraps ExecAdapter to satisfy the review port. Note: Reset operations delegate to exec.Reset() which expects --hard/--soft flags."

**Veredicto:** Mensaje correcto en formato pero body vacío de información real. El código tiene un bug. Rechazable como está.

### Commit: `feat: implement missing Git operations in ExecAdapter`

```
Added Switch, Remove, CreateBranch, DeleteBranch, RenameBranch, DeleteTag,
RebaseContinue, RebaseAbort, AddRemote, RemoveRemote, Init, Clone, ResetSoft,
and Reflog methods to the adapter.
```

**Evaluación: Body es un listado que ya está en el diff, no aporta WHY**

El body lista todos los métodos añadidos. Eso ya lo muestra `git diff`. El WHY sería: "These operations were needed to implement the new port interfaces (GitWritePort, GitWriteReviewPort) introduced for the multi-tool MCP architecture."

Además el commit es muy grande (14 métodos nuevos). Para un portfolio piece como AXIOM/git-courer, commits atómicos son mejores. Podría haberse dividido en: operaciones de branch/tag, operaciones de remote, operaciones de reset/clean.

**Veredicto:** Funciona pero es un dump. Para portfolio, dividirlo o mejorar el body con el contexto arquitectural.

---

## 4. RESUMEN DE PRIORIDADES

### Crítico (rompe funcionalidad)
1. `ResetHard` pasa `"hard"` sin `--` → falla silenciosamente
2. `ReadPlan()` no parsea JSON → rollback nunca funciona
3. `Approve() + WaitForConfirmation()` en mismo goroutine → preview mode roto
4. Deadlock en `CheckAndCleanupStaleResources` → `IsLockStale` dentro de lock ya adquirido

### Alto (bugs reales pero no siempre visibles)
5. `runGit` sin timeout → cuelgue permanente en operaciones remote
6. `WritePlan` serializa JSON manualmente con `%v` → JSON inválido para slices
7. `RotatingLogWriter` sin mutex → race condition con background goroutine
8. Goroutine leak en `executeBackground` sin context propagation

### Medio (Clean Architecture)
9. `security_service.go` importa `infra/` → violación de capas
10. `blocking.go` y `plan.go` importan `infra/config` → violación de capas
11. `mcp/server.go` crea adapters propios en vez de recibirlos inyectados

### Bajo (calidad/mantenibilidad)
12. Status parsing ignora `->` en archivos renombrados
13. `IsRepo()` no funciona con worktrees/bare repos
14. `fmt.Sscanf` sin verificar retorno
15. `var _ ports.LLM` duplicado en ollama_adapter.go