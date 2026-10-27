# Compila con crypto/tls parcheado (sin registro de 1 byte en TLS 1.0 + CBC),
# necesario para conectar a SQL Server 2005/2008 con TLS 1.0. Ver tools/tlsoverlay.
OVERLAY := $(CURDIR)/build/_overlay/overlay.json
GOFLAGS_OVERLAY := -overlay=$(OVERLAY)
WAILS ?= wails
TAGS ?= webkit2_41

.PHONY: overlay build build-windows dev test test-frontend test-integration install uninstall package-linux installer-windows package-windows

# Versión de los paquetes. Mantenla igual que info.productVersion de wails.json (la usa el instalador de Windows).
VERSION ?= 0.1.3
BIN := build/bin/pluton-sql
# Instalación de usuario por defecto (sin sudo). Para todo el sistema: sudo make install PREFIX=/usr/local
PREFIX ?= $(HOME)/.local
DESTDIR ?=
NFPM ?= nfpm

overlay:
	go run ./tools/tlsoverlay

build: overlay
	GOFLAGS=$(GOFLAGS_OVERLAY) $(WAILS) build -tags $(TAGS)

build-windows: overlay
	GOFLAGS=$(GOFLAGS_OVERLAY) $(WAILS) build -platform windows/amd64

dev: overlay
	GOFLAGS=$(GOFLAGS_OVERLAY) $(WAILS) dev -tags $(TAGS)

test: overlay
	PLUTON_EXPECT_NOSPLIT=1 GOFLAGS=$(GOFLAGS_OVERLAY) go test ./internal/...

# Tests del frontend (lógica pura: autocompletado de SQL).
test-frontend:
	cd frontend && npm test

# Levanta PostgreSQL y MariaDB en Docker (puertos 15432 y 13306) y ejecuta los tests de integración.
test-integration:
	docker compose up -d --wait
	go test -count=1 -tags integration ./internal/...

# Instala binario, icono y lanzador (.desktop). Con DESTDIR solo copia a esa carpeta (para empaquetar).
install: build
	install -Dm755 $(BIN) $(DESTDIR)$(PREFIX)/bin/pluton-sql
	install -Dm644 build/appicon.svg $(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/pluton-sql.svg
	install -Dm644 build/linux/pluton-sql.png $(DESTDIR)$(PREFIX)/share/icons/hicolor/512x512/apps/pluton-sql.png
	install -Dm644 build/linux/pluton-sql.desktop $(DESTDIR)$(PREFIX)/share/applications/pluton-sql.desktop
	sed -i 's|^Exec=.*|Exec=$(PREFIX)/bin/pluton-sql|' $(DESTDIR)$(PREFIX)/share/applications/pluton-sql.desktop
	@if [ -z "$(DESTDIR)" ]; then \
		gtk-update-icon-cache -q -t $(PREFIX)/share/icons/hicolor 2>/dev/null || true; \
		update-desktop-database -q $(PREFIX)/share/applications 2>/dev/null || true; \
		echo "Instalado en $(PREFIX). Si no lo encuentras en el lanzador, cierra sesión y vuelve a entrar."; \
	fi

uninstall:
	rm -f $(DESTDIR)$(PREFIX)/bin/pluton-sql \
		$(DESTDIR)$(PREFIX)/share/icons/hicolor/scalable/apps/pluton-sql.svg \
		$(DESTDIR)$(PREFIX)/share/icons/hicolor/512x512/apps/pluton-sql.png \
		$(DESTDIR)$(PREFIX)/share/applications/pluton-sql.desktop

# Paquetes para Linux en dist/: .rpm, .deb (requieren nfpm) y un .tar.gz con la misma estructura de /usr.
package-linux: build
	@command -v $(NFPM) >/dev/null || { echo "Falta nfpm: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest"; exit 1; }
	rm -rf dist/stage && mkdir -p dist
	VERSION=$(VERSION) $(NFPM) package -f packaging/nfpm.yaml -p rpm -t dist/
	VERSION=$(VERSION) $(NFPM) package -f packaging/nfpm.yaml -p deb -t dist/
	$(MAKE) install DESTDIR=$(CURDIR)/dist/stage PREFIX=/usr
	tar -C dist/stage -czf dist/pluton-sql_$(VERSION)_linux_amd64.tar.gz usr
	rm -rf dist/stage
	@ls -la dist

# Instalador de Windows (NSIS) en build/bin/. Requiere NSIS (makensis) en el PATH; WebView2 lo descarga el instalador si falta.
installer-windows: overlay
	@command -v makensis >/dev/null || { echo "Falta NSIS (makensis). En Fedora: sudo dnf install mingw32-nsis; en Windows: https://nsis.sourceforge.io"; exit 1; }
	GOFLAGS=$(GOFLAGS_OVERLAY) $(WAILS) build -platform windows/amd64 -nsis

# Paquetes de Windows en dist/: instalador NSIS (requiere makensis) y zip portable (requiere zip).
package-windows: overlay
	@command -v makensis >/dev/null || { echo "Falta NSIS (makensis)"; exit 1; }
	GOFLAGS=$(GOFLAGS_OVERLAY) $(WAILS) build -clean -platform windows/amd64 -nsis -trimpath
	mkdir -p dist/stage/PlutonSQL
	cp build/bin/pluton-sql-amd64-installer.exe dist/pluton-sql_$(VERSION)_windows_amd64_setup.exe
	cp build/bin/pluton-sql.exe LICENSE README.md dist/stage/PlutonSQL/
	cd dist/stage && zip -qr ../pluton-sql_$(VERSION)_windows_amd64.zip PlutonSQL
	rm -rf dist/stage
	@ls -la dist
