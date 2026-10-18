import { api, errMsg, type Profile, type QueryResult } from './api'
import { TableState, type TableLocation } from './table.svelte'
import { ui } from './ui.svelte'

export type QueryTab = {
  kind: 'query'
  id: string
  title: string
  sql: string
  database: string
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

/** Comillas de identificador según el motor. */
export function quoteIdent(driver: string, name: string): string {
  if (driver === 'mysql') return '`' + name.replaceAll('`', '``') + '`'
  if (driver === 'mssql') return '[' + name.replaceAll(']', ']]') + ']'
  return '"' + name.replaceAll('"', '""') + '"'
}

export function selectFrom(driver: string, schema: string, table: string, limit = 100): string {
  const name = (schema ? quoteIdent(driver, schema) + '.' : '') + quoteIdent(driver, table)
  return driver === 'mssql' ? `SELECT TOP ${limit} * FROM ${name};` : `SELECT * FROM ${name} LIMIT ${limit};`
}

class Workspace {
  profiles = $state<Profile[]>([])
  active = $state<Profile | null>(null)
  private tabsByProfile = $state<Record<string, Tab[]>>({})
  private currentTab = $state<Record<string, string>>({})
  private counter = 0
  /** Sube cuando cambian las tablas (crear/renombrar/borrar) para que el árbol de esquema se recargue. */
  schemaVersion = $state(0)
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

  connect(p: Profile) {
    this.active = p
    if (!this.tabsByProfile[p.id]) {
      this.tabsByProfile[p.id] = this.restore(p.id)
      if (this.tabsByProfile[p.id].length === 0) this.newTab()
      this.currentTab[p.id] = this.tabsByProfile[p.id][0].id
    }
  }

  disconnect() {
    this.active = null
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
      database: opts.database ?? this.tab?.database ?? '',
      result: null, error: '', running: false, queryId: '',
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
    t.running = true
    t.error = ''
    t.queryId = crypto.randomUUID()
    try {
      t.result = await api.execute(t.queryId, p.id, t.database, q)
      t.lastSql = q
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
    const state = new TableState({ profile: p.id, database, schema, table })
    const tab = this.add<DataTab>({ kind: 'table', id: crypto.randomUUID(), title: table, database, state })
    if (mode === 'structure') void tab.state.setMode('structure')
    tab.state.load()
  }

  /** Reacciona a una modificación de estructura ya aplicada. */
  async schemaChanged(loc: TableLocation, op: { kind: string; newName?: string }) {
    const tab = this.tabs.find(
      (t): t is DataTab => t.kind === 'table' && t.state.loc.database === loc.database && t.state.loc.schema === loc.schema && t.state.loc.table === loc.table,
    )
    tab?.state.clearPending() // sus cambios pendientes ya no encajan con la nueva estructura
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

  setDatabase(database: string) {
    if (this.queryTab) this.queryTab.database = database
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
        kind: 'query', id: crypto.randomUUID(), title: r.title, sql: r.sql, database: r.database ?? '',
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
    try { localStorage.removeItem(STORE + p.id) } catch { /* ignorar */ }
    if (this.active?.id === p.id) this.active = null
    await this.loadProfiles()
  }
}

export const ws = new Workspace()
