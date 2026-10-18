import { api, errMsg, type Profile, type QueryResult } from './api'

export type QueryTab = {
  id: string
  title: string
  sql: string
  database: string
  result: QueryResult | null
  error: string
  running: boolean
  queryId: string
}

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
  private tabsByProfile = $state<Record<string, QueryTab[]>>({})
  private currentTab = $state<Record<string, string>>({})
  private counter = 0

  get tabs(): QueryTab[] {
    return this.active ? (this.tabsByProfile[this.active.id] ?? []) : []
  }

  get tab(): QueryTab | null {
    const id = this.active ? this.currentTab[this.active.id] : ''
    return this.tabs.find((t) => t.id === id) ?? null
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

  newTab(opts: Partial<Pick<QueryTab, 'title' | 'sql' | 'database'>> = {}): QueryTab | null {
    const p = this.active
    if (!p) return null
    const t: QueryTab = {
      id: crypto.randomUUID(),
      title: opts.title ?? `Consulta ${++this.counter}`,
      sql: opts.sql ?? '',
      database: opts.database ?? this.tab?.database ?? '',
      result: null, error: '', running: false, queryId: '',
    }
    this.tabsByProfile[p.id] = [...(this.tabsByProfile[p.id] ?? []), t]
    this.currentTab[p.id] = t.id
    // devuelve el proxy reactivo, no el objeto original
    return this.tabsByProfile[p.id].find((x) => x.id === t.id) ?? null
  }

  select(id: string) {
    if (this.active) this.currentTab[this.active.id] = id
  }

  /** Cierra una pestaña; siempre queda al menos una. */
  close(id: string) {
    const p = this.active
    if (!p) return
    const tabs = this.tabsByProfile[p.id]
    const i = tabs.findIndex((t) => t.id === id)
    if (i < 0) return
    if (tabs[i].running) api.cancel(tabs[i].queryId)
    const rest = tabs.filter((t) => t.id !== id)
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
    const t = this.tab
    const p = this.active
    if (!t || !p || t.running) return
    const q = text ?? t.sql
    if (!q.trim()) return
    t.running = true
    t.error = ''
    t.queryId = crypto.randomUUID()
    try {
      t.result = await api.execute(t.queryId, p.id, t.database, q)
    } catch (e) {
      t.error = errMsg(e)
      t.result = null
    }
    t.running = false
  }

  cancel() {
    const t = this.tab
    if (t?.running) api.cancel(t.queryId)
  }

  openTable(database: string, schema: string, table: string) {
    const p = this.active
    if (!p) return
    const t = this.newTab({ title: table, database, sql: selectFrom(p.driver, schema, table) })
    if (t) this.run(t.sql)
  }

  setDatabase(database: string) {
    if (this.tab) this.tab.database = database
  }

  /** Cambia cuando cambia cualquier borrador; sirve para disparar el guardado. */
  get signature(): string {
    return JSON.stringify(Object.entries(this.tabsByProfile).map(([id, ts]) => [id, ts.map((t) => [t.title, t.sql, t.database])]))
  }

  // --- persistencia local de borradores (solo SQL y título) ---
  persist() {
    for (const [id, tabs] of Object.entries(this.tabsByProfile)) {
      try {
        localStorage.setItem(STORE + id, JSON.stringify(tabs.map((t) => ({ title: t.title, sql: t.sql, database: t.database }))))
      } catch { /* sin almacenamiento: se pierde el borrador, nada más */ }
    }
  }

  private restore(profileId: string): QueryTab[] {
    try {
      const raw = JSON.parse(localStorage.getItem(STORE + profileId) ?? '[]') as Pick<QueryTab, 'title' | 'sql' | 'database'>[]
      return raw.map((r) => ({
        id: crypto.randomUUID(), title: r.title, sql: r.sql, database: r.database ?? '',
        result: null, error: '', running: false, queryId: '',
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
