import * as Conn from '../../wailsjs/go/service/ConnectionService'
import * as Schema from '../../wailsjs/go/service/SchemaService'
import * as Query from '../../wailsjs/go/service/QueryService'
import * as Dialogs from '../../wailsjs/go/service/DialogService'
import type { config, driver, service } from '../../wailsjs/go/models'

export type Profile = config.ConnectionProfile
export type Table = driver.TableInfo
export type Column = driver.ColumnInfo
export type Index = driver.IndexInfo
export type QueryResult = service.QueryResult

export const emptyProfile = (): Profile =>
  ({ id: '', name: '', driver: 'postgres', host: 'localhost', port: 5432, user: '', database: '', filePath: '', sslMode: '', tlsMinVersion: '' }) as Profile

export const api = {
  drivers: Conn.Drivers,
  defaultPort: Conn.DefaultPort,
  listConnections: Conn.List,
  saveConnection: Conn.Save,
  deleteConnection: Conn.Delete,
  testConnection: Conn.Test,
  databases: Schema.Databases,
  schemas: Schema.Schemas,
  tables: Schema.Tables,
  columns: Schema.Columns,
  indexes: Schema.Indexes,
  execute: Query.Execute,
  cancel: Query.Cancel,
  pickSQLiteFile: Dialogs.PickSQLiteFile,
  createSQLiteFile: Dialogs.CreateSQLiteFile,
}

export const errMsg = (e: unknown) => (typeof e === 'string' ? e : (e as Error)?.message ?? String(e))
