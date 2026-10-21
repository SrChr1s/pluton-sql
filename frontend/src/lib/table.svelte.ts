import { api, errMsg, type Change, type PageRequest, type TablePage, type TableStructure } from './api'
import { ui } from './ui.svelte'

export type TableLocation = { profile: string; database: string; schema: string; table: string }

type Edits = Record<number, Record<string, string | null>>

const BINARY = /blob|binary|bytea|image/i
const TEXTUAL = /char|text|string|clob|enum|set/i

/** Las columnas binarias llegan como hex y no se pueden reescribir fielmente. */
export const isReadonlyType = (type: string) => BINARY.test(type)

/** Una cadena vacía solo es un valor válido en columnas de texto; en el resto se interpreta como NULL. */
const normalizeInput = (type: string, v: string | null): string | null => (v === '' && !TEXTUAL.test(type) ? null : v)

export const display = (v: unknown): string | null => (v === null || v === undefined ? null : String(v))

/** Estado de una pestaña de datos: página cargada, orden/filtro y cambios pendientes. */
export class TableState {
  readonly loc: TableLocation
  page = $state.raw<TablePage | null>(null) // sin proxy profundo: miles de filas × columnas
  loading = $state(false)
  error = $state('')
  notice = $state('')
  offset = $state(0)
  limit = $state(100)
  orderBy = $state('')
  desc = $state(false)
  filter = $state('')
  /** Columnas ocultas en la rejilla (solo afecta a la vista). */
  hidden = $state<string[]>([])
  total = $state<number | null>(null)
  counting = $state(false)

  // Vista de estructura (columnas, índices, claves foráneas y DDL).
  mode = $state<'data' | 'structure'>('data')
  structure = $state<TableStructure | null>(null)
  structureLoading = $state(false)
  structureError = $state('')

  // Cambios pendientes: no se escribe nada hasta llamar a apply().
  edits = $state<Edits>({})
  deleted = $state<Record<number, boolean>>({})
  drafts = $state<Record<string, string | null>[]>([])

  constructor(loc: TableLocation) {
    this.loc = loc
  }

  async loadStructure() {
    this.structureLoading = true
    this.structureError = ''
    try {
      const st = await api.describeTable(this.loc)
      st.columns = st.columns ?? []
      st.indexes = st.indexes ?? []
      st.foreignKeys = st.foreignKeys ?? []
      this.structure = st
    } catch (e) {
      this.structureError = errMsg(e)
    }
    this.structureLoading = false
  }

  async setMode(mode: 'data' | 'structure') {
    this.mode = mode
    if (mode === 'structure' && !this.structure) await this.loadStructure()
  }

  // --- derivados ---
  get keyColumns(): string[] { return this.page?.keyColumns ?? [] }
  get columns() { return this.page?.columns ?? [] }
  private visible = $derived(this.columns.filter((c) => !this.hidden.includes(c.name)))
  private indexByName = $derived(new Map(this.columns.map((c, i) => [c.name, i])))
  get visibleColumns() { return this.visible }
  get canEditRows() { return !!this.page && !this.page.isView && this.keyColumns.length > 0 }
  get canInsert() { return !!this.page && !this.page.isView }
  get pending(): number {
    return Object.keys(this.edits).length + Object.keys(this.deleted).length + this.drafts.length
  }
  get firstRow() { return this.offset + 1 }
  get lastRow() { return this.offset + (this.page?.rows.length ?? 0) }

  private request(): PageRequest {
    // Wails solo serializa a JSON; el cast evita fabricar la clase con convertValues.
    return { table: this.loc, offset: this.offset, limit: this.limit, orderBy: this.orderBy, desc: this.desc, filter: this.filter } as PageRequest
  }

  /** Pide confirmación antes de descartar cambios pendientes. */
  async canDiscard(action: string): Promise<boolean> {
    if (this.pending === 0) return true
    return ui.confirm({
      title: 'Hay cambios sin aplicar',
      message: `Si ${action}, se perderán ${this.pending} cambio(s) pendiente(s).`,
      confirmLabel: 'Descartar cambios', danger: true,
    })
  }

  /** Recarga la página actual descartando los cambios pendientes (ya confirmados por el llamador). */
  async load() {
    this.loading = true
    this.error = ''
    try {
      const page = await api.tablePage(this.request())
      page.keyColumns = page.keyColumns ?? []
      page.columns = page.columns ?? []
      page.rows = page.rows ?? []
      this.page = page
      this.clearPending()
    } catch (e) {
      this.error = errMsg(e)
    }
    this.loading = false
  }

  async reload(action = 'recargas la tabla') {
    if (await this.canDiscard(action)) await this.load()
  }

  async goto(offset: number) {
    if (!(await this.canDiscard('cambias de página'))) return
    this.offset = Math.max(0, offset)
    await this.load()
  }

  async sortBy(col: string) {
    if (!(await this.canDiscard('cambias el orden'))) return
    // asc → desc → sin orden
    if (this.orderBy !== col) { this.orderBy = col; this.desc = false }
    else if (!this.desc) this.desc = true
    else { this.orderBy = ''; this.desc = false }
    this.offset = 0
    await this.load()
  }

