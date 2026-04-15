# Proposal: Improve Commit and Release Workflows

## Intent

Actualizar los flujos de commit y release para:
1. **Commit**: Fix bug con preview mode y archivos eliminados
2. **Release**: Changelog estructurado para usuario final (Keep a Changelog), auto version bump, validación de cambios

El changelog es para usuarios finales, no desarrolladores — debe ser legible y enfocado en impacto.

## Scope

### In Scope
- Fix commit: preview no stagea archivos
- Fix commit: archivos eliminados (D) se stagean correctamente
- Release: changelog estructurado (Added, Changed, Fixed, Security)
- Release: auto-detectar version bump desde commits (feat→minor, fix→patch, BREAKING→major)
- Release: agregar fecha de release
- Release: detectar referencias a issues (#123)
- Release: validar que hay cambios antes de release

### Out of Scope
- Tests (separado)
- UI changes
- Nuevos comandos git

## Capabilities

### Modified Capabilities
- `commit-workflow`: Fix preview mode y deleted files
- `release-workflow`: Changelog estructurado, auto version bump, validación

## Approach

**Commit**: Agregar flag `preview` a `prepareStages()` - si preview=true, no ejecutar `git.Add()`, solo preparar decision y diff.

**Release**:
1. Parsear commits desde último tag
2. Clasificar por conventional commits (feat, fix, docs, BREAKING CHANGE)
3. Calcular version bump basado en tipos encontrados
4. Generar changelog en formato Keep a Changelog
5. Agregar fecha (YYYY-MM-DD)
6. Buscar patrones #\d+ para referencias
7. Validar que hay cambios antes de ejecutar

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/workflow/commit.go` | Modified | Agregar preview param, fix deleted files staging |
| `internal/workflow/release.go` | Modified | Nuevo changelog estructurado, version auto-detection |
| `internal/adapters/llm/ollama.go` | Modified | Actualizar prompt de changelog para formato estructurado |
| `internal/shared/prompts/txt/changelog_generate.txt` | Modified | Nuevo formato Keep a Changelog |
| `internal/core/domain/git.go` | Modified | Agregar ReleaseOutput type para changelog |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| LLM no genera formato exacto | Medium | Fallback a texto simple si falla parseo |
| Breaking change detection false positive | Low | Solo detectar "BREAKING CHANGE" explícito en commit body |

## Rollback Plan

1. Revertir cambios en prompts/txt
2. Revertir changes en commit.go y release.go
3. Recompilar y testear

## Success Criteria

- [ ] Commit con preview=true no stagea archivos
- [ ] Archivos con status "D " se incluyen en staging
- [ ] Changelog tiene formato Added/Changed/Fixed/Security
- [ ] Version bump correcto (feat→minor, fix→patch, BREAKING→major)
- [ ] Changelog incluye fecha YYYY-MM-DD
- [ ] Referencias a #123 detectadas y mostradas
- [ ] Release falla si no hay cambios desde último tag