package main

import (
	"context"
	"path/filepath"

	"github.com/SrChr1s/pluton-sql/internal/config"
	"github.com/SrChr1s/pluton-sql/internal/history"
	"github.com/SrChr1s/pluton-sql/internal/service"
	"github.com/SrChr1s/pluton-sql/internal/session"
)

// App agrupa los servicios expuestos al frontend.
type App struct {
	ctx         context.Context
	sess        *session.Manager
	Connections *service.ConnectionService
	Workspaces  *service.WorkspaceService
	Schema      *service.SchemaService
	Query       *service.QueryService
	Data        *service.DataService
	Structure   *service.StructureService
	SchemaEdit  *service.SchemaEditService
	History     *service.HistoryService
	Saved       *service.SavedQueryService
	Export      *service.ExportService
	Import      *service.ImportService
	events      *service.Events
	Dialogs     *service.DialogService
}

func NewApp() (*App, error) {
	store, err := config.NewStore("")
	if err != nil {
		return nil, err
	}
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	wsStore, err := config.NewWorkspaceStore("")
	if err != nil {
		return nil, err
	}
	histStore := history.NewStore(filepath.Join(dir, "history.json"))
	sess := session.NewManager(store)
	events := &service.Events{}
	return &App{
		events:      events,
		Export:      service.NewExportService(sess, events),
		Import:      service.NewImportService(sess, events),
		sess:        sess,
		Connections: service.NewConnectionService(store, sess, func(id string) { _ = histStore.Clear(id) }),
		Workspaces:  service.NewWorkspaceService(wsStore, store),
		Schema:      service.NewSchemaService(sess),
		Query:       service.NewQueryService(sess, service.NewHistoryRecorder(histStore)),
		History:     service.NewHistoryService(histStore),
		Saved:       service.NewSavedQueryService(history.NewSavedStore(filepath.Join(dir, "saved_queries.json"))),
		Data:        service.NewDataService(sess),
		Structure:   service.NewStructureService(sess),
		SchemaEdit:  service.NewSchemaEditService(sess),
		Dialogs:     service.NewDialogService(),
	}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.Dialogs.SetContext(ctx)
	a.Data.SetContext(ctx)
	a.events.SetContext(ctx)
}

func (a *App) shutdown(context.Context) { a.sess.CloseAll() }
