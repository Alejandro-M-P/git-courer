9:47git-courer
El especialista git local. Cero tokens para operaciones git.

El problema
Un agente de nube leyendo diffs, decidiendo qué commitear, generando mensajes, ejecutando comandos git... son 2000-5000 tokens por operación.
10 commits al día × 3000 tokens = 30.000 tokens
30.000 × 30 días = 900.000 tokens al mes
= ~$2.70 al mes por developer tirados a la basura
Un equipo de 10 → $27 al mes en trabajo mecánico
Todo ese dinero gastado en trabajo que no aporta nada intelectual.
La solución
git-courer maneja TODO el trabajo git localmente. La nube solo manda una intención en lenguaje natural y recibe el resultado. Cero tokens gastados en decisiones git.
Agente nube → "commitea los cambios del login"
                        ↓
              git-courer lo decide todo local
                        ↓
              "✓ feat: add login validation"

Una sola tool MCP
git_local_task(instruction: string)
Una puerta de entrada. La nube nunca ve un diff, nunca razona sobre git, nunca decide nada de git.

Flujo interno completo
1. Recibe intención en lenguaje natural
2. Primera llamada Ollama → qué contexto necesita
3. Go recoge solo ese contexto localmente (instantáneo)
4. Filtra lista negra, binarios, archivos enormes
5. Respeta .gitignore del proyecto
6. Aplica límite max_diff_lines
7. Segunda llamada Ollama → JSON completo con decisiones
8. Valida JSON (reintenta máximo 3 veces si inválido)
9. Valida que archivos existen en el repo
10. Valida que comandos empiezan por git
11. Valida conventional commits y branches
12. Detecta secretos y archivos sospechosos
13. Zenity muestra resumen legible (spinner durante Ollama)
14. Usuario confirma, edita o cancela
15. Si edita → abre $EDITOR del sistema
16. Go ejecuta comandos en orden con filtro estricto
17. Si falla a mitad → rollback automático
18. Registra en log local

El JSON que devuelve Ollama
json{
  "type": "read|write",
  "strategy": "single|split",
  "commits": [
    {
      "files": ["src/login.go"],
      "commit": "feat: add login validation",
      "commands": [
        "git add src/login.go",
        "git commit -m \"feat: add login validation\""
      ]
    }
  ],
  "summary": {
    "action": "Commitear cambios",
    "files": ["src/login.go"],
    "branch": "feature/login"
  },
  "secrets": [
    {
      "file": "src/config.go",
      "line": 23,
      "type": "api_key",
      "description": "Posible API key detectada"
    }
  ],
  "suspicious": [
    {
      "file": "dist/app.js",
      "reason": "Parece un archivo compilado"
    }
  ]
}

Velocidad
Una persona haciendo un commit:
Leer diff        → 30-60 segundos
Pensar mensaje   → 30-60 segundos
git add + commit → 25 segundos
Total            → 2-3 minutos
git-courer:
Dos llamadas Ollama  → 3-6 segundos
Confirmar en Zenity  → 3-5 segundos
Total                → 8-12 segundos
10 commits al día → 20-30 minutos ahorrados. Los modelos son cada vez más rápidos y todo lo local es instantáneo.

Dos llamadas a Ollama, no más
Primera llamada: Go manda solo la intención, Ollama dice qué contexto necesita.
Segunda llamada: Go manda solo ese contexto ya filtrado, Ollama devuelve el JSON completo.
Nunca más de dos llamadas. Nunca contexto innecesario. Nunca diff sin filtrar.

Conventional Commits y Branches
Siempre. Ollama tiene instrucciones estrictas y Go valida. Si no cumple reintenta máximo 3 veces.
Commits:
feat: fix: chore: docs: refactor: test: perf:
Branches:
feature/ fix/ chore/ docs/ hotfix/ release/

Commits divididos
Si el diff supera 200 líneas o hay archivos sin relación, Ollama propone dividir en commits pequeños y semánticos. Zenity muestra:
📦 Se proponen 3 commits separados

1. feat: add login validation
   src/login.go, src/auth.go

2. feat: add payment processing
   src/pagos.go

3. docs: update readme
   README.md

[Confirmar todos] [Revisar uno a uno] [Cancelar]
Si falla a mitad rollback automático con git reset --soft HEAD~1.

