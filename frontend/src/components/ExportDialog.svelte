<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errMsg, type ExportRequest, type ExportResult } from '../lib/api'
  import { ui, type ExportSource } from '../lib/ui.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'

  let { source, oncancel }: { source: ExportSource; oncancel: () => void } = $props()

  type Format = 'csv' | 'json' | 'sql'
  let format = $state<Format>('csv')
  let delimiter = $state(',')
  let header = $state(true)
  let bom = $state(false)
  let sqlTable = $state(source.kind === 'table' ? source.table : 'tabla')
  let phase = $state<'form' | 'running' | 'done'>('form')
  let rows = $state(0)
  let error = $state('')
  let result = $state<ExportResult | null>(null)
  let jobId = ''

  const ext: Record<Format, string> = { csv: 'csv', json: 'json', sql: 'sql' }
  const formats: [Format, string, string][] = [
    ['csv', 'CSV', 'Texto separado por comas; se abre en Excel y en cualquier herramienta de datos.'],
    ['json', 'JSON', 'Un array de objetos; los números y los NULL conservan su tipo.'],
    ['sql', 'SQL (INSERT)', 'Sentencias INSERT por lotes para recrear los datos en otra base.'],
  ]

  const size = (n: number) => (n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)

  async function start() {
    error = ''
    try {
      const path = await api.chooseExportFile(`${source.name}.${ext[format]}`, format)
      if (!path) return // cancelado en el diálogo del sistema
      phase = 'running'
      rows = 0
      jobId = crypto.randomUUID()
      const req = {
        profile: source.profile, database: source.database, kind: source.kind,
        sql: source.kind === 'query' ? source.sql : '',
        schema: source.kind === 'table' ? source.schema : '', table: source.kind === 'table' ? source.table : '',
        filter: source.kind === 'table' ? source.filter : '', orderBy: source.kind === 'table' ? source.orderBy : '', desc: source.kind === 'table' ? source.desc : false,
        format, path, delimiter, header, bom, sqlTable,
      } as ExportRequest
      const off = EventsOn('export:progress', (e: { id: string; rows: number }) => { if (e.id === jobId) rows = e.rows })
      try { result = await api.exportRun(jobId, req) } finally { off() }
      if (result.cancelled) { phase = 'form'; ui.toast('Exportación cancelada'); return }
      phase = 'done'
      ui.toast(`Exportadas ${result.rows} fila(s) a ${result.path}`)
    } catch (e) {
      error = errMsg(e)
      phase = 'form'
    }
  }

  const cancelJob = () => api.exportCancel(jobId)
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && phase !== 'running' && oncancel()} />

<div class="overlay" role="presentation" onclick={() => phase !== 'running' && oncancel()}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="ex-title" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <h3 id="ex-title">Exportar {source.kind === 'table' ? `la tabla ${source.table}` : 'el resultado'}</h3>

    {#if phase === 'form'}
      <form onsubmit={(e) => { e.preventDefault(); start() }}>
        <p class="note">
          {#if source.kind === 'query'}Se vuelve a ejecutar la consulta en el servidor y se exportan <b>todas</b> las filas, no solo las de la vista.
          {:else}Se exportan <b>todas</b> las filas de la tabla{source.filter ? ' que cumplen el filtro actual' : ''}, en el orden actual.{/if}
        </p>
        {#if source.kind === 'query'}<pre class="q">{source.sql.length > 300 ? source.sql.slice(0, 300) + '…' : source.sql}</pre>{/if}

        <fieldset>
          <legend>Formato</legend>
          {#each formats as [f, label, hint]}
            <label class="radio"><input type="radio" bind:group={format} value={f} /><span><b>{label}</b> <small>{hint}</small></span></label>
          {/each}
        </fieldset>

        {#if format === 'csv'}
          <div class="row">
            <label>Separador
              <select bind:value={delimiter}>
                <option value=",">Coma ( , )</option><option value=";">Punto y coma ( ; )</option><option value="\t">Tabulador</option><option value="|">Barra ( | )</option>
              </select>
            </label>
          </div>
          <label class="check"><input type="checkbox" bind:checked={header} /> Primera fila con los nombres de columna</label>
          <label class="check"><input type="checkbox" bind:checked={bom} /> Compatible con Excel (UTF-8 con BOM, para que se vean bien los acentos)</label>
        {:else if format === 'sql' && source.kind === 'query'}
          <label>Nombre de la tabla en los INSERT <input bind:value={sqlTable} class="mono" /></label>
        {/if}

        {#if error}<p class="err" role="alert">{error}</p>{/if}
        <div class="actions">
          <button type="button" onclick={oncancel}>Cancelar</button>
          <button type="submit" class="primary">Exportar…</button>
        </div>
      </form>
    {:else if phase === 'running'}
      <p class="run"><span class="spin"></span> Exportando… {rows.toLocaleString()} fila(s)</p>
      <div class="actions"><button onclick={cancelJob}>Cancelar exportación</button></div>
    {:else if result}
      <p class="ok"><Icon name="check" size={14} /> {result.rows.toLocaleString()} fila(s) · {size(result.bytes)} · {(result.durationMs / 1000).toFixed(1)} s</p>
      <p class="path">{result.path}</p>
      <div class="actions"><button class="primary" onclick={oncancel}>Cerrar</button></div>
    {/if}
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 20; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow); padding: 18px 20px; width: 520px; max-width: 94vw; max-height: 92vh; overflow: auto; }
  h3 { margin: 0 0 10px; font-size: 15px; }
  form { display: flex; flex-direction: column; gap: 10px; }
  .note { margin: 0; color: var(--muted); }
  .q { margin: 0; padding: 8px 10px; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); font: 11.5px/1.5 var(--mono); white-space: pre-wrap; max-height: 8em; overflow: auto; }
  fieldset { border: 1px solid var(--border); border-radius: var(--radius); padding: 6px 10px 8px; margin: 0; display: flex; flex-direction: column; gap: 6px; }
  legend { color: var(--muted); font-size: 12px; padding: 0 4px; }
  .radio { display: flex; gap: 8px; align-items: flex-start; flex-direction: row; color: var(--text); }
  .radio input { width: auto; margin-top: 3px; }
  .radio small { display: block; color: var(--muted); }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); }
  label.check { flex-direction: row; align-items: center; gap: 8px; color: var(--text); }
  label.check input { width: auto; }
  .row { display: flex; gap: 10px; } .row label { flex: 1; }
  .mono { font-family: var(--mono); font-size: 12px; }
  .err { margin: 0; color: var(--danger); white-space: pre-wrap; word-break: break-word; }
  .ok { margin: 0; color: var(--ok); display: flex; align-items: center; gap: 6px; font-weight: 600; }
  .path { margin: 6px 0 0; font: 12px var(--mono); color: var(--muted); word-break: break-all; }
  .run { display: flex; align-items: center; gap: 10px; margin: 6px 0; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
  .spin { width: 14px; height: 14px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
