import { api, errMsg, type Profile, type QueryResult, type Workspace as WorkspaceInfo } from './api'
import { TableState, type TableLocation } from './table.svelte'
import { quoteIdent } from './sql'
import type { SchemaIndex } from './sqlcomplete'
import { ui } from './ui.svelte'

export type QueryTab = {
  kind: 'query'
  id: string
  title: string
  sql: string
  database: string
  /** Ya se ejecutó: queda vinculada a `database` hasta que se vuelva a ejecutar sobre otra base. */
  bound: boolean
  result: QueryResult | null
  error: string
  running: boolean
  queryId: string
  /** Archivo .sql asociado ('' = consulta sin archivo). savedSql es lo último guardado/abierto, para detectar cambios. */
  filePath: string
  savedSql: string
  crlf: boolean
  /** La consulta que produjo el resultado visible (lo que se exporta). */
  lastSql: string
}

/** Pestaña con los datos de una tabla, editables. */
export type DataTab = { kind: 'table'; id: string; title: string; database: string; state: TableState }

export type Tab = QueryTab | DataTab

const STORE = 'pluton.tabs.'
const WS_KEY = 'pluton.workspace'
const WS_LAST = 'pluton.wsLast'
const RECENT = 'pluton.recent'
const RECENT_MAX = 5

function readWorkspaceId(): string {
  try { return localStorage.getItem(WS_KEY) ?? '' } catch { return '' }
}

function readLastActive(): Record<string, string> {
  try {
    const v = JSON.parse(localStorage.getItem(WS_LAST) ?? '{}')
    return v && typeof v === 'object' && !Array.isArray(v) ? v : {}
  } catch { return {} }
}

function readRecent(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(RECENT) ?? '[]')
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string') : []
  } catch { return [] }
}

export function selectFrom(driver: string, schema: string, table: string, limit = 100): string {
  const name = (schema ? quoteIdent(driver, schema) + '.' : '') + quoteIdent(driver, table)
  return driver === 'mssql' ? `SELECT TOP ${limit} * FROM ${name};` : `SELECT * FROM ${name} LIMIT ${limit};`
}

class Workspace {
  profiles = $state<Profile[]>([])
  /** Bases de datos con pool abierto, por "perfil|base". Sirve para marcar a cuáles estamos conectados. */
  private openDbs = $state<Record<string, true>>({})
  /** Bases de datos de cada conexión (para el selector de la barra de pestañas). */
  private dbLists = $state<Record<string, string[]>>({})
  /** Base de datos que muestra el árbol de entidades en cada conexión (una sola a la vez). */
  private treeDbs = $state<Record<string, string>>({})
  workspaces = $state<WorkspaceInfo[]>([])
  /** Workspace activo: filtra la lista de conexiones ("" = todas). */
  workspaceId = $state<string>(readWorkspaceId())
  /** Última conexión activa de cada workspace, para recuperarla al volver a él. */
  private lastActive = $state<Record<string, string>>(readLastActive())
  /** Ids de las últimas conexiones abiertas, la más reciente primero. */
  recentIds = $state<string[]>(readRecent())
  active = $state<Profile | null>(null)
  private tabsByProfile = $state<Record<string, Tab[]>>({})
  private currentTab = $state<Record<string, string>>({})
  private counter = 0
  /** Sube cuando cambian las tablas (crear/renombrar/borrar) para que el árbol de esquema se recargue. */
  schemaVersion = $state(0)
  /** Índice de tablas y columnas para el autocompletado, por conexión y base de datos ('cargando' mientras llega). */
  private indexes = $state<Record<string, SchemaIndex | 'loading' | 'error'>>({})
  /** Sube tras cada ejecución para que el panel de historial se actualice. */
  historyVersion = $state(0)

  get tabs(): Tab[] {
    return this.active ? (this.tabsByProfile[this.active.id] ?? []) : []
  }

  get tab(): Tab | null {
    const id = this.active ? this.currentTab[this.active.id] : ''
    return this.tabs.find((t) => t.id === id) ?? null
  }

