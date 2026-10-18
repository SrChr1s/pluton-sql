<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws, type DataTab } from '../lib/workspace.svelte'
  import { isReadonlyType } from '../lib/table.svelte'
  import { copyText } from '../lib/clipboard'
  import { ui } from '../lib/ui.svelte'
  import StructureView from './StructureView.svelte'

  let { tab }: { tab: DataTab } = $props()
  const t = $derived(tab.state)

  let filterText = $state(tab.state.filter)
  type Cell = { draft: number | null; row: number; col: string }
  type Edit = Cell & { seq: number }
  let editSeq = 0 // identifica cada sesión de edición; un blur de un input antiguo no debe cerrar la nueva
  let editing = $state<Edit | null>(null)
  let editText = $state('')
  let selected = $state<Cell | null>(null)
  let gridEl = $state<HTMLDivElement>()

  const sizes = [50, 100, 500, 1000]
  const isNumber = (r: number, ci: number) => typeof t.page?.rows[r]?.[ci] === 'number'
  const editable = (col: string) => {
    const c = t.columns.find((x) => x.name === col)
    return !!c && !isReadonlyType(c.type)
  }
  const sameCell = (a: Cell | null, b: Cell) => !!a && a.draft === b.draft && a.row === b.row && a.col === b.col

  function startEdit(cell: Cell) {
    if (!editable(cell.col)) return
    if (cell.draft === null && (!t.canEditRows || t.deleted[cell.row])) return
    if (cell.draft !== null && !t.canInsert) return
    const v = cell.draft === null ? t.value(cell.row, cell.col) : (t.drafts[cell.draft][cell.col] ?? '')
    editText = v ?? ''
    editing = { ...cell, seq: ++editSeq }
    selected = cell
  }

  function commit(value: string | null = editText) {
    const c = editing
    if (!c) return
    editing = null // antes de aplicar: el blur posterior no debe volver a confirmar
    if (c.draft === null) t.setCell(c.row, c.col, value)
    else t.setDraft(c.draft, c.col, value)
  }

  function cancel() { editing = null; gridEl?.focus() }

  function nextEditable(col: string, dir: 1 | -1): string | null {
    const names = t.columns.map((c) => c.name)
    for (let i = names.indexOf(col) + dir; i >= 0 && i < names.length; i += dir) if (editable(names[i])) return names[i]
    return null
  }

  function onEditKey(e: KeyboardEvent) {
    // Estas teclas no deben subir a la cuadrícula: su Enter volvería a abrir el editor recién cerrado.
    if (e.key === 'Enter' || e.key === 'Escape' || e.key === 'Tab') e.stopPropagation()
    if (e.key === 'Enter') { e.preventDefault(); const c = editing!; commit(); selected = { draft: c.draft, row: c.row, col: c.col }; gridEl?.focus() }
    else if (e.key === 'Escape') { e.preventDefault(); cancel() }
    else if (e.key === 'Tab') {
      e.preventDefault()
      const c = editing!
      const next = nextEditable(c.col, e.shiftKey ? -1 : 1)
      commit()
      if (next) startEdit({ ...c, col: next })
    }
  }

  // El valor lo asigna bind:value después de montar: seleccionar en el siguiente frame para que
  // lo escrito reemplace al contenido en vez de añadirse al final.
  function focusSelect(node: HTMLInputElement) {
    node.dataset.seq = String(editing?.seq ?? 0)
    node.focus()
    const f = requestAnimationFrame(() => node.select())
    return { destroy: () => cancelAnimationFrame(f) }
  }

  function onGridKey(e: KeyboardEvent) {
    if (editing || !selected) return
    const s = selected
    const cols = t.columns.map((c) => c.name)
    const rows = t.page?.rows.length ?? 0
    const move = (dr: number, dc: number) => {
      e.preventDefault()
      const ci = Math.min(cols.length - 1, Math.max(0, cols.indexOf(s.col) + dc))
      selected = { draft: null, row: Math.min(rows - 1, Math.max(0, (s.draft === null ? s.row : 0) + dr)), col: cols[ci] }
    }
    if (e.key === 'ArrowDown') move(1, 0)
    else if (e.key === 'ArrowUp') move(-1, 0)
    else if (e.key === 'ArrowRight') move(0, 1)
    else if (e.key === 'ArrowLeft') move(0, -1)
    else if (e.key === 'Enter' || e.key === 'F2') { e.preventDefault(); startEdit(s) }
    else if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'c') {
      e.preventDefault()
      const v = s.draft === null ? t.value(s.row, s.col) : (t.drafts[s.draft][s.col] ?? null)
      void copyText(v ?? '')
    }
  }

  const pendingText = $derived(
    [
      Object.keys(t.edits).length && `${Object.keys(t.edits).length} editada(s)`,
      Object.keys(t.deleted).length && `${Object.keys(t.deleted).length} a borrar`,
      t.drafts.length && `${t.drafts.length} nueva(s)`,
    ].filter(Boolean).join(' · '),
  )

  async function importCsv() {
    if (!t.page || !(await t.canDiscard('importas datos'))) return
    ui.importing = {
      loc: t.loc,
      columns: t.columns.map((c) => ({ name: c.name, type: c.type, primaryKey: c.primaryKey, autoIncrement: c.autoIncrement })),
      onDone: () => { t.total = null; void t.load() },
    }
  }

  function addRow() {
    t.addDraft()
    // Con clave primaria suele ser autogenerada: se empieza por la primera columna que no sea clave.
    const skipKeys = t.page?.hasPk ?? false
    const first = t.columns.find((c) => editable(c.name) && !(skipKeys && t.keyColumns.includes(c.name))) ?? t.columns.find((c) => editable(c.name))
    if (first) queueMicrotask(() => startEdit({ draft: t.drafts.length - 1, row: 0, col: first.name }))
  }
