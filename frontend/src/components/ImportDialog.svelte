<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errMsg, type CsvPreview, type ImportRequest } from '../lib/api'
  import { ui, type ImportTarget } from '../lib/ui.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'

  let { target, oncancel }: { target: ImportTarget; oncancel: () => void } = $props()

  let path = $state('')
  let hasHeader = $state(true)
  let delimiter = $state('') // vacío = detectar
  let emptyAsNull = $state(false)
  let preview = $state<CsvPreview | null>(null)
  let mapping = $state<Record<string, number>>({}) // columna de la tabla → índice de columna del CSV (-1 = no importar)
  let error = $state('')
  let busy = $state(false)
  let phase = $state<'form' | 'running'>('form')
  let rows = $state(0)
  let jobId = ''

  // Normaliza para comparar nombres: sin acentos, mayúsculas ni signos ("Fecha_Alta" ≈ "fecha alta").
  const norm = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase().replace(/[^a-z0-9]/g, '')

  function autoMap(p: CsvPreview) {
    const m: Record<string, number> = {}
    for (const c of target.columns) {
      const i = hasHeader ? p.header.findIndex((h) => norm(h) === norm(c.name)) : -1
      m[c.name] = i
    }
    mapping = m
  }

  /** Mapea por posición, saltando las columnas que el motor genera solo. */
  function mapByPosition() {
    if (!preview) return
    const m: Record<string, number> = {}
    let i = 0
    for (const c of target.columns) {
      if (c.autoIncrement || i >= preview.columns) { m[c.name] = -1; continue }
      m[c.name] = i++
    }
    mapping = m
  }

  async function loadPreview() {
    if (!path) return
    busy = true
    error = ''
    try {
      const p = await api.importPreview(path, delimiter, hasHeader)
      p.rows = p.rows ?? []
      p.header = p.header ?? []
      p.warnings = p.warnings ?? []
      preview = p
      autoMap(p)
    } catch (e) {
      preview = null
      error = errMsg(e)
    }
    busy = false
  }

  async function choose() {
    try {
      const p = await api.chooseCsvFile()
      if (!p) return
      path = p
      await loadPreview()
    } catch (e) { error = errMsg(e) }
  }

  const mapped = $derived(Object.values(mapping).filter((v) => v >= 0).length)
  const sample = (col: string) => { const i = mapping[col]; return i >= 0 && preview?.rows[0] ? (preview.rows[0][i] ?? '') : '' }
  const basename = (p: string) => p.split(/[\\/]/).pop() ?? p

  async function run() {
    if (!preview) return
    phase = 'running'
    rows = 0
    error = ''
    jobId = crypto.randomUUID()
    const req = {
      table: target.loc, path, delimiter: preview.delimiter, hasHeader, emptyAsNull,
      mapping: target.columns.map((c) => ({ column: c.name, source: mapping[c.name] ?? -1 })),
    } as unknown as ImportRequest
    const off = EventsOn('import:progress', (e: { id: string; rows: number }) => { if (e.id === jobId) rows = e.rows })
    try {
      const res = await api.importRun(jobId, req)
      if (res.cancelled) { phase = 'form'; ui.toast('Importación cancelada: no se insertó ninguna fila'); return }
      ui.toast(`Importadas ${res.inserted.toLocaleString()} fila(s) en ${target.loc.table}`)
      target.onDone()
      oncancel()
    } catch (e) {
      error = errMsg(e)
      phase = 'form'
    } finally { off() }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && phase !== 'running' && oncancel()} />

