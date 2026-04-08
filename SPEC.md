

---

## FLOW: Arquitectura de Bifurcación (Phase 1)

### MCP Tool: git_write_commit

Un solo tool MCP con subcomandos para el flujo de commits:

```
git_write_commit(subcommand: "start" | "status" | "summary" | "apply" | "abort", ...)
```

**Subcomandos:**

| Subcommand | Descripción |
|------------|-------------|
| `start` | Genera el plan de commits, bloquea hasta confirmación del usuario. Retorna `pending` si hay que esperar. |
| `status` | Verifica si el plan está listo. Retorna `pending` o `ready`. |
| `summary` | Obtiene los mensajes del plan generado. |
| `apply` | Ejecuta los commits (si está bloqueado) o ejecuta directamente (si preview=false). |
| `abort` | Cancela, limpia lock + plan, hace ResetSoft si es necesario. |

### TTL Configurable

- **Default:** 10 minutos
- **Configurable** en `.gcourer/config.yaml`
- Si el proceso crashea con un plan pendiente:
  - Al reiniciar, detectar stale lock file
  - Si TTL expiró → cleanup: remover lock + plan + ResetSoft si es necesario
  - Sin traces left

```yaml
git_write_commit:
  ttl_minutes: 10  # default
```

### Crash Handling

```
┌─────────────────────────────────────────────────────────────────┐
│                    CRASH RECOVERY FLOW                          │
└─────────────────────────────────────────────────────────────────┘

   App Start
       │
       ▼
┌─────────────────┐
│ Lock file exists?│
└────────┬────────┘
         │
    ┌────┴────┐
    │         │
   NO        YES
    │         │
    │         ▼
    │  ┌─────────────────┐
    │  │ TTL expired?     │
    │  └────────┬────────┘
    │           │
    │      ┌────┴────┐
    │      │         │
    │     NO        YES → Cleanup: rm lock, rm plan, git reset --soft
    │      │         │
    │      ▼         ▼
    │   Normal    Resume normal
    │   startup   operations
    │      │         │
    └──────┴─────────┘
         │
         ▼
   Normal Flow
```

### Commit Workflow Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                     COMMIT WORKFLOW FLOW                        │
└─────────────────────────────────────────────────────────────────┘

User: "commitea todo"
        │
        ▼
┌─────────────────┐
│ Ollama decide   │  ← Qué commitlear
└────────┬────────┘
        │
        ▼
┌─────────────────┐
│   Go chunks     │  ← Divide diff en partes
└────────┬────────┘
        │
        ▼
┌─────────────────┐
│ Ollama genera   │  ← Mensajes para cada chunk
│  los mensajes   │
└────────┬────────┘
        │
        ▼
   ══════════════
   ║ BIFURCACIÓN ║
   ══════════════
        │
        ├────────────────────────┐
        │                        │
        ▼                        ▼
┌───────────────┐      ┌─────────────────┐
│ PREVIEW=TRUE  │      │ PREVIEW=FALSE   │
│               │      │                 │
│   CARRIL A    │      │   CARRIL B      │
│ (espera user's│      │   (directo)    │
│ confirmacón)  │      │                 │
└───────┬───────┘      └────────┬────────┘
        │                        │
        │    ┌───────────────────┘
        ▼    ▼
   ══════════════
   ║PUNTO UNIÓN ║
   ══════════════
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                    GO EJECUTA COMMITS                           │
│                                                                 │
│            ↑↑↑ CÓDIGO EXISTENTE, NUNCA TOCADO ↑↑↑              │
└─────────────────────────────────────────────────────────────────┘
```

### Carriles Explicados

**Carril A (preview=true)**
- El código VA al punto de espera
- Se DETIENE hasta que el usuario confirme
- Ollama genera el plan, AI muestra al usuario
- User confirma → `commit_apply` → ejecuta

**Carril B (preview=false)**
- El código PASA DE LARGO por el punto de unión
- Va directo al execute sin detenerse
- Sin preview, sin polling, sin esperar
- `commit_apply` → ejecuta directamente

### Punto de Unión
- Donde ambos carriles se encuentran
- El código existente de execute NUNCA se modifica
- Solo se añade la bifurcación antes

### Protocolo de Ejecución

**Para preview=true:**
```
1. git_write_commit(subcommand="start") → returns "pending" (respuesta rápida)
2. AI polls con git_write_commit(subcommand="status") hasta que returns "ready"
3. AI obtiene git_write_commit(subcommand="summary") → muestra plan al usuario
4. Usuario confirma
5. git_write_commit(subcommand="apply") → ejecuta
```

**Para preview=false:**
```
1. git_write_commit(subcommand="apply") → ejecuta directamente
2. Sin preview, sin polling, sin punto de espera
```

**Abort:**
```
git_write_commit(subcommand="abort") → cancela, cleanup, ResetSoft si necesario
```

### Principio Clave
**El punto de espera es lo ÚNICO que se añade.** El código de execute existe y nunca se toca — solo se añade la bifurcación antes del punto de unión.