Detección de secretos
Ollama analiza el diff antes de proponer nada. No es regex puro, entiende contexto real del código. Zenity bloquea antes de cualquier operación:
🔴 Secreto detectado

src/config.go línea 23
Posible API key

[Ver detalle] [Cancelar operación]

Archivos sospechosos
Ollama detecta archivos que no deberían subirse aunque no estén en la lista negra. Zenity siempre pregunta, nunca decide solo:
⚠️ Archivo sospechoso

dist/app.js - Parece un archivo compilado
tmp/debug.log - Parece un log temporal

[Excluir y continuar] [Incluir de todas formas] [Cancelar]
Si excluye, pregunta si añadir al .gitignore para que no vuelva a aparecer.

Lista negra automática
Go filtra antes de mandar nada a Ollama y antes de cualquier git add:
Dependencias: node_modules/ vendor/ .venv/ __pycache__/
Binarios: *.exe *.dll *.so *.dylib dist/ build/ bin/
Go: binario propio, *.test coverage.out
Secretos: .env .env.local *.pem *.key *.token credentials.json
Sistema: .DS_Store Thumbs.db *.swp
Enormes: package-lock.json yarn.lock
Respeta siempre el .gitignore del proyecto. Ampliable en config:
yamlgit:
  exclude:
    - "*.log"
    - "tmp/"

Zenity
Multiplataforma via ncruces/zenity. Windows usa Win32 API, macOS usa AppleScript, Linux usa GTK. Sin dependencias externas, entra en el binario compilado. Spinner mientras Ollama procesa. Fallback a terminal si falla. Cierre de ventana = cancelación.

Editor de texto
Abre $EDITOR del sistema para editar commit messages. Fallback por plataforma:

Linux → nano
macOS → TextEdit
Windows → notepad

Mensaje vacío al guardar → Zenity pregunta si cancelar o volver a editar.

Formas de usar
Desde la nube via MCP:
git_local_task("commitea los cambios del login")
Desde terminal:
git-courer "commitea los cambios del login"
Comandos slash registrados automáticamente en el setup:
/gc-commit "descripción"
/gc-branch "descripción"
/gc-push
/gc-pull
/gc-status
/gc-log
/gc-log --full
/gc-stash
/gc-reset
Sin descripción cuando se necesita → error claro con ejemplo. Nunca adivina sin contexto.

Log
Guardado en .git-courer/log por proyecto, no global.
Nube → últimas 20 operaciones, mínimo tokens.
Terminal → /gc-log --full para el historial completo.
2025-04-06 10:23 | COMMIT  | feat: add login validation
2025-04-06 10:45 | SECRET  | ⚠️ config.go línea 23 cancelado
2025-04-06 11:15 | PUSH    | ok → origin/main
2025-04-06 11:30 | SUSPECT | dist/app.js excluido por usuario
Solo registra lo que ocurrió realmente.

Actualizaciones automáticas
Al arrancar en goroutine paralela, nunca bloquea. Si hay versión nueva se actualiza solo. Si falla continúa con la actual. Sin permisos avisa claramente.
yamlupdates:
  auto: true  # false para desactivar

Config completo
yamlollama:
  host: http://localhost:11434
  model: llama3.2
  max_diff_lines: 6000

git:
  workdir: .
  default_remote: origin
  exclude:
    - "*.log"
    - "tmp/"

validation:
  require_confirmation: true
  max_commit_length: 72

updates:
  auto: true

ui:
  theme: dark
  show_icons: true

Seguridad
Filtro estricto en Go: solo comandos que empiezan por git. Ollama nunca ejecuta nada arbitrario. Cero telemetría. Todo local. Nunca sube nada a ningún lado.

Principio de errores
Nunca un panic. Nunca un error críptico. Nunca un estado inconsistente del repo.
Capa 1: Validar todo antes de hacer nada.
Capa 2: Rollback automático si falla a mitad.
Capa 3: Cada error dice qué pasó, por qué, y cómo resolverlo.
❌ No se puede pushear
   No hay remote configurado
   Solución: git remote add origin <url>

