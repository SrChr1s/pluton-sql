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
4. Doble clic en una tabla ejecuta `SELECT * ... LIMIT 100` (`TOP 100` en SQL Server).
5. Escribe SQL en el editor y pulsa **Ctrl+Enter** (o **▶ Ejecutar**) para ejecutar la selección, o todo el texto si no hay selección. **■ Cancelar** interrumpe una consulta en curso.
6. Edita una conexión con **✎** o doble clic, y elimínala con **✕**.

Los resultados se limitan a 1000 filas; la barra inferior avisa si se truncaron.

### Dónde se guardan los datos

- **Conexiones:** `connections.json` en el directorio de configuración del usuario (`~/.config/pluton-sql/` en Linux, `%AppData%\pluton-sql\` en Windows). No contiene contraseñas.
- **Contraseñas:** en el llavero del sistema (Secret Service en Linux, Credential Manager en Windows). En Linux necesitas un servicio de llavero activo, como GNOME Keyring o KWallet.

### Servidores antiguos con TLS 1.0 / 1.1

En el formulario de conexión, el selector **Versión mínima de TLS** permite negociar TLS 1.0 o 1.1 con servidores legacy (no aplica a SQLite). Es inseguro: con esta opción no se verifica el certificado del servidor. Úsala solo con servidores que no puedas actualizar. Para SQL Server antiguo compila con `make build` (ver *Compilar*).

### Diagnóstico de conexión

`PLUTON_DEBUG=1 ./build/bin/pluton-sql` imprime la traza del driver de SQL Server (`[mssql]`) y cada lectura/escritura TCP (`[tcp]`: tiempo, tamaño y primeros bytes; los datos TLS van cifrados).

## Desarrollo

```bash
make test                       # tests del backend (con el parche TLS)
make test-integration           # además, contra PostgreSQL y MariaDB reales (requiere Docker)
cd frontend && npm run check    # comprobación de tipos del frontend
```

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
