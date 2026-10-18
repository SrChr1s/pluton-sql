export type ThemeMode = 'light' | 'dark'
const KEY = 'pluton.theme'

function read(): ThemeMode {
  try {
    if (localStorage.getItem(KEY) === 'light') return 'light'
  } catch { /* sin almacenamiento: se usa el oscuro */ }
  return 'dark'
}

class Theme {
  mode = $state<ThemeMode>(read())

  constructor() {
    this.apply()
  }

  /** Tema efectivo (alias de mode, lo usan los componentes). */
  get resolved(): ThemeMode {
    return this.mode
  }

  set(mode: ThemeMode) {
    this.mode = mode
    try { localStorage.setItem(KEY, mode) } catch { /* no crítico */ }
    this.apply()
  }

  /** Alterna entre oscuro y claro. */
  toggle() {
    this.set(this.mode === 'dark' ? 'light' : 'dark')
  }

  private apply() {
    document.documentElement.dataset.theme = this.mode
  }
}

export const theme = new Theme()