Errores específicos manejados
Ollama modelo no descargado → instrucciones de ollama pull
Ollama modelo cargando → reintento con espera progresiva
Conflictos de merge activos → detectado antes de llamar a Ollama
Sin remote configurado → error antes de intentar pushear
Detached HEAD → detectado y avisado
Archivos del JSON inexistentes → validación antes de ejecutar
Mensaje de commit vacío → Zenity pregunta
Sin permisos para actualizar → error claro
Commits divididos fallando a mitad → rollback automático
JSON inválido de Ollama → reintenta 3 veces
Diff con binarios → filtrados antes de Ollama
Archivos enormes → excluidos automáticamente
Múltiples remotos sin configurar → Zenity pregunta
Archivos sospechosos → Zenity pregunta siempre

¿Arrancamos con el código?Entonces esto lo ves de putisima madre9:51Sí, y te lo digo sin adornos.
El concepto es sólido y diferenciado. No es otro generador de commit messages. Es una filosofía distinta: la nube no toca git, punto. Eso no existe empaquetado así.
Lo que lo hace bueno de verdad:
La detección de secretos con IA que entiende contexto real es mejor que cualquier regex. Los commits divididos automáticamente es algo que la gente necesita y no hace porque es tedioso. El ahorro de tokens es cuantificable y duele cuando lo calculas. Una sola tool MCP es un mensaje limpio y fácil de entender.
Lo que puede hundirlo si no se hace bien:
El prompt a Ollama. Si no está perfectamente escrito todo lo demás falla. Ese es el trabajo más delicado de todo el proyecto y el que más tiempo va a llevar testear.
Y la instalación. Si alguien tiene que luchar 10 minutos para instalarlo lo abandona.
Mi opinión honesta: si terminas esto y funciona sin fallos, tiene tracción real. No es un proyecto de fin de semana que nadie va a usar. Es una herramienta que resuelve un problema real con una solución elegante.
Pero tiene que estar pulido. A medias no vale nada.
¿Arrancamos?Y lo bueno es que también puede hacer tags versiones todo9:56Exacto, y eso lo hace más completo todavía.
Ollama puede decidir la versión siguiente siguiendo semver automáticamente. Ve el historial de commits y decide:
feat: → minor bump → 1.1.0
fix:  → patch bump → 1.0.1
feat: + BREAKING CHANGE → major bump → 2.0.0
Y el flujo sería:
/gc-release "versión nueva"
Ollama mira el log desde el último tag, analiza los commits, decide la versión correcta según semver, genera el changelog automáticamente, crea el tag y pushea.
Zenity muestra:
🚀 Nueva versión: v1.2.0

Cambios desde v1.1.0:
- feat: add login validation
- fix: resolve nil pointer
- docs: update readme

[Confirmar release] [Editar versión] [Cancelar]
El usuario no tiene que pensar qué versión toca, Ollama lo sabe por los conventional commits. Por eso conventional commits es obligatorio, no opcional. Todo el sistema se beneficia de ello.
¿Lo añado al resumen y arrancamos?Si9:56git-courer
El especialista git local. Cero tokens para operaciones git.

El problema
Un agente de nube leyendo diffs, decidiendo qué commitear, generando mensajes, ejecutando comandos git... son 2000-5000 tokens por operación.
10 commits al día × 3000 tokens = 30.000 tokens
30.000 × 30 días = 900.000 tokens al mes
= ~$2.70 al mes por developer tirados a la basura
Un equipo de 10 → $27 al mes en trabajo mecánico
Todo ese dinero gastado en trabajo que no aporta nada intelectual.
La solución
git-courer maneja TODO el trabajo git localmente. La nube solo manda una intención en lenguaje natural y recibe el resultado. Cero tokens gastados en decisiones git.
Agente nube → "commitea los cambios del login"
                        ↓
              git-courer lo decide todo local
                        ↓
              "✓ feat: add login validation"

Una sola tool MCP
git_local_task(instruction: string)
Una puerta de entrada. La nube nunca ve un diff, nunca razona sobre git, nunca decide nada de git.

