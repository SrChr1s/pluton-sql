package service

import (
	"log"

	"github.com/SrChr1s/pluton-sql/internal/history"
)

// HistoryRecorder alimenta el historial. Va aparte de HistoryService para que Wails no lo exponga al
// frontend: solo el backend registra ejecuciones.
type HistoryRecorder struct{ store *history.Store }

func NewHistoryRecorder(store *history.Store) *HistoryRecorder { return &HistoryRecorder{store: store} }

// Record implementa Recorder. Un fallo al guardar el historial nunca debe romper una consulta.
func (h *HistoryRecorder) Record(profile, database, sql string, durationMs, rows int64, isQuery bool, errMsg string) {
	if err := h.store.Add(history.Entry{Profile: profile, Database: database, SQL: sql, DurationMs: durationMs, Rows: rows, IsQuery: isQuery, Error: errMsg}); err != nil {
		log.Printf("historial: no se pudo guardar: %v", err)
	}
}

// HistoryService expone la consulta del historial al frontend.
type HistoryService struct{ store *history.Store }

func NewHistoryService(store *history.Store) *HistoryService { return &HistoryService{store: store} }

// List devuelve las ejecuciones de la conexión, la más reciente primero. search filtra por texto.
func (h *HistoryService) List(profile, search string, limit int) ([]history.Entry, error) {
	return h.store.List(profile, search, limit)
}

func (h *HistoryService) Delete(id string) error { return h.store.Delete(id) }

// Clear borra el historial de una conexión.
func (h *HistoryService) Clear(profile string) error { return h.store.Clear(profile) }

// SavedQueryService expone las consultas guardadas.
type SavedQueryService struct{ store *history.SavedStore }

func NewSavedQueryService(store *history.SavedStore) *SavedQueryService {
	return &SavedQueryService{store: store}
}

func (s *SavedQueryService) List() ([]history.Saved, error) { return s.store.List() }

// Save crea (ID vacío) o actualiza una consulta guardada.
func (s *SavedQueryService) Save(q history.Saved) (history.Saved, error) { return s.store.Save(q) }

func (s *SavedQueryService) Delete(id string) error { return s.store.Delete(id) }