  /** La pestaña actual si es de consulta (si no, null). */
  get queryTab(): QueryTab | null {
    const t = this.tab
    return t?.kind === 'query' ? t : null
  }

  /** La pestaña actual si es de datos (si no, null). */
  get dataTab(): DataTab | null {
    const t = this.tab
    return t?.kind === 'table' ? t : null
  }

  async loadProfiles() {
    this.profiles = await api.listConnections()
  }

  async loadWorkspaces() {
    this.workspaces = await api.listWorkspaces()
    if (this.workspaceId && !this.workspaces.some((w) => w.id === this.workspaceId)) this.saveWorkspaceId('')
  }

  private saveWorkspaceId(id: string) {
    this.workspaceId = id
    try { localStorage.setItem(WS_KEY, id); localStorage.setItem(WS_LAST, JSON.stringify(this.lastActive)) } catch { /* no crítico */ }
  }

  get currentWorkspace(): WorkspaceInfo | null {
    return this.workspaces.find((w) => w.id === this.workspaceId) ?? null
  }

  /** Las conexiones del workspace activo (o todas). */
  get visibleProfiles(): Profile[] {
    return this.workspaceId ? this.profiles.filter((p) => p.workspace === this.workspaceId) : this.profiles
  }

  /** Cambia de workspace: recuerda la conexión activa del actual y recupera la del nuevo (sus pestañas siguen siendo las de esa conexión). */
  selectWorkspace(id: string) {
    if (id === this.workspaceId) return
    this.lastActive[this.workspaceId] = this.active?.id ?? ''
    this.saveWorkspaceId(id)
    const target = this.visibleProfiles.find((p) => p.id === this.lastActive[id])
    if (target) this.connect(target)
    else this.active = null
  }

  /** Mueve una conexión a otro workspace ("" = ninguno) sin cerrar sus pools. */
  async moveProfile(profileId: string, workspaceId: string) {
    const moved = await api.moveConnection(profileId, workspaceId)
    await this.loadProfiles()
    if (this.active?.id === profileId) {
      this.active = this.profiles.find((p) => p.id === profileId) ?? moved
      // La conexión en uso sigue visible: se acompaña al workspace al que se movió.
      if (this.workspaceId && this.workspaceId !== workspaceId) this.saveWorkspaceId(workspaceId)
    }
  }

  async createWorkspace(name: string): Promise<WorkspaceInfo> {
    const w = await api.saveWorkspace({ id: '', name } as WorkspaceInfo)
    await this.loadWorkspaces()
    return w
  }

  async renameWorkspace(w: WorkspaceInfo, name: string) {
    await api.saveWorkspace({ ...w, name } as WorkspaceInfo)
    await this.loadWorkspaces()
  }

  /** Borra el workspace; sus conexiones se conservan y quedan sin workspace. */
  async deleteWorkspace(w: WorkspaceInfo) {
    await api.deleteWorkspace(w.id)
    delete this.lastActive[w.id]
    if (this.workspaceId === w.id) this.saveWorkspaceId('')
    await Promise.all([this.loadWorkspaces(), this.loadProfiles()])
  }

  /** Las conexiones recientes que aún existen. */
  get recents(): Profile[] {
    return this.recentIds.map((id) => this.visibleProfiles.find((p) => p.id === id)).filter((p): p is Profile => !!p)
  }

  /** Bases de datos de la conexión activa (vacío hasta que se cargan). */
  get databases(): string[] {
    return this.active ? (this.dbLists[this.active.id] ?? []) : []
  }

  /** Base por defecto de la conexión: la del perfil o, en SQLite, la única. */
  get defaultDb(): string {
    const p = this.active
    if (!p) return ''
    return p.driver === 'sqlite' ? (this.dbLists[p.id]?.[0] ?? 'main') : p.database
  }

  /** Base de datos de una pestaña (la suya o, si no tiene, la de la conexión). */
  dbOf(t: Tab | null): string {
    return t?.database || this.defaultDb
  }

