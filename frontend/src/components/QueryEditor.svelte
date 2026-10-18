<script lang="ts">
  import { onMount } from 'svelte'
  import { EditorView, keymap } from '@codemirror/view'
  import { EditorState, Prec, Compartment } from '@codemirror/state'
  import { basicSetup } from 'codemirror'
  import { sql, PostgreSQL, MySQL, SQLite, MSSQL } from '@codemirror/lang-sql'
  import { oneDark } from '@codemirror/theme-one-dark'
  import { theme } from '../lib/theme.svelte'
  import type { CompletionContext, CompletionResult } from '@codemirror/autocomplete'
  import { complete, type SchemaIndex, type Suggestion } from '../lib/sqlcomplete'

  let { value = $bindable(''), driver, index = null, onrun }: { value: string; driver: string; index?: SchemaIndex | null; onrun: (sql: string) => void } = $props()

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

  // Autocompletado con el esquema de la conexión. Lee el índice más reciente sin recrear el editor.
  let indexRef: SchemaIndex | null = null
  $effect(() => { indexRef = index })
  const cmType: Record<Suggestion['kind'], string> = { table: 'class', view: 'interface', schema: 'namespace', column: 'property', alias: 'variable' }
  const WINDOW_BEFORE = 60000 // en un script enorme solo se analiza el entorno del cursor
  const WINDOW_AFTER = 20000
  function schemaSource(ctx: CompletionContext): CompletionResult | null {
    const doc = ctx.state.doc
    const lo = Math.max(0, ctx.pos - WINDOW_BEFORE)
    const text = doc.sliceString(lo, Math.min(doc.length, ctx.pos + WINDOW_AFTER))
    const r = complete(text, ctx.pos - lo, indexRef, driver, ctx.explicit)
    if (!r) return null
    return {
      from: r.from + lo,
      options: r.options.map((o) => ({ label: o.label, apply: o.apply, type: cmType[o.kind], detail: o.detail, boost: o.boost })),
      validFor: /^[\w$]*$/,
    }
  }

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
          // Se suma a las palabras clave que ya ofrece el lenguaje SQL.
          EditorState.languageData.of(() => [{ autocomplete: schemaSource }]),
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