  /** Ordena por la columna en un sentido concreto; desc = null quita el orden. */
  async setSort(col: string, desc: boolean | null) {
    if (!(await this.canDiscard('cambias el orden'))) return
    this.orderBy = desc === null ? '' : col
    this.desc = desc === true
    this.offset = 0
    await this.load()
  }

  hide(col: string) { if (!this.hidden.includes(col) && this.visibleColumns.length > 1) this.hidden.push(col) }
  showAll() { this.hidden = [] }

  /** Valores de las columnas clave de una fila cargada: identifican la fila en el backend. */
  keyOf(row: number): Record<string, unknown> {
    const o: Record<string, unknown> = {}
    for (const k of this.keyColumns) o[k] = this.page?.rows[row]?.[this.colIndex(k)]
    return o
  }

  async applyFilter(f: string) {
    if (!(await this.canDiscard('cambias el filtro'))) return
    this.filter = f.trim()
    this.offset = 0
    this.total = null
    await this.load()
  }

  async setLimit(n: number) {
    if (!(await this.canDiscard('cambias el tamaño de página'))) return
    this.limit = n
    this.offset = 0
    await this.load()
  }

  async count() {
    this.counting = true
    try { this.total = await api.tableCount(this.loc, this.filter) } catch (e) { this.error = errMsg(e) }
    this.counting = false
  }

  // --- edición de celdas ---
  colIndex(col: string) { return this.indexByName.get(col) ?? -1 }
  private colType(col: string) { return this.columns[this.colIndex(col)]?.type ?? '' }

  original(row: number, col: string): string | null {
    return display(this.page?.rows[row]?.[this.colIndex(col)])
  }

  /** Valor mostrado de una fila existente: el editado si lo hay, si no el original. */
  value(row: number, col: string): string | null {
    const e = this.edits[row]
    return e && col in e ? e[col] : this.original(row, col)
  }

  isEdited(row: number, col: string): boolean {
    const e = this.edits[row]
    return !!e && col in e
  }

  setCell(row: number, col: string, raw: string | null) {
    const v = normalizeInput(this.colType(col), raw)
    if (v === this.original(row, col)) {
      // vuelve a ser el original: no es un cambio
      if (this.edits[row]) {
        delete this.edits[row][col]
        if (Object.keys(this.edits[row]).length === 0) delete this.edits[row]
      }
      return
    }
    ;(this.edits[row] ??= {})[col] = v
  }

  toggleDelete(row: number) {
    if (this.deleted[row]) delete this.deleted[row]
    else this.deleted[row] = true
  }

  addDraft() { this.drafts.push({}) }
  removeDraft(i: number) { this.drafts.splice(i, 1) }
  setDraft(i: number, col: string, raw: string | null) {
    if (raw === null) { this.drafts[i][col] = null; return } // NULL explícito
    if (raw === '' && !(col in this.drafts[i])) return // Enter sin escribir nada: sigue valiendo el valor por defecto
    const v = normalizeInput(this.colType(col), raw)
    if (v === null) delete this.drafts[i][col] // vacío en columna no textual: decide el valor por defecto
    else this.drafts[i][col] = v // incluye '' en columnas de texto
  }

  clearPending() {
    this.edits = {}
    this.deleted = {}
    this.drafts = []
  }

  discard() { this.clearPending(); this.notice = '' }

  /** Cambios en el formato del backend. */
  private changes(): Change[] {
    const out: Change[] = []
    const keyOf = (r: number) => {
      const o: Record<string, unknown> = {}
      this.columns.forEach((c, i) => (o[c.name] = this.page!.rows[r][i]))
      return o
    }
    for (const r of Object.keys(this.deleted).map(Number)) out.push({ kind: 'delete', key: keyOf(r), values: {} } as Change)
    for (const [r, values] of Object.entries(this.edits)) {
      if (this.deleted[Number(r)]) continue // se borra: la edición no importa
      out.push({ kind: 'update', key: keyOf(Number(r)), values: { ...values } } as Change)
    }
    for (const d of this.drafts) out.push({ kind: 'insert', key: {}, values: { ...d } } as Change)
    return out
  }

  /** Aplica todos los cambios en una transacción y recarga. Devuelve true si tuvo éxito. */
  async apply(): Promise<boolean> {
    if (this.pending === 0) return true
    this.loading = true
    this.error = ''
    try {
      const r = await api.applyChanges(this.loc, this.changes())
      const parts = [r.inserted && `${r.inserted} insertada(s)`, r.updated && `${r.updated} modificada(s)`, r.deleted && `${r.deleted} borrada(s)`].filter(Boolean)
      this.notice = `Aplicado: ${parts.join(', ')}`
      this.clearPending()
      this.total = null
      await this.load()
      return true
    } catch (e) {
      this.error = errMsg(e) // los cambios pendientes se conservan para corregirlos
      this.loading = false
      return false
    }
  }
}
