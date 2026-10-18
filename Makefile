# Compila con crypto/tls parcheado (sin registro de 1 byte en TLS 1.0 + CBC),
# necesario para conectar a SQL Server 2005/2008 con TLS 1.0. Ver tools/tlsoverlay.
OVERLAY := $(CURDIR)/build/_overlay/overlay.json
GOFLAGS_OVERLAY := -overlay=$(OVERLAY)
WAILS ?= wails
TAGS ?= webkit2_41

.PHONY: overlay build build-windows dev test test-frontend test-integration

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
