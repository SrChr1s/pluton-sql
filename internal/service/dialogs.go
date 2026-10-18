package service

import (
	"context"
	"errors"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DialogService abre los diálogos nativos del sistema (GTK en Linux, Windows en Windows).
type DialogService struct{ ctx context.Context }

func NewDialogService() *DialogService { return &DialogService{} }

// SetContext recibe el contexto de Wails, necesario para mostrar diálogos.
func (d *DialogService) SetContext(ctx context.Context) { d.ctx = ctx }

var sqliteFilters = []runtime.FileFilter{
	{DisplayName: "Bases SQLite (*.db, *.sqlite, *.sqlite3, *.db3)", Pattern: "*.db;*.sqlite;*.sqlite3;*.db3"},
	{DisplayName: "Todos los archivos (*)", Pattern: "*"},
}

// PickSQLiteFile deja elegir una base SQLite existente. Devuelve "" si se cancela.
func (d *DialogService) PickSQLiteFile() (string, error) {
	if d.ctx == nil {
		return "", errors.New("diálogos no disponibles")
	}
	return runtime.OpenFileDialog(d.ctx, runtime.OpenDialogOptions{
		Title:   "Abrir base de datos SQLite",
		Filters: sqliteFilters,
	})
}

// CreateSQLiteFile deja elegir dónde crear una base nueva y deja allí un archivo
// vacío (SQLite lo trata como una base sin tablas). Devuelve "" si se cancela.
func (d *DialogService) CreateSQLiteFile() (string, error) {
	if d.ctx == nil {
		return "", errors.New("diálogos no disponibles")
	}
	path, err := runtime.SaveFileDialog(d.ctx, runtime.SaveDialogOptions{
		Title:           "Crear base de datos SQLite",
		DefaultFilename: "nueva.db",
		Filters:         sqliteFilters,
	})
	if err != nil || path == "" {
		return "", err
	}
	// O_EXCL no pisa nada: el diálogo ya pregunta si se sobrescribe, pero un
	// archivo existente debe conservar su contenido.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err == nil {
		err = f.Close()
	} else if errors.Is(err, os.ErrExist) {
		err = nil
	}
	return path, err
}
