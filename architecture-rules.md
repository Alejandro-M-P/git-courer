# git-courer - Normas de Arquitectura

# objetivo hacer una clean arquitecture perfecta (para si mañana tener que cambiar algo sea solo un archivo)

---

## 1. ESTRUCTURA DE CARPETAS

```
internal/
├── core/           ← CIEGO y TONTO
│   ├── domain/
│   ├── config/
│   ├── ports/
│   └── errors/
│
├── app/
│   └── {dominio}/
│       ├── services.go
│       ├── commands.go
│       └── ...
│
├── infra/
│   ├── git/
│   ├── llm/
│   ├── mcp/
│   ├── secrets/
│   ├── logging/
│   └── config/
│
└── wiring/         ← Composition Root
```

---

## 2. REGLAS FUNDAMENTALES

1. **Core es ciego y tonto** → NO conoce app ni infra
2. **App conoce Core** → NO importar infra directamente
3. **Infra conoce Core** → NO importar app directamente
4. **Wiring conecta todo** → único lugar donde se tocan app + infra
5. **Excepciones**: logging, metrics, tests

---

## 3. NOMENCLATURA

| Tipo | Sufijo | Ejemplo |
|------|--------|---------|
| Puerto (interfaz) | Port | GitPort, SecurityPort |
| Adapter (implementación) | Adapter | ExecAdapter, OllamaAdapter |
| Servicio (app) | Service | SecurityService, CommitService |
| Comandos | commands.go | app/{dominio}/commands.go |

---

## 4. DIVISIÓN DE ARCHIVOS

### Cuando separar
- Archivo con **+200 líneas** → separar
- Lógica que puede reutilizarse → mover a archivo propio
- Tests en archivo separado (`*_test.go`)

### Cómo separar
Dentro de cada dominio:
```
app/commit/
├── services.go    ← lógica de negocio
├── commands.go   ← handlers/MCP
├── blocking.go   ← gestión de locks
├── plan.go       ← gestión de planes
└── ...
```

---

## 5. DEPENDENCIAS

### Lo que PUEDE importarse
- app → core (domain, ports, errors, config)
- infra → core (domain, ports, errors)
- wiring → todo

### Lo que NO PUEDE
- app → infra (NUNCA)
- core → app o infra (NUNCA)

### Cómo recibir dependencias
```go
// ❌ MAL - crea sus propios adapters
func NewService(cfg *config.Config) *Service {
    adapter := git.NewAdapter() // violando reglas
}

// ✅ BIEN - recibe dependencias inyectadas
func NewService(gitPort ports.GitPort, modelSize string) *Service {
    // solo recibe lo que necesita
}
```

---

## 6. CONFIGURACIÓN

- Valores puros (strings, ints, bools) → en `core/config`
- Implementación (lectura de archivos YAML) → en `infra/config`
- Los services reciben config como PARÁMETROS, no crean config

---

## 7. COMPOSITION ROOT (wiring/)

唯一 lugar donde app e infra se encuentran:

```go
// cmd/main.go o wiring/wire.go

// 1. Crear adapters (infra)
gitAdapter := git.NewExecAdapter(workDir)
llmAdapter := llm.NewAdapter(host, model)

// 2. Crear servicios (app) - pasando adapters
securitySvc := security.NewSecurityService(secretDetector, modelSize)

// 3. Crear servidor MCP - pasando todo
mcpServer := mcp.NewServer(gitAdapter, llmAdapter, securitySvc)
```

---

## 8. ERRORES COMUNES

- ❌ Service importa infra/config
- ❌ Service crea sus propios adapters
- ❌ Adapter importa otro adapter
- ❌ MCP server crea adapters (debe recibirlos)
- ❌ Archivo de 500+ líneas sin separar

---

## 9. FLUJO DE TRABAJO (SDD)

### Norma principal
**Un archivo = Un change = Un SDD completo**

### Fases SDD
```
explore → propose → spec → design → tasks → apply → verify → archive
```

### Reglas\

1. **Dividir por archivo** — cada archivo auditado = su propio change
2. **CRITICAL primero** — siempre priorizar bugs críticos sobre warnings/suggestions
3. **Un archivo por change** — cuando sea posible, un change = un archivo
4. **Explorar antes de SDD** — investigar el archivo primero, documentar bugs
5. **Interactive mode** — pausas entre fases para review (salvo que usuario pida automático)
6. **Artifact store: engram** — persistencia en memoria (default)
7. **Un cambio a la vez** — no mezclar múltiples archivos en un mismo SDD



### Proceso por archivo
1. Explorar archivo → reportar bugs + violaciones de arquitectura
2. Si hay issues → SDD completo (propose → spec → design → tasks → apply → verify → archive)
3. Marcar en `audit-progress.md` como revisado
8. **Ver que el archivo correspondinte tiene bugs y sigue las reglas** verificar que el archivo correspondiente tiene bugs o si implementa las reglas de arquitectura 
9. **Borrar cosas que ya no se usen o que sean innecesatias** se hace despues de todos los cambios 
4. Avanzar al siguiente archivo

### Artifact store
- **engram** (default) — rápido, en memoria
- **openspec** — archivos en `openspec/` (para compartir con equipo)

---

Última actualización: 2026-04-08