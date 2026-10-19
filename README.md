# Pluton SQL

Herramienta de escritorio para administrar bases de datos SQL, escrita en Go ([Wails v2](https://wails.io)) con interfaz Svelte + TypeScript.

- **Motores:** PostgreSQL, MySQL/MariaDB, SQLite y SQL Server.
- **Plataformas:** Linux (Wayland y X11) y Windows.
- **Funciones:** conexiones guardadas, explorador de esquema (bases, esquemas, tablas, columnas) y editor de consultas con resultados en tabla.

## Requisitos

| Herramienta | Versión |
|---|---|
| Go | 1.25 o superior |
| Node.js + npm | 20 o superior |
| Wails CLI | v2 |

Instalar el CLI de Wails:

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Asegúrate de que `~/go/bin` está en tu `PATH`.

### Linux

Se necesitan GTK3 y WebKitGTK 4.1.

```bash
# Fedora
sudo dnf install -y gtk3-devel webkit2gtk4.1-devel gcc-c++ pkg-config

# Debian / Ubuntu
sudo apt install -y libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config
```

Todos los comandos de Wails en Linux llevan `-tags webkit2_41` para usar WebKitGTK 4.1.

> `wails doctor` puede marcar `libwebkit` como no encontrado porque busca la versión 4.0. Si `pkg-config --modversion webkit2gtk-4.1` responde, es un falso negativo.

### Windows

Basta con Go, Node.js y el CLI de Wails. WebView2 viene incluido en Windows 10 y 11.

## Compilar

```bash
git clone https://github.com/SrChr1s/pluton-sql.git
cd pluton-sql

make build            # Linux (usa -tags webkit2_41)
make build-windows    # Windows amd64 (cross-compilando desde Linux, o nativo)
```

El binario queda en `build/bin/` (`pluton-sql` o `pluton-sql.exe`). Los drivers de base de datos son Go puro, así que no hace falta CGO para la compilación cruzada.

`make` primero genera un parche de `crypto/tls` (`tools/tlsoverlay`) y compila con `go build -overlay`. **No modifica tu instalación de Go.** El parche desactiva la mitigación BEAST de Go (un primer registro TLS de 1 byte) que SQL Server 2005/2008 no tolera con TLS 1.0 y corta la conexión. Solo afecta a conexiones TLS 1.0 con cifrado CBC. Si compilas con `wails build` a secas, todo funciona excepto conectar a esos SQL Server antiguos con TLS.

Si `wails` no está en tu `PATH`: `make build WAILS=$HOME/go/bin/wails`.

## Ejecutar

```bash
./build/bin/pluton-sql
```

Modo desarrollo, con recarga en caliente del frontend:

```bash
make dev
```

### Linux: Wayland, X11 y NVIDIA

- En Wayland funciona de forma nativa. Para forzar X11: `GDK_BACKEND=x11 ./build/bin/pluton-sql`.
- Con GPU NVIDIA, WebKitGTK puede mostrar la ventana en blanco. La aplicación define `WEBKIT_DISABLE_DMABUF_RENDERER=1` al arrancar para evitarlo. Si tú ya definiste esa variable, se respeta tu valor.

## Uso

1. Pulsa **＋** en el panel *Conexiones* para crear una conexión: nombre, motor, host, puerto, usuario, contraseña y base de datos. Para SQLite, indica la ruta del archivo `.db`.
2. Usa **Probar** para verificar los datos y **Guardar** para guardarla.
3. Haz clic en una conexión para activarla. El árbol de esquema se carga bajo demanda: base de datos → esquema → tabla → columnas.
4. Un clic en una tabla del árbol abre sus datos en una pestaña editable (la flecha de la izquierda despliega sus columnas): paginación, orden (clic en la cabecera), filtro `WHERE` y recuento (**Contar**). **SQL** abre un `SELECT` de esa tabla como consulta.
5. Para editar datos: doble clic (o `Enter`) en una celda, `Tab` pasa a la siguiente, `Esc` cancela y el botón **NULL** pone el valor nulo. **+ Fila** añade una fila nueva (las columnas que dejes vacías usan su valor por defecto) y el icono de papelera marca una fila para borrar.
   Nada se escribe hasta pulsar **Aplicar** (`Ctrl+S`): todos los cambios van en **una sola transacción**, y si uno falla no se aplica ninguno. **Descartar** los deshace. Al cambiar de página, orden o filtro, o al cerrar la pestaña con cambios pendientes, pide confirmación.
   Las vistas son de solo lectura. Si una tabla no tiene clave primaria, las filas se identifican por todas sus columnas y cada cambio debe afectar **exactamente una fila**; con filas idénticas se rechaza. Las columnas binarias no son editables.
6. **Estructura:** el selector *Datos / Estructura* de cada tabla (o el icono de capas que aparece al pasar el ratón por una tabla del árbol) muestra las columnas con su tipo completo, `PK` y `AUTO`, los índices, las claves foráneas (con enlace a la tabla referenciada) y el **DDL** (`CREATE TABLE`), que puedes copiar o abrir en una consulta sin ejecutarlo. En PostgreSQL y SQL Server el DDL se reconstruye desde el catálogo y puede omitir triggers y opciones de almacenamiento (en SQL Server, también los `CHECK`).
7. **Modificar la estructura:** en la vista *Estructura* puedes añadir, modificar, renombrar y eliminar columnas (lápiz de cada fila), crear y eliminar índices, añadir y eliminar claves foráneas, y renombrar o eliminar la tabla. Para crear una tabla nueva, pasa el ratón por la base de datos o el esquema en el árbol y pulsa **+**.
   Cada cambio muestra primero el **SQL exacto** que se va a ejecutar, con avisos (operaciones destructivas, datos que pueden no encajar…), y solo se ejecuta al confirmar. En PostgreSQL, SQLite y SQL Server las sentencias van en una transacción; **MySQL no admite DDL transaccional** y lo avisa. SQLite no permite con `ALTER TABLE` modificar columnas ni añadir o borrar claves foráneas: lo dice con un mensaje claro en vez de recrear la tabla en silencio.
8. En una pestaña de consulta, escribe SQL y pulsa **Ctrl+Enter** (o **▶ Ejecutar**) para ejecutar la selección, o todo el texto si no hay selección. **■ Cancelar** interrumpe una consulta en curso.
9. **Archivos .sql:** **Abrir** (`Ctrl+O`) carga un script en una pestaña nueva y **Guardar** (`Ctrl+S`) lo escribe en su archivo; **Guardar como…** (`Ctrl+Shift+S`) elige otro destino. La pestaña muestra un punto si tiene cambios sin guardar y pide confirmación al cerrarla. Se conserva el fin de línea del archivo (CRLF o LF) y la escritura es atómica. Solo se abren archivos de texto UTF-8 de hasta 10 MB.
10. **Historial y consultas guardadas** (`Ctrl+H`): cada ejecución de la conexión queda registrada con su duración, filas y error si lo hubo; las ejecuciones consecutivas idénticas se agrupan (`×3`). Haz clic en una entrada para abrirla en una pestaña nueva, o guárdala con un nombre en *Guardadas* (sirven para cualquier conexión).
11. **Exportar:** el botón *Exportar* de una consulta o de una tabla genera **CSV**, **JSON** o **INSERT**. Se vuelve a ejecutar la consulta en el servidor y se exportan **todas** las filas (no solo las 1000 de la vista), escribiendo en streaming; el destino solo se reemplaza si la exportación termina entera, y cancelar no deja archivos a medias. En una tabla se respetan el filtro y el orden actuales. El CSV puede usar `;`, tabulador o `|` y un BOM para que Excel lea bien los acentos.
12. **Importar CSV:** el botón *Importar* de una tabla abre un asistente con vista previa, detección del separador y mapeo de columnas (por nombre o por posición). Todo va en **una transacción**: si falla una fila, se indica cuál y no se inserta ninguna; cancelar también lo revierte. En columnas que no son de texto el valor vacío se importa como `NULL`.
    - Los valores binarios se exportan como texto si son UTF-8 válido y en hexadecimal si no.
    - Un CSV abierto en una hoja de cálculo puede interpretar como fórmula una celda que empiece por `=`, `+`, `-` o `@`: no abras con Excel archivos exportados de datos que no controlas.
13. **Autocompletado:** el editor sugiere tablas, vistas y columnas de la conexión mientras escribes (o con `Ctrl+Espacio`), y entiende el contexto: tras `FROM`/`JOIN`/`INTO`/`UPDATE` ofrece tablas (las de otros esquemas cualificadas), tras `alias.` las columnas de esa tabla —aunque el alias se declare después, en el `FROM`—, y en el resto las columnas de las tablas de la consulta, indicando de cuál es cada una. Cita los nombres que lo necesitan (mayúsculas en PostgreSQL, espacios, palabras reservadas) con las comillas de cada motor. No sugiere dentro de cadenas ni comentarios. El índice se carga una vez por base de datos y se actualiza al cambiar la estructura, al ejecutar un `CREATE`/`ALTER`/`DROP` o con *Recargar esquema*. No entiende subconsultas ni CTE: ahí simplemente no sugiere.
14. Edita una conexión con **✎** o doble clic, y elimínala con **✕**.
15. **Workspaces:** la barra de iconos de la izquierda agrupa las conexiones guardadas (p. ej. *Producción*, *Cliente X*, *Local*). Cada workspace es un botón con su inicial y un color propio; **＋** crea uno nuevo, el icono de cuadrícula (*Todas las conexiones*) muestra todo, y con clic derecho en un workspace puedes renombrarlo o eliminarlo. Asigna cada conexión desde su formulario o **arrastrándola** desde la lista hasta el workspace (o hasta *Todas las conexiones* para dejarla sin workspace); mover una conexión en uso no cierra su sesión. Al cambiar de workspace se recuerda cuál era la conexión activa de cada uno y se recupera al volver (con sus pestañas); los pools de la conexión que dejas siguen abiertos hasta que la desconectes. Al eliminar un workspace sus conexiones no se borran: quedan sin workspace.

Los resultados de una consulta se limitan a 1000 filas; la barra inferior avisa si se truncaron.

### Dónde se guardan los datos

- **Conexiones:** `connections.json` en el directorio de configuración del usuario (`~/.config/pluton-sql/` en Linux, `%AppData%\pluton-sql\` en Windows). No contiene contraseñas.
- **Workspaces:** `workspaces.json` en la misma carpeta; la pertenencia de cada conexión va en `connections.json`. El workspace activo se recuerda por usuario del webview.
- **Historial y consultas guardadas:** `history.json` y `saved_queries.json` en la misma carpeta, con permisos `0600`. **Guardan el SQL tal cual se ejecutó**, así que pueden contener datos sensibles (por ejemplo un `INSERT` con contraseñas): el historial se limita a 500 entradas por conexión y se borra al eliminar la conexión o con **Borrar historial**.
- **Contraseñas:** en el llavero del sistema (Secret Service en Linux, Credential Manager en Windows). En Linux necesitas un servicio de llavero activo, como GNOME Keyring o KWallet.

### Servidores antiguos con TLS 1.0 / 1.1

En el formulario de conexión, el selector **Versión mínima de TLS** permite negociar TLS 1.0 o 1.1 con servidores legacy (no aplica a SQLite). Es inseguro: con esta opción no se verifica el certificado del servidor. Úsala solo con servidores que no puedas actualizar. Para SQL Server antiguo compila con `make build` (ver *Compilar*).

### Diagnóstico de conexión

`PLUTON_DEBUG=1 ./build/bin/pluton-sql` imprime la traza del driver de SQL Server (`[mssql]`) y cada lectura/escritura TCP (`[tcp]`: tiempo, tamaño y primeros bytes; los datos TLS van cifrados).

## Desarrollo

```bash
make test                       # tests del backend (con el parche TLS)
make test-frontend              # tests del autocompletado (vitest)
make test-integration           # además, contra PostgreSQL y MariaDB reales (requiere Docker)
docker compose --profile mssql up -d --wait   # opcional: SQL Server, que activa sus casos en los tests de integración
cd frontend && npm run check    # comprobación de tipos del frontend
```

El icono de la app es `build/appicon.png`, generado desde `build/appicon.svg` (el mismo Plutón de `Logo.svelte`): si tocas el dibujo, vuelve a exportar el SVG a PNG de 1024×1024.

Si cambias los métodos expuestos en `internal/service`, los bindings de `frontend/wailsjs/` se regeneran con `wails generate module -tags webkit2_41` (también lo hace `make build`).

Los tests de integración usan `docker-compose.yml` (PostgreSQL 16 y MariaDB 11 en los puertos 15432 y 13306, con datos de `testdata/integration/`). Al terminar: `docker compose down`.

### Estructura

```
main.go, app.go        arranque de Wails y servicios expuestos
internal/config        perfiles de conexión (JSON)
internal/secrets       contraseñas en el llavero del sistema
internal/driver        un Dialect por motor (DSN y consultas de catálogo)
internal/session       pools de conexión abiertos
internal/service       API que consume el frontend
frontend/src           interfaz Svelte (App, componentes, lib/api.ts)
```