</script>

<div class="tv">
  <div class="modes" role="tablist" aria-label="Vista de la tabla">
    <button class="mode" class:on={t.mode === 'data'} role="tab" aria-selected={t.mode === 'data'} onclick={() => t.setMode('data')}><Icon name="table" size={13} /> Datos</button>
    <button class="mode" class:on={t.mode === 'structure'} role="tab" aria-selected={t.mode === 'structure'} onclick={() => t.setMode('structure')}><Icon name="layers" size={13} /> Estructura</button>
    <span class="who">{t.loc.schema ? `${t.loc.schema}.` : ''}{t.loc.table}{t.page?.isView || t.structure?.isView ? ' · vista' : ''}</span>
    {#if t.pending > 0 && t.mode === 'structure'}<span class="pend-note">{t.pending} cambio(s) sin aplicar en Datos</span>{/if}
  </div>

  {#if t.mode === 'structure'}
    <StructureView {tab} />
  {:else}
  <div class="bar">
    <button class="ghost icon" title="Recargar (F5)" aria-label="Recargar" onclick={() => t.reload()} disabled={t.loading}><Icon name="refresh" size={15} /></button>
    <div class="filter">
      <input placeholder="Filtro WHERE — p. ej. edad > 30 AND nombre LIKE 'A%'" aria-label="Filtro WHERE" bind:value={filterText}
             onkeydown={(e) => e.key === 'Enter' && t.applyFilter(filterText)} />
      {#if filterText || t.filter}
        <button class="ghost icon" aria-label="Quitar filtro" title="Quitar filtro" onclick={() => { filterText = ''; t.applyFilter('') }}><Icon name="x" size={12} /></button>
      {/if}
    </div>
    <button onclick={() => t.applyFilter(filterText)} disabled={t.loading}>Filtrar</button>
    <span class="sep"></span>
    <button class="ghost icon" aria-label="Página anterior" title="Página anterior" disabled={t.offset === 0 || t.loading} onclick={() => t.goto(t.offset - t.limit)}><Icon name="left" size={15} /></button>
    <span class="range">
      {#if t.page}{t.page.rows.length ? `${t.firstRow}–${t.lastRow}` : '0'}{t.total !== null ? ` de ${t.total}` : ''}{/if}
    </span>
    <button class="ghost icon" aria-label="Página siguiente" title="Página siguiente" disabled={!t.page?.hasMore || t.loading} onclick={() => t.goto(t.offset + t.limit)}><Icon name="right" size={15} /></button>
    {#if t.total === null}<button class="ghost" onclick={() => t.count()} disabled={t.counting} title="Contar todas las filas">{t.counting ? 'Contando…' : 'Contar'}</button>{/if}
    <select aria-label="Filas por página" value={t.limit} onchange={(e) => t.setLimit(Number(e.currentTarget.value))}>
      {#each sizes as n}<option value={n}>{n} filas</option>{/each}
    </select>
    <span class="grow"></span>
    <button onclick={addRow} disabled={!t.canInsert || t.loading}><Icon name="plus" size={13} /> Fila</button>
    {#if t.pending > 0}
      <button onclick={() => t.discard()}><Icon name="undo" size={13} /> Descartar</button>
      <button class="primary" onclick={() => t.apply()} disabled={t.loading} title="Aplicar en una transacción (Ctrl+S)"><Icon name="check" size={13} /> Aplicar ({t.pending})</button>
    {/if}
    <button class="ghost" title="Exportar la tabla (todas las filas, con el filtro y el orden actuales)" disabled={!t.page}
            onclick={() => (ui.exporting = { kind: 'table', profile: t.loc.profile, database: t.loc.database, schema: t.loc.schema, table: t.loc.table, filter: t.filter, orderBy: t.orderBy, desc: t.desc, name: t.loc.table })}>
      <Icon name="download" size={14} /> Exportar
    </button>
    {#if t.canInsert}
      <button class="ghost" title="Importar un CSV en esta tabla" disabled={!t.page} onclick={importCsv}><Icon name="upload" size={14} /> Importar</button>
    {/if}
    <button class="ghost" title="Abrir como consulta SQL" onclick={() => ws.openTableAsSql(t.loc.database, t.loc.schema, t.loc.table)}><Icon name="sql" size={14} /> SQL</button>
  </div>

  {#if t.error}
    <div class="banner err" role="alert"><pre>{t.error}</pre><button class="ghost icon" aria-label="Cerrar" onclick={() => (t.error = '')}><Icon name="x" size={13} /></button></div>
  {:else if t.notice}
    <div class="banner ok" role="status"><span>{t.notice}</span><button class="ghost icon" aria-label="Cerrar" onclick={() => (t.notice = '')}><Icon name="x" size={13} /></button></div>
  {:else if t.page?.isView}
    <div class="banner info">Es una vista: solo lectura.</div>
  {:else if t.page && t.keyColumns.length === 0}
    <div class="banner warn">No hay columnas con las que identificar una fila: solo se pueden insertar filas nuevas.</div>
  {:else if t.page && !t.page.hasPk}
    <div class="banner warn">Sin clave primaria: las filas se identifican por todas sus columnas. Si hay filas idénticas, el cambio se rechaza.</div>
  {/if}

  <div class="grid" bind:this={gridEl} tabindex="0" role="grid" aria-label="Datos de {tab.title}" onkeydown={onGridKey}>
    {#if !t.page}
      <div class="empty">{t.loading ? 'Cargando…' : 'Sin datos'}</div>
    {:else}
      <table class:busy={t.loading}>
        <thead>
          <tr>
            <th class="n"></th>
            {#each t.columns as c (c.name)}
              <th aria-sort={t.orderBy === c.name ? (t.desc ? 'descending' : 'ascending') : 'none'}>
                <button class="hd" onclick={() => t.sortBy(c.name)} title="{c.name} · {c.type}{c.nullable ? '' : ' · NOT NULL'} — clic para ordenar">
                  {#if t.keyColumns.includes(c.name) && t.page.hasPk}<span class="pk"><Icon name="key" size={11} /></span>{/if}
                  <span class="cn">{c.name}</span><span class="ct">{c.type}</span>
                  {#if t.orderBy === c.name}<Icon name={t.desc ? 'down' : 'up'} size={11} />{/if}
                </button>
              </th>
            {/each}
          </tr>
        </thead>
        <tbody>
          {#each t.drafts as d, i (i)}
            <tr class="draft">
              <td class="n"><button class="ghost icon rm" aria-label="Quitar fila nueva" onclick={() => t.removeDraft(i)}><Icon name="x" size={12} /></button></td>
              {#each t.columns as c (c.name)}
                {@const cell = { draft: i, row: 0, col: c.name }}
                <td class:sel={sameCell(selected, cell)} class:ro={!editable(c.name)} onclick={() => { selected = cell; gridEl?.focus() }} ondblclick={() => startEdit(cell)}>
                  {#if sameCell(editing, cell)}
                    {@render editor()}
                  {:else if c.name in d}
                    {#if d[c.name] === null}<span class="null">NULL</span>{:else}{d[c.name]}{/if}
                  {:else}<span class="dflt">por defecto</span>{/if}
                </td>
              {/each}
            </tr>
          {/each}
          {#each t.page.rows as row, r (r)}
            <tr class:del={t.deleted[r]}>
              <td class="n">
                <span class="num">{t.firstRow + r}</span>
                {#if t.canEditRows}
                  <button class="ghost icon rm" aria-label={t.deleted[r] ? 'Deshacer borrado' : 'Borrar fila'} title={t.deleted[r] ? 'Deshacer borrado' : 'Borrar fila'} onclick={() => t.toggleDelete(r)}>
                    <Icon name={t.deleted[r] ? 'undo' : 'trash'} size={12} />
                  </button>
                {/if}
              </td>
              {#each t.columns as c, ci (c.name)}
                {@const cell = { draft: null, row: r, col: c.name }}
                {@const v = t.value(r, c.name)}
                <td class:sel={sameCell(selected, cell)} class:edited={t.isEdited(r, c.name)} class:ro={!t.canEditRows || !editable(c.name)}
                    class:right={isNumber(r, ci)} title={v ?? 'NULL'}
                    onclick={() => { selected = cell; gridEl?.focus() }} ondblclick={() => startEdit(cell)}>
                  {#if sameCell(editing, cell)}
                    {@render editor()}
                  {:else if v === null}<span class="null">NULL</span>{:else}{v}{/if}
                </td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
      {#if t.page.rows.length === 0 && t.drafts.length === 0}
        <div class="empty">{t.filter ? 'Ninguna fila cumple el filtro.' : 'La tabla está vacía.'}</div>
      {/if}
    {/if}
  </div>

  <div class="foot">
    <span>{t.loc.schema ? `${t.loc.schema}.` : ''}{t.loc.table}{t.page ? ` · ${t.page.durationMs} ms` : ''}</span>
    {#if t.pending > 0}<span class="pending">{pendingText}</span>{/if}
    <span class="grow"></span>
    <span class="hint">doble clic o <kbd>Enter</kbd> edita · <kbd>Tab</kbd> siguiente · <kbd>Esc</kbd> cancela · <kbd>Ctrl</kbd>+<kbd>C</kbd> copia</span>
  </div>
  {/if}
</div>

{#snippet editor()}
  <div class="ed">
    <input use:focusSelect bind:value={editText} onkeydown={onEditKey} onblur={(e) => { if (editing && Number(e.currentTarget.dataset.seq) === editing.seq) commit() }} aria-label="Valor de la celda" />
    <button class="nullbtn" tabindex="-1" title="Poner NULL" onmousedown={(e) => { e.preventDefault(); commit(null) }}>NULL</button>
  </div>
{/snippet}

<style>
  .tv { display: flex; flex-direction: column; flex: 1; min-height: 0; background: var(--surface); }
  .modes { display: flex; align-items: center; gap: 2px; padding: 6px 10px 0; border-bottom: 1px solid var(--border); }
  .mode { border: none; border-bottom: 2px solid transparent; border-radius: 0; background: transparent; padding: 5px 12px; color: var(--muted); }
  .mode:hover:not(:disabled) { background: var(--hover); color: var(--text); }
  .mode.on { color: var(--text); border-bottom-color: var(--accent); font-weight: 600; }
  .who { margin-left: 12px; color: var(--muted); font: 12px var(--mono); }
  .pend-note { margin-left: auto; color: var(--accent); font-size: 12px; }
  .bar { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-bottom: 1px solid var(--border); flex-wrap: wrap; }
  .filter { flex: 1 1 220px; max-width: 460px; display: flex; align-items: center; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); padding-right: 2px; }
  .filter:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
  .filter input { border: none; background: transparent; box-shadow: none; font-family: var(--mono); font-size: 12px; }
  .bar select { width: auto; }
  .sep { width: 1px; align-self: stretch; background: var(--border); margin: 2px 4px; }
  .range { min-width: 72px; text-align: center; color: var(--muted); font-variant-numeric: tabular-nums; }
  .grow { flex: 1; }

  .banner { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; padding: 7px 12px; font-size: 12.5px; border-bottom: 1px solid var(--border); }
  .banner pre { margin: 0; white-space: pre-wrap; font: 12px var(--mono); }
  .banner.err { background: color-mix(in srgb, var(--danger) 12%, var(--surface)); color: var(--danger); }
  .banner.ok { background: color-mix(in srgb, var(--ok) 12%, var(--surface)); color: var(--ok); }
  .banner.warn { background: color-mix(in srgb, var(--warn) 12%, var(--surface)); color: var(--warn); }
  .banner.info { background: var(--surface-2); color: var(--muted); }

  .grid { flex: 1; overflow: auto; min-height: 0; outline: none; position: relative; background: var(--bg); }
  .grid:focus-visible { box-shadow: inset 0 0 0 2px var(--accent-soft); }
  table { border-collapse: separate; border-spacing: 0; font: 12px var(--mono); min-width: 100%; }
  table.busy { opacity: .55; }
  th { position: sticky; top: 0; z-index: 2; background: var(--surface); text-align: left; padding: 0; border-bottom: 1px solid var(--muted); border-right: 1px solid color-mix(in srgb, var(--border) 60%, transparent); }
  .hd { width: 100%; justify-content: flex-start; gap: 5px; border: none; background: transparent; border-radius: 0; padding: 3px 10px; font: 600 11.5px var(--sans); color: var(--muted); white-space: nowrap; }
  .hd:hover { background: var(--hover) !important; color: var(--text); }
  .ct { font: 10.5px var(--mono); opacity: .6; font-weight: 400; }
  .pk { color: var(--warn); display: inline-flex; }
  td { padding: 2px 10px; border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent); border-right: 1px solid color-mix(in srgb, var(--border) 60%, transparent); white-space: nowrap; max-width: 420px; overflow: hidden; text-overflow: ellipsis; cursor: default; position: relative; height: 24px; }
  td.right { text-align: right; }
  td.sel { outline: 2px solid var(--accent); outline-offset: -2px; }
  td.edited { background: color-mix(in srgb, var(--accent) 16%, transparent); box-shadow: inset 3px 0 var(--accent); }
  tbody tr:hover td:not(.edited):not(.n) { background: var(--accent-soft); }
  tr.del td:not(.n) { text-decoration: line-through; opacity: .55; background: color-mix(in srgb, var(--danger) 10%, transparent); }
  tr.draft td:not(.n) { background: color-mix(in srgb, var(--ok) 12%, transparent); }
  td.n, th.n { position: sticky; left: 0; z-index: 1; background: var(--surface); color: var(--muted); width: 1%; min-width: 58px; padding: 0 4px 0 8px; text-align: right; }
  th.n { z-index: 3; }
  .num { font-variant-numeric: tabular-nums; }
  .rm { opacity: 0; padding: 2px; margin-left: 2px; vertical-align: middle; }
  tr:hover .rm, .rm:focus-visible, tr.del .rm { opacity: 1; }
  .null { color: var(--muted); font-style: italic; opacity: .75; }
  .dflt { color: var(--muted); font-style: italic; opacity: .6; font-size: 11px; }

  .ed { display: flex; align-items: center; position: absolute; inset: 0; background: var(--surface); }
  .ed input { height: 100%; border-radius: 0; border: 2px solid var(--accent); font: 12px var(--mono); padding: 0 52px 0 10px; box-shadow: none; }
  .nullbtn { position: absolute; right: 3px; padding: 0 6px; font: 600 10px var(--mono); color: var(--muted); }

  .empty { padding: 28px; text-align: center; color: var(--muted); }
  .foot { display: flex; align-items: center; gap: 14px; padding: 4px 12px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; background: var(--surface-2); }
  .pending { color: var(--accent); font-weight: 600; }
  .hint { opacity: .8; }
</style>