Flujo interno completo
1. Recibe intención en lenguaje natural
2. Primera llamada Ollama → qué contexto necesita
3. Go recoge solo ese contexto localmente (instantáneo)
4. Filtra lista negra, binarios, archivos enormes
5. Respeta .gitignore del proyecto
6. Aplica límite max_diff_lines
7. Segunda llamada Ollama → JSON completo con decisiones
8. Valida JSON (reintenta máximo 3 veces si inválido)
9. Valida que archivos existen en el repo
10. Valida que comandos empiezan por git
11. Valida conventional commits y branches
12. Detecta secretos y archivos sospechosos
13. Zenity muestra resumen legible (spinner durante Ollama)
14. Usuario confirma, edita o cancela
15. Si edita → abre $EDITOR del sistema
16. Go ejecuta comandos en orden con filtro estricto
17. Si falla a mitad → rollback automático
18. Registra en log local

El JSON que devuelve Ollama
json{
  "type": "read|write",
  "strategy": "single|split",
  "commits": [
    {
      "files": ["src/login.go"],
      "commit": "feat: add login validation",
      "commands": [
        "git add src/login.go",
        "git commit -m \"feat: add login validation\""
      ]
    }
  ],
  "summary": {
    "action": "Commitear cambios",
    "files": ["src/login.go"],
    "branch": "feature/login"
  },
  "secrets": [
    {
      "file": "src/config.go",
      "line": 23,
      "type": "api_key",
      "description": "Posible API key detectada"
    }
  ],
  "suspicious": [
    {
      "file": "dist/app.js",
      "reason": "Parece un archivo compilado"
    }
  ]
}

Velocidad
Una persona haciendo un commit:
Leer diff        → 30-60 segundos
Pensar mensaje   → 30-60 segundos
git add + commit → 25 segundos
Total            → 2-3 minutos
git-courer:
Dos llamadas Ollama  → 3-6 segundos
Confirmar en Zenity  → 3-5 segundos
Total                → 8-12 segundos
10 commits al día → 20-30 minutos ahorrados. Los modelos son cada vez más rápidos y todo lo local es instantáneo.

Dos llamadas a Ollama, no más
Primera llamada: Go manda solo la intención, Ollama dice qué contexto necesita.
Segunda llamada: Go manda solo ese contexto ya filtrado, Ollama devuelve el JSON completo.
Nunca más de dos llamadas. Nunca contexto innecesario. Nunca diff sin filtrar.

Conventional Commits y Branches
Siempre. Ollama tiene instrucciones estrictas y Go valida. Si no cumple reintenta máximo 3 veces.
Commits:
feat: fix: chore: docs: refactor: test: perf:
Branches:
feature/ fix/ chore/ docs/ hotfix/ release/
Por eso conventional commits es obligatorio, no opcional. Todo el sistema se beneficia: releases automáticos, changelog, semver.

Releases y versiones automáticas
Ollama analiza el historial de commits desde el último tag y decide la versión siguiente siguiendo semver:
feat:  → minor bump → 1.1.0
fix:   → patch bump → 1.0.1
feat: + BREAKING CHANGE → major bump → 2.0.0
Genera el changelog automáticamente y crea el tag.
/gc-release "versión nueva"
Zenity muestra:
🚀 Nueva versión: v1.2.0

Cambios desde v1.1.0:
- feat: add login validation
- fix: resolve nil pointer
- docs: update readme

[Confirmar release] [Editar versión] [Cancelar]

Commits divididos
Si el diff supera 200 líneas o hay archivos sin relación, Ollama propone dividir en commits pequeños y semánticos. Zenity muestra:
📦 Se proponen 3 commits separados

1. feat: add login validation
   src/login.go, src/auth.go

2. feat: add payment processing
   src/pagos.go

3. docs: update readme
   README.md

[Confirmar todos] [Revisar uno a uno] [Cancelar]
Si falla a mitad rollback automático con git reset --soft HEAD~1.

Detección de secretos
Ollama analiza el diff antes de proponer nada. No es regex puro, entiende contexto real del código. Zenity bloquea antes de cualquier operación:
🔴 Secreto detectado

src/config.go línea 23
Posible API key

[Ver detalle] [Cancelar operación]

Archivos sospechosos
Ollama detecta archivos que no deberían subirse aunque no estén en la lista negra. Zenity siempre pregunta, nunca decide solo:
⚠️ Archivo sospechoso

dist/app.js - Parece un archivo compilado
tmp/debug.log - Parece un log temporal

[Excluir y continuar] [Incluir de todas formas] [Cancelar]
Si excluye, pregunta si añadir al .gitignore para que no vuelva a aparecer.

