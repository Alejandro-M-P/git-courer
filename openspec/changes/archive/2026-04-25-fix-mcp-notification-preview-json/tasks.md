# Tasks: Fix MCP Notification Preview JSON

## Phase 1: Setup & Infrastructure

- [ ] 1.1 Crear archivo `internal/delivery/mcp/helpers.go` con tipos struct para `StructuredPreview`, `PreviewSection`, `Action`
- [ ] 1.2 Definir constantes de operation types (`OpCommit`, `OpRelease`, `OpBranch`, `OpTag`, `OpMerge`, `OpReset`, `OpCherryPick`, `OpRevert`, `OpClean`, `OpRemote`, `OpClone`, `OpInit`, `OpGeneric`)
- [ ] 1.3 Implementar helper functions para section templates: `commitSections`, `releaseSections`, `genericSections`, `processingSections`
- [ ] 1.4 Implementar helper functions para actions: `commitActions`, `releaseActions`, `genericActions`, `processingActions`

## Phase 2: Core Implementation — Modificar funciones JSON preview

- [ ] 2.1 Actualizar `readyJSON(preview string) string` para incluir `structured_preview` con `genericSections` y `genericActions`
-- Test: JSON debe incluir campo `structured_preview` con section "Operation" y "Preview"
-- Depende de 1.1 y 1.3

- [ ] 2.2 Actualizar `commitPlanJSON(plan *domain.OperationPlan) string` para incluir `structured_preview` con `commitSections`
-- Agregar campo `reasoning` con `plan.Reasoning`
-- Actualizar `options` a valores reales desde `commitActions`
-- Test: JSON debe incluir `structured_preview`, `reasoning`, y `options` con labels "Execute", "Regenerate", "Edit manually", "Cancel"
-- Depende de 1.1, 1.3 y 1.4

- [ ] 2.3 Actualizar `releasePlanJSON(intent *domain.ReleaseIntent, changelog string, warnings []string, ghAuth string) string` para incluir `structured_preview` con `releaseSections`
-- Agregar campo `impact` con valor calculado basado en warnings
-- Actualizar `options` a valores reales desde `releaseActions`
-- Test: JSON debe incluir `structured_preview`, `impact`, y `options` con labels "Execute", "Cancel"
-- Depende de 1.1, 1.3 y 1.4

- [ ] 2.4 Actualizar `processingJSON(message string) string` para incluir `structured_preview` simple con `processingSections`
-- `options` debe ser array vacío `[]`
-- Test: JSON debe incluir `structured_preview` con una section "Processing" y `actions` vacío
-- Depende de 1.1 y 1.3

## Phase 3: Testing & Coverage

1. [ ] 3.1 Actualizar tests existentes en `handlers_preview_test.go`
-- Agregar asserts para presencia de campo `structured_preview` en todos los tests
-- Verificar que `options` contiene valores reales según operation type
-- Mantener backward compatibility: tests existing deben pasar sin modificación

2. [ ] 3.2 Agregar tests unit específicos para nuevos campos en `handlers_preview_test.go`
-- Test `TestStructuredPreviewInCommitJSON`: verificar campo `reasoning` y sections correctas
-- Test `TestStructuredPreviewInReleaseJSON`: verificar campo `impact` y sections correctas
-- Test `TestOptionsMapping`: verificar que `options` labels corresponden con `structured_preview.actions`

3. [ ] 3.3 Verificar coverage mínimo 80% para funciones JSON modificadas
-- Ejecutar `go test -cover ./internal/delivery/mcp/...`
-- Añadir tests si coverage insuficiente

## Phase fresh1: Verification & Manual Test

- [ ] 4.1 Test manual con commit preview usando git-courer MCP
-- Ejecutar `COMMIT_START` en rama actual via `git_write_review`
-- Verificar que JSON response incluye `structured_preview` visible en terminal
-- Confimar que AI (este agente) mostraría todas las sections

- [ ] 4.2 Verificar integration path: `handleGitWriteReview` fase `start` retorna JSON con structured_preview
-- No modificar handler — solo verificar que JSON generado pasa por flujo
-- Ejecutar test de integración si existe

## Dependencies Critical Path

```
1.1 → 1.3 → 2.1 → 2.2 → 3.1
1.2 → 1.4 → 2.3 → 3.2
4.1 depende de 2.1-2.4 implementados
```

## Exit Criteria

- Campo `structured_preview` presente en TODOS JSON previews (ready, commit, release, processing)
- Campo `options` contiene valores reales según operation type (no placeholders)
- Tests existentes pasan sin regresión
- Tests nuevos cubren >80% funciones modificadas
- Manual test con `COMMIT_START` muestra structured sections completo
