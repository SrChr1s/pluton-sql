import * as Conn from '../../wailsjs/go/service/ConnectionService'
import * as Work from '../../wailsjs/go/service/WorkspaceService'
import * as Schema from '../../wailsjs/go/service/SchemaService'
import * as Query from '../../wailsjs/go/service/QueryService'
import * as Dialogs from '../../wailsjs/go/service/DialogService'
import * as Data from '../../wailsjs/go/service/DataService'
import * as History from '../../wailsjs/go/service/HistoryService'
import * as SavedQ from '../../wailsjs/go/service/SavedQueryService'
import * as Export from '../../wailsjs/go/service/ExportService'
import * as Import from '../../wailsjs/go/service/ImportService'
import * as Structure from '../../wailsjs/go/service/StructureService'
import * as SchemaEdit from '../../wailsjs/go/service/SchemaEditService'
import type { config, driver, service, history, transfer } from '../../wailsjs/go/models'

export type Profile = config.ConnectionProfile
export type Workspace = config.Workspace
export type Table = driver.TableInfo
export type Column = driver.ColumnInfo
export type Index = driver.IndexInfo
export type QueryResult = service.QueryResult
export type TablePage = service.TablePage
export type TableRef = service.TableRef
export type Change = service.Change
export type PageRequest = service.PageRequest
export type TableStructure = service.TableStructure
export type SchemaOp = service.SchemaOp
export type SchemaPlan = service.SchemaPlan
export type ColumnDef = service.ColumnDef
export type HistoryEntry = history.Entry
export type SavedQuery = history.Saved
export type ExportRequest = service.ExportRequest
export type ExportResult = service.ExportResult
export type ImportRequest = service.ImportRequest
export type ImportResult = service.ImportResult
export type CsvPreview = transfer.Preview
export type ForeignKey = driver.ForeignKeyInfo

export const emptyProfile = (): Profile =>
  ({ id: '', name: '', driver: 'postgres', host: 'localhost', port: 5432, user: '', database: '', filePath: '', sslMode: '', tlsMinVersion: '', color: '', workspace: '' }) as Profile

// Go serializa un slice nil como null: las listas se normalizan a [] en un solo punto.
const list = <A extends unknown[], T>(fn: (...a: A) => Promise<T[] | null>) =>
  (...a: A): Promise<T[]> => fn(...a).then((v) => v ?? [])

export const api = {
  drivers: list(Conn.Drivers),
  defaultPort: Conn.DefaultPort,
  listConnections: list(Conn.List),
  saveConnection: Conn.Save,
  deleteConnection: Conn.Delete,
  disconnectConnection: Conn.Disconnect,
  disconnectDatabase: Conn.DisconnectDatabase,
  testConnection: Conn.Test,
  listWorkspaces: list(Work.List),
  saveWorkspace: Work.Save,
  deleteWorkspace: Work.Delete,
  databases: list(Schema.Databases),
  schemas: list(Schema.Schemas),
  schemaIndex: Schema.Index,
  tables: list(Schema.Tables),
  columns: list(Schema.Columns),
  indexes: list(Schema.Indexes),
  execute: Query.Execute,
  cancel: Query.Cancel,
  describeTable: Structure.Describe,
  previewSchema: SchemaEdit.Preview,
  applySchema: SchemaEdit.Apply,
  tablePage: Data.Page,
  tableCount: Data.Count,
  applyChanges: Data.Apply,
  historyList: list(History.List),
  historyDelete: History.Delete,
  historyClear: History.Clear,
  savedList: list(SavedQ.List),
  savedSave: SavedQ.Save,
  savedDelete: SavedQ.Delete,
  openSqlFile: Dialogs.OpenSQLFile,
  saveSqlFile: Dialogs.SaveSQLFile,
  exportRun: Export.Run,
  exportCancel: Export.Cancel,
  importPreview: Import.Preview,
  importRun: Import.Run,
  importCancel: Import.Cancel,
  chooseExportFile: Dialogs.ChooseExportFile,
  chooseCsvFile: Dialogs.ChooseCSVFile,
  pickSQLiteFile: Dialogs.PickSQLiteFile,
  createSQLiteFile: Dialogs.CreateSQLiteFile,
}

export const errMsg = (e: unknown) => (typeof e === 'string' ? e : (e as Error)?.message ?? String(e))
