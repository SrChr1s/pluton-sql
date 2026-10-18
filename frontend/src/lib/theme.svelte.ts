export type ThemeMode = 'system' | 'light' | 'dark'
const KEY = 'pluton.theme'

function read(): ThemeMode {
  try {
    const v = localStorage.getItem(KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch { /* sin almacenamiento: se usa el del sistema */ }
  return 'system'
}

class Theme {
  mode = $state<ThemeMode>(read())
  private systemDark = $state(matchMedia('(prefers-color-scheme: dark)').matches)

  constructor() {
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => (this.systemDark = e.matches))
    this.apply()
  }

  /** Tema efectivo tras resolver "system". */
  get resolved(): 'light' | 'dark' {
    return this.mode === 'system' ? (this.systemDark ? 'dark' : 'light') : this.mode
  }

  set(mode: ThemeMode) {
    this.mode = mode
    try { localStorage.setItem(KEY, mode) } catch { /* no crítico */ }
    this.apply()
  }

  /** Alterna claro → oscuro → sistema. */
  cycle() {
    this.set(this.mode === 'light' ? 'dark' : this.mode === 'dark' ? 'system' : 'light')
  }

  private apply() {
    const el = document.documentElement
    if (this.mode === 'system') delete el.dataset.theme
    else el.dataset.theme = this.mode
  }
}

export const theme = new Theme()
