# ai-usage

Consulta el estado de tus cuotas de **Codex, MiniMax y z.ai** desde una terminal pequeña o desde la barra de estado de [Tern](https://docs.stencil.so/tern/).

La aplicación consulta los proveedores configurados en paralelo, normaliza sus respuestas a un modelo común y muestra cuánto has consumido y cuándo se reinicia cada ventana. Este repositorio es un fork de [fuelcheck](https://github.com/emanuelarcos/fuelcheck).

## Qué ofrece

- **Tres proveedores:** Codex, MiniMax y z.ai. Solo se consultan los que declares y actives.
- **Salida compacta:** una línea horizontal si cabe o una fila por proveedor en terminales estrechas.
- **Indicadores por ventana:** consumo, cuenta atrás hasta el reinicio y colores según el porcentaje consumido.
- **Modo continuo:** `--watch` actualiza el widget con un intervalo configurable.
- **JSON normalizado:** `--json` facilita integrar las cuotas en scripts y otras herramientas.
- **Plugin de Tern:** muestra las cuotas en la barra inferior de las ventanas, sin ocupar un pane de terminal.
- **Configuración local:** permite activar proveedores, aportar credenciales, cambiar el intervalo y sobrescribir endpoints.

## Requisitos

- macOS o Linux, en arquitecturas `amd64` o `arm64`.
- Go **1.26.1 o posterior** para compilar desde el código.
- Una cuenta o suscripción compatible y credenciales válidas para cada proveedor que quieras consultar.
- Para el plugin, Tern de escritorio con soporte de plugins. La integración se ha comprobado con Tern 0.9.12.

## Instalación

### Con el instalador

Necesitas Git, Go 1.26.1 o posterior y `curl`. El instalador clona temporalmente este fork público, compila la CLI e instala `ai-usage` con `go install`; no necesita una release publicada ni modifica tu configuración de proveedores.

```bash
curl -fsSL https://raw.githubusercontent.com/fxescandell/fuelcheck/main/install.sh | sh
```

Para instalar una rama o etiqueta concreta, descarga el script y pásale su nombre:

```bash
curl -fsSLo install-ai-usage.sh https://raw.githubusercontent.com/fxescandell/fuelcheck/main/install.sh
sh install-ai-usage.sh v1.0.0
```

Sustituye `v1.0.0` por una etiqueta o rama que exista. El plugin de Tern se instala por separado, como se explica más abajo.

### Desde un clon

Si ya tienes el repositorio clonado, entra en su directorio y omite el primer comando:

```bash
git clone https://github.com/fxescandell/fuelcheck.git
cd fuelcheck

go install ./cmd/ai-usage
```

Go instala el ejecutable en el directorio indicado por `go env GOBIN`; si está vacío, utiliza el directorio `bin` de `go env GOPATH`. En una instalación estándar es `~/go/bin`. Asegúrate de que ese directorio esté en tu `PATH` y comprueba la instalación:

```bash
ai-usage --version
ai-usage --help
```

Si usas la ubicación estándar y tu shell no encuentra el ejecutable, puedes añadirla a tu configuración de shell:

```bash
export PATH="$HOME/go/bin:$PATH"
```

### Compilar sin instalar

Desde el directorio del repositorio:

```bash
go build -o ./ai-usage ./cmd/ai-usage
./ai-usage --help
./ai-usage
```

En este caso se ejecuta el binario del directorio actual; para que Tern pueda usarlo, instala también la CLI o ponla en una ruta que su proceso pueda encontrar.

> El instalador compila desde el código. La configuración de GoReleaser permite publicar archivos para macOS y Linux, `amd64` y `arm64`, cuando se cree una release.

## Configuración

La CLI y el plugin comparten el fichero:

- Por defecto: `~/.config/ai-usage/config.json`.
- Si está definida `XDG_CONFIG_HOME`: `$XDG_CONFIG_HOME/ai-usage/config.json`.

Crea el directorio y edita el fichero con tu editor. Si ya existe, **edítalo sin sobrescribir tus credenciales actuales**:

```bash
mkdir -p "${XDG_CONFIG_HOME:-$HOME/.config}/ai-usage"
```

Ejemplo usando credenciales autodetectadas:

```json
{
  "codex": { "enabled": true },
  "minimax": { "enabled": true },
  "z.ai": { "enabled": true },
  "interval_seconds": 60
}
```

Guarda el fichero con permisos privados, especialmente si contiene claves:

```bash
chmod 600 "${XDG_CONFIG_HOME:-$HOME/.config}/ai-usage/config.json"
```

| Campo | Uso |
| --- | --- |
| `codex`, `minimax`, `z.ai` | Declaración de cada proveedor. Omítelo o pon `"enabled": false` para no consultarlo. |
| `enabled` | Activa el proveedor cuando es `true`. |
| `api_key` | Credencial opcional. Una cadena no vacía tiene prioridad sobre la autodetección. |
| `endpoint` | URL opcional que sustituye la URL de consulta integrada para ese proveedor. |
| `interval_seconds` | Intervalo predeterminado de `--watch` y del plugin de Tern, en segundos enteros positivos. Por defecto: `60`. |

El fichero debe ser un objeto JSON válido. Sin fichero o sin proveedores activos, la CLI devuelve un error en vez de consultar todos los servicios. Si detecta credenciales de un proveedor no declarado, avisa por la salida de error, pero no lo consulta.

### Credenciales por proveedor

Una `api_key` escrita en `config.json` tiene prioridad en los tres casos. Si no la indicas, se usa la siguiente autodetección:

| Proveedor | Credenciales autodetectadas |
| --- | --- |
| **Codex** | `~/.codex/auth.json`; si no está disponible, `~/.config/codex/auth.json`. |
| **MiniMax** | El campo `api_key` de `~/.mmx/config.json`, utilizado por el cliente de MiniMax. |
| **z.ai** | La variable de entorno `ZAI_API_KEY`. Si no existe, se buscan asignaciones literales en `~/.zshrc`, `~/.zprofile`, `~/.bashrc` y `~/.profile`. |

Para definir una clave de z.ai en el entorno de la sesión actual:

```bash
export ZAI_API_KEY="TU_CLAVE_ZAI"
```

También puedes poner una clave directamente en la declaración correspondiente:

```json
{
  "minimax": { "enabled": true, "api_key": "TU_CLAVE_MINIMAX" }
}
```

En Codex, `api_key` debe ser un **token de acceso compatible con la API de cuotas**, no necesariamente una clave de la API de pago por uso. El token escrito manualmente no se renueva automáticamente. Cuando se usan las credenciales locales de Codex, la CLI intenta renovarlas si la API responde con un rechazo de autorización.

### Endpoints e intervalo

Normalmente no necesitas declarar `endpoint`. Las URL integradas son:

| Proveedor | Endpoint de consulta predeterminado |
| --- | --- |
| Codex | `https://chatgpt.com/backend-api/wham/usage` |
| MiniMax | `https://api.minimax.io/v1/token_plan/remains` |
| z.ai | `https://api.z.ai/api/monitor/usage/quota/limit` |

Puedes añadir `"endpoint": "https://tu-endpoint"` dentro de la declaración de un proveedor para pruebas o una instalación específica. La aplicación **envía la credencial a esa URL**: usa únicamente destinos de confianza y HTTPS, salvo pruebas locales controladas.

`interval_seconds` se aplica si `--watch` no recibe `--interval`; un flag explícito tiene prioridad. La CLI en `--watch` carga la configuración al iniciarse: reiníciala después de modificarla. El plugin de Tern la vuelve a leer durante sus ciclos de consulta.

## Uso de la CLI

```bash
ai-usage                         # proveedores activos en la configuración
ai-usage codex                   # solo Codex
ai-usage minimax z.ai            # seleccionar varios proveedores activos
ai-usage --layout auto           # horizontal si cabe; vertical si no
ai-usage --layout vertical       # una fila por proveedor
ai-usage --layout horizontal     # una línea, aunque no quepa
ai-usage --watch                 # consultar continuamente
ai-usage --watch --interval 30s  # intervalo explícito entre consultas
ai-usage --json                  # salida JSON normalizada
ai-usage --json --watch          # un objeto JSON por consulta
ai-usage --version
ai-usage --help
```

Los nombres de proveedores se indican sin distinguir mayúsculas de minúsculas, pero deben estar activados en `config.json`. El modo `auto` conserva la salida horizontal si no puede conocer el ancho del terminal. Usa `NO_COLOR=1 ai-usage` para desactivar los colores.

### Modo continuo

`--watch` realiza una consulta inmediatamente y espera el intervalo configurado **después** de cada consulta. `--interval` admite duraciones como `30s` o `2m` y debe ser mayor que cero.

En un terminal compatible, el widget se repinta en una pantalla alternativa que se restaura al salir. En salida redirigida, con `TERM=dumb` o con `--json`, cada actualización se añade a la salida sin secuencias de pantalla alternativa. Al pulsar Ctrl-C, la aplicación espera a que termine la consulta en curso para no interrumpir una posible renovación de credenciales de Codex.

### Cómo interpretar las cuotas

Los porcentajes indican **consumo**, no cuota restante. `C` es la ventana corta y `S` la semanal; un proveedor puede no tener ambas. La cuenta atrás indica cuándo se reinicia cada ventana, no cuándo empezó.

| Consumo | Color |
| --- | --- |
| Hasta el 30 % | Verde |
| Más del 30 % y hasta el 65 % | Amarillo |
| Más del 65 % | Rojo |

En la salida de terminal, el semáforo del proveedor representa su ventana finita de mayor consumo; los puntos adicionales representan cada ventana. Una cuota ilimitada o no disponible se muestra en gris. Los errores aparecen junto al proveedor afectado.

MiniMax devuelve porcentajes **restantes**, que se convierten a consumidos; Codex y z.ai ya entregan porcentajes consumidos. Cuando hay varias cuotas finitas del mismo tipo, se conserva la de mayor consumo junto con su propio reinicio y duración. El pool de herramientas `TIME_LIMIT` de z.ai se descarta.

### Salida JSON

```bash
ai-usage --json | jq '.Codex.windows'
```

La salida es un objeto por consulta, con un campo por proveedor como `Codex`, `MiniMax` o `z.ai`:

```json
{
  "Codex": {
    "ok": true,
    "windows": [
      {
        "kind": "corta",
        "used_percent": 25,
        "resets_at": "2026-11-07T12:00:00Z"
      }
    ]
  }
}
```

El ejemplo es ilustrativo. Cada proveedor tiene `ok`, `windows` y, si falla, `error`. Cada ventana contiene `kind` y `used_percent`; `resets_at` aparece si se conoce el reinicio, `duration` si se conoce la duración —en nanosegundos— y `unlimited` si la cuota es ilimitada. Un proveedor sin cuota devuelve `windows: []`.

Los errores JSON son genéricos para evitar exponer credenciales o contenido de respuestas; la vista de terminal aporta más detalle. No se incluyen los payloads originales. Con `--json --watch`, la salida completa es una secuencia de objetos JSON, no un único array.

La CLI termina con código `0` si al menos un proveedor devuelve datos y con código distinto de cero si ninguno lo hace o si la configuración es inválida. En `--watch`, al interrumpirla se usa el estado de la última consulta completada.

## Plugin de Tern

Si ya tienes un clon, instala la CLI y enlaza el plugin desde el directorio del repositorio:

```bash
go install ./cmd/ai-usage
tern plugin link ./tern-plugin
tern plugin list
```

El enlace deja el plugin en este repositorio: Tern carga sus archivos desde `tern-plugin/`. Para quitar el enlace:

```bash
tern plugin unlink ai-usage
```

También puedes instalar una copia del plugin directamente desde el fork público, después de instalar la CLI:

```bash
tern plugin install github.com/fxescandell/fuelcheck/tern-plugin
```

Para desinstalar esa copia usa `tern plugin remove ai-usage`.

El plugin muestra en la **barra inferior** de cada ventana un grupo por proveedor, separado por `│`, con las ventanas disponibles `C` y `S`, sus porcentajes consumidos y sus colores. Cuando hay espacio, un reloj y una cuenta atrás indican el reinicio. En ventanas de hasta 900 px de ancho se ocultan las cuentas atrás para priorizar los porcentajes.

La barra de estado de Tern debe estar visible. Si no aparece, ejecuta **Show status line** desde la paleta de Tern.

### Controles

- Pulsa **Cuotas** en la propia barra inferior para ocultar o mostrar los datos de esa ventana. El control sigue disponible cuando los datos están ocultos.
- También puedes usar el icono de gráfica de la cabecera de **Archivos**. La visibilidad se cambia por ventana.
- Pulsa un porcentaje o una cuenta atrás para abrir `ai-usage --watch` en otro pane.
- Desde la paleta de Tern puedes ejecutar **Mostrar u ocultar AI Usage**, **Abrir detalle de AI Usage** o **Abrir ajustes de AI Usage**. Este último abre el fichero `config.json` en Tern.

### Funcionamiento del plugin

La mitad *host* del plugin ejecuta `ai-usage --json` en segundo plano y comparte el resultado con las ventanas mediante el almacenamiento del plugin de Tern. Se realiza un sondeo por daemon, no uno por ventana; el daemon puede seguir consultando mientras esté activo aunque no haya ventanas abiertas. El intervalo por defecto es de 60 segundos y se ajusta mediante `interval_seconds`.

Cada ventana actualiza sus segmentos a partir de ese estado compartido. Si la CLI no produce una respuesta JSON válida, se conserva el último resultado disponible y se muestra un aviso; si los datos superan el doble del intervalo configurado, se marcan como antiguos. Una respuesta JSON válida sustituye la anterior, incluso cuando contiene errores: estos aparecen en los segmentos de los proveedores afectados.

El proceso busca el ejecutable primero en `~/go/bin/ai-usage`, luego en su `PATH` y después en ubicaciones habituales como `~/.local/bin`, `/opt/homebrew/bin` y `/usr/local/bin`. Si tienes varias copias, ten en cuenta ese orden al actualizar. El plugin de ventana necesita ejecutarse en Tern de escritorio; la vista web de Tern no ejecuta plugins de ventana.

## Solución de problemas

- **No se encuentra `ai-usage`:** comprueba `go env GOBIN`, `go env GOPATH` y el `PATH`; instala con `go install ./cmd/ai-usage` desde el repositorio. Tern puede tener un `PATH` distinto al de tu shell.
- **No hay proveedores activos o falta el fichero:** crea `config.json` en la ruta XDG indicada arriba y declara al menos un proveedor con `"enabled": true`.
- **Falla un proveedor:** comprueba su credencial y ejecuta `ai-usage codex`, `ai-usage minimax` o `ai-usage z.ai` para ver el detalle en terminal. El argumento elegido también debe estar activado en el fichero.
- **Codex rechaza un token configurado:** sustitúyelo en `config.json` o utiliza las credenciales del CLI de Codex. Un token configurado manualmente no se renueva.
- **No aparecen cuotas en Tern:** comprueba `tern plugin list`, activa **Show status line** en la paleta y pulsa **Cuotas** si están ocultas. Si la CLI devuelve datos pero el plugin no los encuentra, revisa qué copia del ejecutable se está usando.
- **La barra muestra datos antiguos o un error:** comprueba la conexión, las credenciales y la salida de la CLI. La siguiente consulta usa el intervalo configurado; también puedes recargar el plugin con `tern plugin reload`.

## Estructura y desarrollo

- `cmd/ai-usage/`: comando CLI y opciones.
- `internal/auth/`: descubrimiento y renovación de credenciales.
- `internal/config/`: lectura y validación de la configuración.
- `internal/providers/`: clientes de API y normalización de cuotas.
- `internal/ui/`: salida de terminal, adaptación al ancho y JSON.
- `tern-plugin/`: manifiesto, sondeo, segmentos de Tern y estilos.
- `.github/workflows/` y `.goreleaser.yml`: CI y configuración de releases.

Comprobaciones habituales desde el repositorio:

```bash
go test ./...
go vet ./...
go build -o ./ai-usage ./cmd/ai-usage
```

## Aviso y licencia

La herramienta usa endpoints de proveedores que pueden cambiar sin aviso. No está afiliada ni respaldada por OpenAI, MiniMax ni z.ai.

Licencia: [MIT](LICENSE).