  /** La base seleccionada: la que explora el árbol y la que usan las pestañas nuevas ('' = ninguna elegida aún). */
  get treeDb(): string {
    const p = this.active
    return p ? (this.treeDbs[p.id] ?? this.defaultDb) : ''
  }

  /** Al cambiar de pestaña, el árbol pasa a la base de esa pestaña. */
  followTab() {
    const db = this.dbOf(this.tab)
    const p = this.active
    if (p && db && (p.driver === 'sqlite' || this.isDbOpen(p.id, db))) this.treeDbs[p.id] = db // no reabre una base cerrada
  }

  /** La base de la pestaña actual (la de la barra inferior). */
  get currentDb(): string {
    return this.dbOf(this.tab)
  }

  async loadDatabases(p: Profile | null = this.active) {
    if (!p) return
    try { this.dbLists[p.id] = await api.databases(p.id) } catch { /* el árbol muestra el error */ }
  }

  isDbOpen(profileId: string, database: string): boolean {
    return !!this.openDbs[`${profileId}|${database}`]
  }

  /** Hay al menos una base de datos abierta en la conexión (en SQLite, la única base cuenta siempre). */
  hasOpenDb(p: Profile): boolean {
    return p.driver === 'sqlite' || Object.keys(this.openDbs).some((k) => k.startsWith(`${p.id}|`))
  }

  markDbOpen(database: string) {
    if (this.active && database) this.openDbs[`${this.active.id}|${database}`] = true
  }

  /** Cierra el pool de una sola base de datos. En SQLite (una sola base) equivale a desconectar. */
  async disconnectDatabase(database: string) {
    const p = this.active
    if (!p) return
    if (p.driver === 'sqlite') return this.disconnect()
    await api.disconnectDatabase(p.id, database)
    delete this.openDbs[`${p.id}|${database}`]
    if (this.treeDbs[p.id] === database) this.treeDbs[p.id] = ''
    delete this.indexes[this.indexKey(database)]
    this.resetTabDatabases(p.id, database)
  }

  connect(p: Profile) {
    // Conectar a una conexión de otro workspace lleva a ese workspace (recordando dónde estábamos).
    if (this.workspaceId && p.workspace !== this.workspaceId) {
      this.lastActive[this.workspaceId] = this.active?.id ?? ''
      this.saveWorkspaceId(p.workspace || '')
    }
    this.active = p
    if (!this.dbLists[p.id]) void this.loadDatabases(p)
    if (p.driver !== 'sqlite' && p.database) this.openDbs[`${p.id}|${p.database}`] = true
    this.recentIds = [p.id, ...this.recentIds.filter((id) => id !== p.id)].slice(0, RECENT_MAX)
    try { localStorage.setItem(RECENT, JSON.stringify(this.recentIds)) } catch { /* no crítico */ }
    if (!this.tabsByProfile[p.id]) {
      this.tabsByProfile[p.id] = this.restore(p.id)
      if (this.tabsByProfile[p.id].length === 0) this.newTab()
      this.currentTab[p.id] = this.tabsByProfile[p.id][0].id
    }
  }

  /** Cierra los pools de la conexión activa. Sus pestañas se conservan para cuando se vuelva a conectar. */
  async disconnect() {
    const p = this.active
    if (!p) return
    this.active = null
    for (const k of Object.keys(this.indexes)) if (k.startsWith(`${p.id}|`)) delete this.indexes[k]
    for (const k of Object.keys(this.openDbs)) if (k.startsWith(`${p.id}|`)) delete this.openDbs[k]
    delete this.treeDbs[p.id]
    this.resetTabDatabases(p.id)
    await api.disconnectConnection(p.id)
  }

  /** Las pestañas de consulta que apuntaban a `database` (o a todas, si no se indica) vuelven a la base por defecto. */
  private resetTabDatabases(profileId: string, database?: string) {
    for (const t of this.tabsByProfile[profileId] ?? []) {
      if (t.kind === 'query' && (database === undefined || t.database === database)) { t.database = ''; t.bound = false }
    }
  }