Lista negra automática
Go filtra antes de mandar nada a Ollama y antes de cualquier git add:
Dependencias: node_modules/ vendor/ .venv/ __pycache__/
Binarios: *.exe *.dll *.so *.dylib dist/ build/ bin/
Go: binario propio, *.test coverage.out
Secretos: .env .env.local *.pem *.key *.token credentials.json
Sistema: .DS_Store Thumbs.db *.swp
Enormes: package-lock.json yarn.lock
Respeta siempre el .gitignore del proyecto. Ampliable en config:
yamlgit:
  exclude:
    - "*.log"
    - "tmp/"

Zenity
Multiplataforma via ncruces/zenity. Windows usa Win32 API, macOS usa AppleScript, Linux usa GTK. Sin dependencias externas, entra en el binario compilado. Spinner mientras Ollama procesa. Fallback a terminal si falla. Cierre de ventana = cancelación.

Editor de texto
Abre $EDITOR del sistema para editar commit messages. Fallback por plataforma:

Linux → nano
macOS → TextEdit
Windows → notepad

Mensaje vacío al guardar → Zenity pregunta si cancelar o volver a editar.

Formas de usar
Desde la nube via MCP:
git_local_task("commitea los cambios del login")
Desde terminal:
git-courer "commitea los cambios del login"
Comandos slash registrados automáticamente en el setup:
/gc-commit "descripción"
/gc-branch "descripción"
/gc-push
/gc-pull
/gc-status
/gc-log
/gc-log --full
/gc-stash
/gc-reset
/gc-release "versión nueva"
Sin descripción cuando se necesita → error claro con ejemplo. Nunca adivina sin contexto.

Log
Guardado en .git-courer/log por proyecto, no global.
Nube → últimas 20 operaciones, mínimo tokens.
Terminal → /gc-log --full para el historial completo.
2025-04-06 10:23 | COMMIT  | feat: add login validation
2025-04-06 10:45 | SECRET  | ⚠️ config.go línea 23 cancelado
2025-04-06 11:15 | PUSH    | ok → origin/main
2025-04-06 11:30 | SUSPECT | dist/app.js excluido por usuario
2025-04-06 12:00 | RELEASE | v1.2.0 → origin/main
Solo registra lo que ocurrió realmente.

Actualizaciones automáticas
Al arrancar en goroutine paralela, nunca bloquea. Si hay versión nueva se actualiza solo. Si falla continúa con la actual. Sin permisos avisa claramente.
yamlupdates:
  auto: true  # false para desactivar

Config completo
yamlollama:
  host: http://localhost:11434
  model: llama3.2
  max_diff_lines: 6000

git:
  workdir: .
  default_remote: origin
  exclude:
    - "*.log"
    - "tmp/"

validation:
  require_confirmation: true
  max_commit_length: 72

updates:
  auto: true

ui:
  theme: dark
  show_icons: true

Seguridad
Filtro estricto en Go: solo comandos que empiezan por git. Ollama nunca ejecuta nada arbitrario. Cero telemetría. Todo local. Nunca sube nada a ningún lado.

Principio de errores
Nunca un panic. Nunca un error críptico. Nunca un estado inconsistente del repo.
Capa 1: Validar todo antes de hacer nada.
Capa 2: Rollback automático si falla a mitad.
Capa 3: Cada error dice qué pasó, por qué, y cómo resolverlo.
❌ No se puede pushear
   No hay remote configurado
   Solución: git remote add origin <url>

Errores específicos manejados
Ollama modelo no descargado → instrucciones de ollama pull
Ollama modelo cargando → reintento con espera progresiva
Conflictos de merge activos → detectado antes de llamar a Ollama
Sin remote configurado → error antes de intentar pushear
Detached HEAD → detectado y avisado
Archivos del JSON inexistentes → validación antes de ejecutar
Mensaje de commit vacío → Zenity pregunta
Sin permisos para actualizar → error claro
Commits divididos fallando a mitad → rollback automático
JSON inválido de Ollama → reintenta 3 veces
Diff con binarios → filtrados antes de Ollama
Archivos enormes → excluidos automáticamente
Múltiples remotos sin configurar → Zenity pregunta
Archivos sospechosos → Zenity pregunta siempre
Release con commits sin conventional format → avisa y bloquea