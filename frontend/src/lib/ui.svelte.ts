import type { TableStructure } from './api'
import type { TableLocation } from './table.svelte'

/** Diálogos propios: los confirm()/alert() nativos no son fiables en el webview. */
export type ConfirmOptions = { title: string; message?: string; confirmLabel?: string; danger?: boolean }

export type SchemaDialog = { loc: TableLocation; kind: string; target: string; structure: TableStructure | null }

class UI {
  schema = $state<SchemaDialog | null>(null)

  openSchema(loc: TableLocation, kind: string, target = '', structure: TableStructure | null = null) {
    this.schema = { loc, kind, target, structure }
  }

  dialog = $state<(ConfirmOptions & { resolve: (ok: boolean) => void }) | null>(null)

  confirm(opts: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => (this.dialog = { ...opts, resolve }))
  }

  answer(ok: boolean) {
    this.dialog?.resolve(ok)
    this.dialog = null
  }
}

export const ui = new UI()