<div class="overlay" role="presentation" onclick={() => phase !== 'running' && oncancel()}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="im-title" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <h3 id="im-title">Importar CSV en {target.loc.table}</h3>

    {#if phase === 'running'}
      <p class="run"><span class="spin"></span> Importando… {rows.toLocaleString()} fila(s)</p>
      <p class="hint">Todo se inserta en una transacción: si algo falla o cancelas, no queda nada a medias.</p>
      <div class="actions"><button onclick={() => api.importCancel(jobId)}>Cancelar importación</button></div>
    {:else}
      <div class="file">
        <button onclick={choose} disabled={busy}><Icon name="folder" size={14} /> {path ? 'Cambiar archivo…' : 'Elegir archivo CSV…'}</button>
        {#if path}<span class="fname" title={path}>{basename(path)}</span>{/if}
      </div>

      {#if path}
        <div class="opts">
          <label class="check"><input type="checkbox" bind:checked={hasHeader} onchange={loadPreview} /> La primera fila es la cabecera</label>
          <label>Separador
            <select bind:value={delimiter} onchange={loadPreview}>
              <option value="">Detectar automáticamente</option><option value=",">Coma ( , )</option><option value=";">Punto y coma ( ; )</option><option value="\t">Tabulador</option><option value="|">Barra ( | )</option>
            </select>
          </label>
        </div>
      {/if}

      {#if preview}
        {#each preview.warnings as w}<p class="warn">⚠ {w}</p>{/each}
        <div class="sect">Vista previa <span class="muted">({preview.rows.length} primeras filas, {preview.columns} columna(s))</span></div>
        <div class="pv">
          <table>
            <thead><tr>{#each preview.header as h, i (i)}<th>{h}</th>{/each}</tr></thead>
            <tbody>{#each preview.rows.slice(0, 6) as r}<tr>{#each preview.header as _, i (i)}<td title={r[i] ?? ''}>{r[i] ?? ''}</td>{/each}</tr>{/each}</tbody>
          </table>
        </div>

        <div class="sect">
          Mapeo de columnas
          <span class="grow"></span>
          <button class="ghost" type="button" onclick={mapByPosition}>Por posición</button>
          <button class="ghost" type="button" onclick={() => preview && autoMap(preview)}>Por nombre</button>
        </div>
        <div class="map">
          {#each target.columns as c (c.name)}
            <div class="mrow">
              <span class="cname">{c.name}<small>{c.type}{c.primaryKey ? ' · PK' : ''}{c.autoIncrement ? ' · AUTO' : ''}</small></span>
              <select bind:value={mapping[c.name]} aria-label="Columna del CSV para {c.name}">
                <option value={-1}>(no importar{c.autoIncrement ? ': lo genera el motor' : ': valor por defecto'})</option>
                {#each preview.header as h, i (i)}<option value={i}>{h}</option>{/each}
              </select>
              <span class="sample" title={sample(c.name)}>{sample(c.name)}</span>
            </div>
          {/each}
        </div>
        <label class="check"><input type="checkbox" bind:checked={emptyAsNull} /> Tratar el texto vacío como NULL también en columnas de texto</label>
        <p class="hint">En columnas que no son de texto (números, fechas…) el vacío siempre se importa como NULL.</p>
      {/if}

      {#if error}<p class="err" role="alert">{error}</p>{/if}
      <div class="actions">
        <button onclick={oncancel}>Cancelar</button>
        <button class="primary" onclick={run} disabled={!preview || mapped === 0}>Importar{mapped ? ` (${mapped} columna${mapped === 1 ? '' : 's'})` : ''}</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 20; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow); padding: 18px 20px; width: 760px; max-width: 94vw; max-height: 92vh; overflow: auto; display: flex; flex-direction: column; gap: 10px; }
  h3 { margin: 0; font-size: 15px; }
  .file { display: flex; align-items: center; gap: 10px; }
  .fname { font: 12px var(--mono); color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .opts { display: flex; gap: 18px; align-items: flex-end; flex-wrap: wrap; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); }
  label.check { flex-direction: row; align-items: center; gap: 8px; color: var(--text); }
  label.check input { width: auto; }
  .sect { display: flex; align-items: center; gap: 8px; font-size: 12px; font-weight: 600; text-transform: uppercase; letter-spacing: .05em; color: var(--muted); margin-top: 4px; }
  .muted { font-weight: 400; text-transform: none; letter-spacing: 0; }
  .grow { flex: 1; }
  .sect button { text-transform: none; letter-spacing: 0; font-weight: 400; font-size: 12px; }
  .pv { overflow: auto; max-height: 170px; border: 1px solid var(--border); border-radius: var(--radius); }
  table { border-collapse: collapse; font: 11.5px var(--mono); }
  th { position: sticky; top: 0; background: var(--surface-2); text-align: left; font: 600 11.5px var(--sans); color: var(--muted); }
  th, td { padding: 3px 10px; border-bottom: 1px solid var(--border); border-right: 1px solid var(--border); white-space: nowrap; max-width: 220px; overflow: hidden; text-overflow: ellipsis; }
  .map { border: 1px solid var(--border); border-radius: var(--radius); max-height: 230px; overflow: auto; }
  .mrow { display: grid; grid-template-columns: 1.2fr 1.4fr 1fr; gap: 10px; align-items: center; padding: 4px 10px; border-bottom: 1px solid var(--border); }
  .mrow:last-child { border-bottom: none; }
  .cname { font-weight: 600; display: flex; flex-direction: column; }
  .cname small { font: 11px var(--mono); color: var(--muted); font-weight: 400; }
  .sample { font: 11.5px var(--mono); color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .warn { margin: 0; color: var(--warn); font-size: 12.5px; }
  .hint { margin: 0; color: var(--muted); font-size: 12px; }
  .err { margin: 0; color: var(--danger); white-space: pre-wrap; word-break: break-word; }
  .run { display: flex; align-items: center; gap: 10px; margin: 6px 0 0; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px; }
  .spin { width: 14px; height: 14px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
