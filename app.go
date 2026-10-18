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
	Schema      *service.SchemaService
	Query       *service.QueryService
	Data        *service.DataService
	Structure   *service.StructureService
	SchemaEdit  *service.SchemaEditService
	History     *service.HistoryService
	Saved       *service.SavedQueryService
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
	histStore := history.NewStore(filepath.Join(dir, "history.json"))
	sess := session.NewManager(store)
	return &App{
		sess:        sess,
		Connections: service.NewConnectionService(store, sess, func(id string) { _ = histStore.Clear(id) }),
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
}

func (a *App) shutdown(context.Context) { a.sess.CloseAll() }
