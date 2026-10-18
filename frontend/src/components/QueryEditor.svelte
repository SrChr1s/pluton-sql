<script lang="ts">
  import { onMount } from 'svelte'
  import { EditorView, keymap } from '@codemirror/view'
  import { EditorState, Prec, Compartment } from '@codemirror/state'
  import { basicSetup } from 'codemirror'
  import { sql, PostgreSQL, MySQL, SQLite, MSSQL } from '@codemirror/lang-sql'
  import { oneDark } from '@codemirror/theme-one-dark'
  import { theme } from '../lib/theme.svelte'

  let { value = $bindable(''), driver, onrun }: { value: string; driver: string; onrun: (sql: string) => void } = $props()

  const dialects: Record<string, typeof PostgreSQL> = { postgres: PostgreSQL, mysql: MySQL, sqlite: SQLite, mssql: MSSQL }
  const themeSlot = new Compartment()
  let host: HTMLDivElement
  let view: EditorView

  // Colores propios sobre los tokens de la app; el oscuro añade el resaltado de One Dark.
  const base = EditorView.theme({
    '&': { height: '100%', backgroundColor: 'var(--surface)', color: 'var(--text)', fontSize: '13px' },
    '.cm-scroller': { fontFamily: 'var(--mono)', lineHeight: '1.55' },
    '.cm-gutters': { backgroundColor: 'var(--surface)', color: 'var(--muted)', border: 'none' },
    '.cm-activeLine, .cm-activeLineGutter': { backgroundColor: 'var(--hover)' },
    '&.cm-focused': { outline: 'none' },
  })
  // Prec.highest: que el fondo y las fuentes de la app prevalezcan sobre los de One Dark.
  const themeExt = (dark: boolean) => (dark ? [oneDark, Prec.highest(base)] : [Prec.highest(base)])

  // Ctrl/Cmd+Enter ejecuta la selección, o todo el texto si no hay selección.
  const run = (v: EditorView) => {
    const sel = v.state.selection.main
    onrun(sel.empty ? v.state.doc.toString() : v.state.sliceDoc(sel.from, sel.to))
    return true
  }

  onMount(() => {
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          themeSlot.of(themeExt(theme.resolved === 'dark')),
          sql({ dialect: dialects[driver] ?? PostgreSQL, upperCaseKeywords: true }),
          Prec.highest(keymap.of([{ key: 'Mod-Enter', run }])),
          EditorView.updateListener.of((u) => { if (u.docChanged) value = u.state.doc.toString() }),
        ],
      }),
    })
    view.focus()
    return () => view.destroy()
  })

  $effect(() => {
    const dark = theme.resolved === 'dark'
    if (view) view.dispatch({ effects: themeSlot.reconfigure(themeExt(dark)) })
  })

  // Permite que el padre reemplace el texto.
  $effect(() => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    }
  })
</script>

<div class="editor" bind:this={host}></div>

<style>
  .editor { height: 100%; overflow: hidden; }
</style>
