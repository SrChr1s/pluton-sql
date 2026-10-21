// Ventana de filas visibles para tablas grandes: solo se montan las filas que caen dentro del
// área de scroll (más un margen), el resto se sustituye por dos filas separadoras.
const BLOCK = 8

export class RowWindow {
  top = $state(0)
  view = $state(600)
  rowH = $state(24)
  left = $state(0)
  viewW = $state(1200)
  overscan = 8
  private el: HTMLElement | null = null
  private raf = 0

  // Índices cuantizados por bloques: mientras el scroll no cruce un bloque, nada cambia y no se re-renderiza.
  private first = $derived(Math.floor(this.top / this.rowH / BLOCK) * BLOCK)
  private last = $derived(Math.ceil((this.top + this.view) / this.rowH / BLOCK) * BLOCK)

  range(count: number): { start: number; end: number; before: number; after: number } {
    const start = Math.max(0, this.first - this.overscan)
    const end = Math.min(count, this.last + this.overscan)
    return { start, end, before: start * this.rowH, after: Math.max(0, count - end) * this.rowH }
  }

  private read = () => {
    this.raf = 0
    if (!this.el) return
    if (Math.abs(this.el.scrollTop - this.top) >= 1) this.top = this.el.scrollTop
    this.view = this.el.clientHeight || this.view
    if (Math.abs(this.el.scrollLeft - this.left) >= 1) this.left = this.el.scrollLeft
    this.viewW = this.el.clientWidth || this.viewW
    const tr = this.el.querySelector<HTMLElement>('tbody tr[data-row]')
    if (tr && tr.offsetHeight > 8 && Math.abs(tr.offsetHeight - this.rowH) > 0.5) this.rowH = tr.offsetHeight
  }
  private schedule = () => { if (!this.raf) this.raf = requestAnimationFrame(this.read) }

  // use:win.attach — engancha scroll y cambios de tamaño del contenedor.
  attach = (node: HTMLElement) => {
    this.el = node
    node.addEventListener('scroll', this.schedule, { passive: true })
    const ro = new ResizeObserver(this.schedule)
    ro.observe(node)
    this.schedule()
    return {
      destroy: () => {
        node.removeEventListener('scroll', this.schedule)
        ro.disconnect()
        if (this.raf) cancelAnimationFrame(this.raf)
        this.el = null
      },
    }
  }

  // Tras cargar otra página/resultado: volver arriba y re-medir.
  reset() { if (this.el) this.el.scrollTop = 0; this.top = 0; this.schedule() }

  // Índice de la columna que contiene la posición x (anchos acumulados, con `lead` px de cabecera fija).
  static colAt(widths: number[], x: number, lead = 72): number {
    let acc = lead
    for (let i = 0; i < widths.length; i++) { acc += widths[i]; if (acc > x) return i }
    return Math.max(0, widths.length - 1)
  }
}

// Anchos de columna calculados una vez por página (cabecera + muestra de filas): con
// table-layout: fixed el navegador no tiene que re-medir toda la tabla al cambiar las filas visibles.
export function columnWidths(headers: string[], rows: unknown[][], headExtra: number[] = []): number[] {
  const sample = rows.slice(0, 40)
  return headers.map((h, i) => {
    let chars = 0
    for (const r of sample) {
      const v = r[i]
      const n = v === null || v === undefined ? 4 : String(v).length
      if (n > chars) chars = n
    }
    const body = (chars + 2) * 7.3 + 22
    const head = h.length * 7.2 + (headExtra[i] ?? 0) * 6.4 + 46
    return Math.round(Math.min(420, Math.max(70, body, head)))
  })
}