  private add<T extends Tab>(tab: T): T {
    const p = this.active!
    this.tabsByProfile[p.id] = [...(this.tabsByProfile[p.id] ?? []), tab]
    this.currentTab[p.id] = tab.id
    // devuelve el proxy reactivo, no el objeto original
    return this.tabsByProfile[p.id].find((x) => x.id === tab.id) as T
  }

  newTab(opts: Partial<Pick<QueryTab, 'title' | 'sql' | 'database'>> = {}): QueryTab | null {
    if (!this.active) return null
    return this.add<QueryTab>({
      kind: 'query',
      id: crypto.randomUUID(),
      title: opts.title ?? `Consulta ${++this.counter}`,
      sql: opts.sql ?? '',
      database: opts.database ?? this.treeDb,
      bound: false, result: null, error: '', running: false, queryId: '',
      filePath: '', savedSql: '', crlf: false, lastSql: '',
    })
  }

  /** ¿Tiene la pestaña de consulta cambios sin guardar en su archivo? */
  isDirty(t: Tab): boolean {
    return t.kind === 'query' && t.filePath !== '' && t.sql !== t.savedSql
  }

  select(id: string) {
    if (this.active) this.currentTab[this.active.id] = id
  }

  /** Cierra una pestaña (pide confirmación si hay cambios sin aplicar); siempre queda al menos una. */
  async close(id: string) {
    const p = this.active
    if (!p) return
    const tabs = this.tabsByProfile[p.id]
    const i = tabs.findIndex((t) => t.id === id)
    if (i < 0) return
    const t = tabs[i]
    if (t.kind === 'table' && !(await t.state.canDiscard('cierras la pestaña'))) return
    if (this.isDirty(t)) {
      const ok = await ui.confirm({
        title: `Cambios sin guardar en ${t.title}`,
        message: 'Si cierras la pestaña se perderán los cambios que no hayas guardado en el archivo.',
        confirmLabel: 'Descartar cambios', danger: true,
      })
      if (!ok) return
    }
    if (t.kind === 'query' && t.running) api.cancel(t.queryId)
    const rest = this.tabsByProfile[p.id].filter((x) => x.id !== id)
    this.tabsByProfile[p.id] = rest
    if (rest.length === 0) this.newTab()
    else if (this.currentTab[p.id] === id) this.currentTab[p.id] = rest[Math.min(i, rest.length - 1)].id
  }

  /** Cambia de pestaña en cíclico (dir = ±1). */
  step(dir: number) {
    const tabs = this.tabs
    if (tabs.length < 2 || !this.tab) return
    const i = tabs.findIndex((t) => t.id === this.tab!.id)
    this.select(tabs[(i + dir + tabs.length) % tabs.length].id)
  }

  selectNth(n: number) {
    const t = this.tabs[n]
    if (t) this.select(t.id)
  }

  async run(text?: string) {
    const t = this.queryTab
    const p = this.active
    if (!t || !p || t.running) return
    const q = text ?? t.sql
    if (!q.trim()) return
    // Se ejecuta en la base seleccionada; si es otra que la de la pestaña, la pestaña pasa a quedar vinculada a ella.
    const db = this.treeDb || t.database
    if (db !== t.database) void this.ensureIndex(db)
    t.database = db
    t.bound = true
    this.markDbOpen(db)
    t.running = true
    t.error = ''
    t.queryId = crypto.randomUUID()
    try {
      t.result = await api.execute(t.queryId, p.id, t.database, q)
      t.lastSql = q
      if (/^\s*(?:--[^\n]*\n\s*)*(CREATE|ALTER|DROP|RENAME)\b/i.test(q)) this.invalidateIndex() // la estructura pudo cambiar
    } catch (e) {
      t.error = errMsg(e)
      t.result = null
    }
    t.running = false
    this.historyVersion++
  }

  cancel() {
    const t = this.queryTab
    if (t?.running) api.cancel(t.queryId)
  }

