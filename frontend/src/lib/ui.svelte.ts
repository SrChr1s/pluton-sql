import type { Profile, TableStructure } from './api'
import type { TableLocation } from './table.svelte'

/** Diálogos propios: los confirm()/alert() nativos no son fiables en el webview. */
export type ConfirmOptions = { title: string; message?: string; confirmLabel?: string; danger?: boolean }

/** Qué se exporta: el resultado de una consulta, o una tabla con su filtro y orden actuales. */
export type ExportSource =
  | { kind: 'query'; profile: string; database: string; sql: string; name: string }
  | { kind: 'table'; profile: string; database: string; schema: string; table: string; filter: string; orderBy: string; desc: boolean; name: string }

export type ImportTarget = { loc: TableLocation; columns: { name: string; type: string; primaryKey: boolean; autoIncrement: boolean }[]; onDone: () => void }

export type SchemaDialog = { loc: TableLocation; kind: string; target: string; structure: TableStructure | null }

export type Toast = { id: number; msg: string; kind: 'ok' | 'err' }
export type PromptOptions = { title: string; label: string; value?: string; confirmLabel?: string }

class UI {
  /** Formulario de conexión abierto en el área principal (profile null = nueva). */
  connForm = $state<{ profile: Profile | null } | null>(null)
  exporting = $state<ExportSource | null>(null)
  importing = $state<ImportTarget | null>(null)
  toasts = $state<Toast[]>([])
  prompting = $state<(PromptOptions & { resolve: (v: string | null) => void }) | null>(null)
  private toastSeq = 0

  /** Aviso breve que desaparece solo; los errores duran más. */
  toast(msg: string, kind: 'ok' | 'err' = 'ok') {
    const id = ++this.toastSeq
    this.toasts.push({ id, msg, kind })
    setTimeout(() => (this.toasts = this.toasts.filter((t) => t.id !== id)), kind === 'err' ? 7000 : 3000)
  }

  dismissToast(id: number) {
    this.toasts = this.toasts.filter((t) => t.id !== id)
  }

  /** Pide un texto al usuario; null si cancela. */
  prompt(opts: PromptOptions): Promise<string | null> {
    return new Promise((resolve) => (this.prompting = { ...opts, resolve }))
  }

  answerPrompt(v: string | null) {
    this.prompting?.resolve(v)
    this.prompting = null
  }

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
