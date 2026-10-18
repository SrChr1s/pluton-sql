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

// Go serializa un slice nil como null: las listas se normalizan a [] en un solo punto.
const list = <A extends unknown[], T>(fn: (...a: A) => Promise<T[] | null>) =>
  (...a: A): Promise<T[]> => fn(...a).then((v) => v ?? [])

export const api = {
  drivers: list(Conn.Drivers),
  defaultPort: Conn.DefaultPort,
  listConnections: list(Conn.List),
  saveConnection: Conn.Save,
  deleteConnection: Conn.Delete,
  testConnection: Conn.Test,
  databases: list(Schema.Databases),
  schemas: list(Schema.Schemas),
  tables: list(Schema.Tables),
  columns: list(Schema.Columns),
  indexes: list(Schema.Indexes),
  execute: Query.Execute,
  cancel: Query.Cancel,
  pickSQLiteFile: Dialogs.PickSQLiteFile,
  createSQLiteFile: Dialogs.CreateSQLiteFile,
}

export const errMsg = (e: unknown) => (typeof e === 'string' ? e : (e as Error)?.message ?? String(e))
