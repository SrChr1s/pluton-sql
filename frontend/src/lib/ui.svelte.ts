/** Diálogos propios: los confirm()/alert() nativos no son fiables en el webview. */
export type ConfirmOptions = { title: string; message?: string; confirmLabel?: string; danger?: boolean }

class UI {
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