  /** Abre un script .sql en una pestaña nueva (si ya está abierto, solo la selecciona). */
  async openFile() {
    try {
      const f = await api.openSqlFile()
      if (!f || !this.active) return
      const open = this.tabs.find((t): t is QueryTab => t.kind === 'query' && t.filePath === f.path)
      if (open) return this.select(open.id)
      const t = this.newTab({ title: f.name, sql: f.content })
      if (t) { t.filePath = f.path; t.savedSql = f.content; t.crlf = f.crlf }
    } catch (e) {
      ui.toast(`No se pudo abrir el archivo: ${errMsg(e)}`, 'err')
    }
  }

  /** Guarda la pestaña en su archivo; sin archivo (o con saveAs) pregunta dónde. */
  async saveFile(t: QueryTab | null = this.queryTab, saveAs = false) {
    if (!t) return
    try {
      const path = await api.saveSqlFile(saveAs ? '' : t.filePath, t.sql, t.crlf)
      if (!path) return // cancelado
      t.filePath = path
      t.savedSql = t.sql
      t.title = path.split(/[\\/]/).pop() ?? t.title
      ui.toast(`Guardado en ${path}`)
    } catch (e) {
      ui.toast(`No se pudo guardar: ${errMsg(e)}`, 'err')
    }
  }

  /** Abre un SQL (de historial o consulta guardada) en una pestaña nueva, sin tocar las existentes. */
  openSql(sql: string, title: string) {
    const t = this.newTab({ title, sql })
    return t
  }

  /** Abre los datos de una tabla en una pestaña editable (reutiliza la pestaña si ya está abierta). */
  openTable(database: string, schema: string, table: string, mode: 'data' | 'structure' = 'data') {
    const p = this.active
    if (!p) return
    const open = this.tabs.find(
      (t): t is DataTab => t.kind === 'table' && t.state.loc.database === database && t.state.loc.schema === schema && t.state.loc.table === table,
    )
    if (open) {
      this.select(open.id)
      void open.state.setMode(mode)
      return
    }
    this.markDbOpen(database)
    const state = new TableState({ profile: p.id, database, schema, table })
    const tab = this.add<DataTab>({ kind: 'table', id: crypto.randomUUID(), title: table, database, state })
    if (mode === 'structure') void tab.state.setMode('structure')
    tab.state.load()
  }

  private indexKey(database: string) { return `${this.active?.id}|${database}` }

  /** El índice ya cargado de esa base de datos (null si aún no está). Es una lectura pura. */
  indexFor(database: string): SchemaIndex | null {
    const v = this.indexes[this.indexKey(database)]
    return v && typeof v === 'object' ? v : null
  }

  /** Carga el índice si falta. El autocompletado es opcional: un fallo no molesta, solo no hay sugerencias. */
  async ensureIndex(database: string) {
    const p = this.active
    if (!p) return
    // Cargar el índice abre el pool de esa base: no se hace con una base que el usuario desconectó.
    // Se reabre sola al ejecutar algo en ella (run) o al abrirla en el árbol.
    const target = database || p.database
    if (p.driver !== 'sqlite' && target && !this.isDbOpen(p.id, target)) return
    const key = this.indexKey(database)
    if (this.indexes[key]) return
    this.indexes[key] = 'loading'
    try {
      this.indexes[key] = await api.schemaIndex(p.id, database)
    } catch {
      this.indexes[key] = 'error'
    }
  }

  /** Olvida los índices de la conexión activa (o de todas) para que se vuelvan a cargar. */
  invalidateIndex(all = false) {
    const prefix = `${this.active?.id}|`
    for (const k of Object.keys(this.indexes)) if (all || k.startsWith(prefix)) delete this.indexes[k]
  }

