<script lang="ts">
  import { onMount } from 'svelte'
  import { EditorView, keymap } from '@codemirror/view'
  import { EditorState, Prec } from '@codemirror/state'
  import { basicSetup } from 'codemirror'
  import { sql, PostgreSQL, MySQL, SQLite, MSSQL } from '@codemirror/lang-sql'
  import { oneDark } from '@codemirror/theme-one-dark'

  let { value = $bindable(''), driver, onrun }: { value: string; driver: string; onrun: (sql: string) => void } = $props()

  const dialects: Record<string, typeof PostgreSQL> = { postgres: PostgreSQL, mysql: MySQL, sqlite: SQLite, mssql: MSSQL }
  let host: HTMLDivElement
  let view: EditorView

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
          oneDark,
          sql({ dialect: dialects[driver] ?? PostgreSQL, upperCaseKeywords: true }),
          Prec.highest(keymap.of([{ key: 'Mod-Enter', run }])),
          EditorView.updateListener.of((u) => { if (u.docChanged) value = u.state.doc.toString() }),
          EditorView.theme({ '&': { height: '100%' }, '.cm-scroller': { fontFamily: 'var(--mono)' } }),
        ],
      }),
    })
    return () => view.destroy()
  })

  // Permite que el padre reemplace el texto (p. ej. al abrir una tabla).
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