  /** Reacciona a una modificación de estructura ya aplicada. */
  async schemaChanged(loc: TableLocation, op: { kind: string; newName?: string }) {
    const tab = this.tabs.find(
      (t): t is DataTab => t.kind === 'table' && t.state.loc.database === loc.database && t.state.loc.schema === loc.schema && t.state.loc.table === loc.table,
    )
    tab?.state.clearPending() // sus cambios pendientes ya no encajan con la nueva estructura
    this.invalidateIndex()
    if (op.kind === 'create_table') {
      this.schemaVersion++
      this.openTable(loc.database, loc.schema, loc.table, 'structure')
    } else if (op.kind === 'rename_table') {
      this.schemaVersion++
      if (tab) await this.close(tab.id)
      this.openTable(loc.database, loc.schema, op.newName ?? loc.table, 'structure')
    } else if (op.kind === 'drop_table') {
      this.schemaVersion++
      if (tab) await this.close(tab.id)
    } else if (tab) {
      await Promise.all([tab.state.load(), tab.state.loadStructure()])
    }
  }

  /** Abre un SELECT de la tabla en una pestaña de consulta. */
  openTableAsSql(database: string, schema: string, table: string) {
    const p = this.active
    if (!p) return
    const t = this.newTab({ title: table, database, sql: selectFrom(p.driver, schema, table) })
    if (t) this.run(t.sql)
  }

  /** Elige la base que explora el árbol; una pestaña de consulta aún sin ejecutar la sigue (la ya ejecutada queda vinculada a la suya). */
  setDatabase(database: string) {
    if (!this.active) return
    this.markDbOpen(database)
    this.treeDbs[this.active.id] = database
    if (this.queryTab && !this.queryTab.bound) { // una consulta ya ejecutada conserva su base hasta que se vuelva a ejecutar
      this.queryTab.database = database
      void this.ensureIndex(database) // abre el pool y carga el autocompletado de esa base
    }
  }

  /** Cambia cuando cambia cualquier borrador; sirve para disparar el guardado. */
  get signature(): string {
    return JSON.stringify(
      Object.entries(this.tabsByProfile).map(([id, ts]) => [id, ts.filter((t) => t.kind === 'query').map((t) => [t.title, t.sql, t.database, t.filePath, t.savedSql])]),
    )
  }

  // --- persistencia local de borradores (solo pestañas de consulta: SQL y título) ---
  persist() {
    for (const [id, tabs] of Object.entries(this.tabsByProfile)) {
      try {
        const qs = tabs.filter((t): t is QueryTab => t.kind === 'query')
        localStorage.setItem(STORE + id, JSON.stringify(qs.map((t) => ({ title: t.title, sql: t.sql, database: t.database, filePath: t.filePath, savedSql: t.savedSql, crlf: t.crlf }))))
      } catch { /* sin almacenamiento: se pierde el borrador, nada más */ }
    }
  }

  private restore(profileId: string): Tab[] {
    try {
      const raw = JSON.parse(localStorage.getItem(STORE + profileId) ?? '[]') as Pick<QueryTab, 'title' | 'sql' | 'database' | 'filePath' | 'savedSql' | 'crlf'>[]
      return raw.map((r): QueryTab => ({
        kind: 'query', id: crypto.randomUUID(), title: r.title, sql: r.sql, database: '', bound: false, // se entra siempre a la base del perfil, no a la última usada
        result: null, error: '', running: false, queryId: '',
        filePath: r.filePath ?? '', savedSql: r.savedSql ?? '', crlf: r.crlf ?? false, lastSql: '',
      }))
    } catch {
      return []
    }
  }

  async removeProfile(p: Profile) {
    await api.deleteConnection(p.id)
    delete this.tabsByProfile[p.id]
    for (const k of Object.keys(this.indexes)) if (k.startsWith(`${p.id}|`)) delete this.indexes[k]
    try { localStorage.removeItem(STORE + p.id) } catch { /* ignorar */ }
    if (this.active?.id === p.id) this.active = null
    this.recentIds = this.recentIds.filter((id) => id !== p.id)
    try { localStorage.setItem(RECENT, JSON.stringify(this.recentIds)) } catch { /* no crítico */ }
    await this.loadProfiles()
  }
}

export const ws = new Workspace()
