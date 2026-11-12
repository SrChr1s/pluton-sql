<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import Icon from './Icon.svelte'
  import { api, errMsg, type ScriptObject, type ScriptProgress, type ScriptRequest, type ScriptResult } from '../lib/api'
  import { ui, type ScriptSource } from '../lib/ui.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { EventsOn } from '../../wailsjs/runtime/runtime'

  let { source, oncancel }: { source: ScriptSource; oncancel: () => void } = $props()

  const KINDS = [['table', 'Tablas'], ['view', 'Vistas'], ['procedure', 'Procedimientos'], ['function', 'Funciones']] as const
  const key = (o: ScriptObject) => [o.kind, o.schema, o.name, o.id].join('\u0000')
  const label = (o: ScriptObject) => (o.schema ? `${o.schema}.${o.name}` : o.name) + (o.args ? `(${o.args})` : '')

  let objects = $state<ScriptObject[] | null>(null)
  let loadError = $state('')
  // El diálogo se remonta (#key) cada vez que se abre: la preselección inicial no necesita reaccionar a `source`.
  let scope = $state<'all' | 'select'>(untrack(() => (source.object || source.schema !== undefined ? 'select' : 'all')))
  let picked = $state<Record<string, boolean>>({})
  let filter = $state('')
  let mode = $state<'both' | 'schema' | 'data'>('both')
  let includeDrop = $state(false)
  let includeSchemas = $state(true)
  let includeIndexes = $state(true)
  let includeForeignKeys = $state(true)
  let dest = $state<'file' | 'editor'>('file')

  let phase = $state<'form' | 'running' | 'done'>('form')
  let progress = $state<ScriptProgress | null>(null)
  let error = $state('')
  let result = $state<ScriptResult | null>(null)
  let jobId = ''

  const schemaOn = $derived(mode !== 'data')
  const q = $derived(filter.trim().toLowerCase())
  const visible = (kind: string) => (objects ?? []).filter((o) => o.kind === kind && (!q || label(o).toLowerCase().includes(q)))
  // En modo "solo datos" únicamente cuentan las tablas.
  const relevant = (o: ScriptObject) => mode !== 'data' || o.kind === 'table'
  const chosen = $derived((objects ?? []).filter((o) => picked[key(o)] && relevant(o)))
  const canStart = $derived(!!objects && (scope === 'all' ? objects.length > 0 : chosen.length > 0))

  onMount(async () => {
    try {
      const all = await api.scriptObjects(source.profile, source.database)
      const sel: Record<string, boolean> = {}
      for (const o of all) {
        if (source.object) sel[key(o)] = o.kind === source.object.kind && o.schema === source.object.schema && o.name === source.object.name && o.id === source.object.id
        else if (source.schema !== undefined) sel[key(o)] = o.schema === source.schema
      }
      picked = sel
      objects = all
    } catch (e) {
      loadError = errMsg(e)
    }
  })

  function setGroup(kind: string, on: boolean) {
    for (const o of visible(kind)) picked[key(o)] = on
  }

  const size = (n: number) => (n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)
  const pct = $derived(progress && progress.total ? Math.min(100, Math.round((100 * progress.done) / progress.total)) : 0)

  async function start() {
    error = ''
    try {
      let path = ''
      if (dest === 'file') {
        path = await api.chooseExportFile(`${source.database || 'script'}.sql`, 'sql')
        if (!path) return // cancelado en el diálogo del sistema
      }
      phase = 'running'
      progress = null
      jobId = crypto.randomUUID()
      const req = {
        profile: source.profile, database: source.database, path,
        options: {
          objects: scope === 'all' ? [] : chosen, mode, includeDrop, includeSchemas, includeIndexes, includeForeignKeys, batchRows: 100,
        },
      } as ScriptRequest
      const off = EventsOn('script:progress', (e: ScriptProgress) => { if (e.id === jobId) progress = e })
      try { result = await api.scriptGenerate(jobId, req) } finally { off() }
      if (result.cancelled) { phase = 'form'; ui.toast('Generación cancelada'); return }
      if (dest === 'editor') {
        ws.newTab({ title: `${source.database || 'script'}.sql`, database: source.database, sql: result.script })
        ui.toast(result.errors?.length ? `Script abierto con ${result.errors.length} error(es); busca "-- ERROR"` : 'Script abierto en una pestaña nueva', result.errors?.length ? 'err' : 'ok')
        oncancel()
        return
      }
      phase = 'done'
    } catch (e) {
      error = errMsg(e)
      phase = 'form'
    }
  }

  const cancelJob = () => api.scriptCancel(jobId)
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && phase !== 'running' && oncancel()} />

<div class="overlay" role="presentation" onclick={() => phase !== 'running' && oncancel()}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="sc-title" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <h3 id="sc-title">Generar scripts{source.database ? ` de ${source.database}` : ''}</h3>

    {#if phase === 'form'}
      <form onsubmit={(e) => { e.preventDefault(); if (canStart) start() }}>
        <fieldset>
          <legend>Objetos</legend>
          <label class="radio"><input type="radio" bind:group={scope} value="all" /><span><b>Toda la base de datos</b> <small>Todas las tablas, vistas, procedimientos y funciones.</small></span></label>
          <label class="radio"><input type="radio" bind:group={scope} value="select" /><span><b>Elegir objetos</b></span></label>

          {#if scope === 'select'}
            {#if loadError}
              <p class="err" role="alert">{loadError}</p>
            {:else if !objects}
              <p class="run"><span class="orbit"></span> Leyendo objetos…</p>
            {:else}
              <input class="filter" placeholder="Filtrar objetos…" bind:value={filter} aria-label="Filtrar objetos" />
              <div class="list">
                {#each KINDS as [kind, title] (kind)}
                  {@const items = visible(kind)}
                  {#if items.length}
                    <div class="grp">
                      <span>{title} <small>({items.length})</small></span>
                      <span class="all">
                        <button type="button" class="ghost" onclick={() => setGroup(kind, true)}>Todos</button>
                        <button type="button" class="ghost" onclick={() => setGroup(kind, false)}>Ninguno</button>
                      </span>
                    </div>
                    {#each items as o (key(o))}
                      <label class="check item" class:off={!relevant(o)}><input type="checkbox" bind:checked={picked[key(o)]} disabled={!relevant(o)} /> <span class="mono">{label(o)}</span></label>
                    {/each}
                  {/if}
                {:else}
                  <p class="empty">{q ? 'Sin coincidencias' : 'La base no tiene objetos.'}</p>
                {/each}
              </div>
              <p class="count">{chosen.length} objeto(s) seleccionado(s){mode === 'data' ? ' · en modo "solo datos" solo se usan las tablas' : ''}</p>
            {/if}
          {/if}
        </fieldset>

        <fieldset>
          <legend>Contenido</legend>
          <label class="radio"><input type="radio" bind:group={mode} value="both" /><span><b>Esquema y datos</b> <small>Crea los objetos y los rellena con INSERT.</small></span></label>
          <label class="radio"><input type="radio" bind:group={mode} value="schema" /><span><b>Solo esquema</b> <small>CREATE de tablas, vistas, procedimientos y funciones.</small></span></label>
          <label class="radio"><input type="radio" bind:group={mode} value="data" /><span><b>Solo datos</b> <small>INSERT de las tablas, en orden de claves foráneas.</small></span></label>
        </fieldset>

        <fieldset disabled={!schemaOn}>
          <legend>Opciones del esquema</legend>
          <label class="check"><input type="checkbox" bind:checked={includeDrop} /> Borrar antes cada objeto si existe (DROP … IF EXISTS)</label>
          <label class="check"><input type="checkbox" bind:checked={includeSchemas} /> Crear los esquemas que no existan</label>
          <label class="check"><input type="checkbox" bind:checked={includeIndexes} /> Incluir índices</label>
          <label class="check"><input type="checkbox" bind:checked={includeForeignKeys} /> Incluir claves foráneas</label>
        </fieldset>

        <fieldset>
          <legend>Destino</legend>
          <label class="radio"><input type="radio" bind:group={dest} value="file" /><span><b>Guardar en un archivo .sql</b> <small>Recomendado para bases grandes.</small></span></label>
          <label class="radio"><input type="radio" bind:group={dest} value="editor" /><span><b>Abrir en una pestaña nueva</b> <small>Hasta 8 MB; si el script es mayor, guárdalo en un archivo.</small></span></label>
        </fieldset>

        {#if error}<p class="err" role="alert">{error}</p>{/if}
        <div class="actions">
          <button type="button" onclick={oncancel}>Cancelar</button>
          <button type="submit" class="primary" disabled={!canStart}>{dest === 'file' ? 'Generar…' : 'Generar'}</button>
        </div>
      </form>
    {:else if phase === 'running'}
      <p class="run"><span class="orbit"></span>
        {#if progress}{progress.phase === 'data' ? 'Datos' : 'Esquema'} · <span class="mono">{progress.object}</span> ({progress.done}/{progress.total}){#if progress.rows} · {progress.rows.toLocaleString()} fila(s){/if}
        {:else}Generando…{/if}
      </p>
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax="100" aria-valuenow={pct}><span style:width="{pct}%"></span></div>
      <div class="actions"><button onclick={cancelJob}>Cancelar</button></div>
    {:else if result}
      <p class="ok"><Icon name="check" size={14} /> {result.objects} objeto(s) · {result.rows.toLocaleString()} fila(s) · {size(result.bytes)} · {(result.durationMs / 1000).toFixed(1)} s</p>
      <p class="path">{result.path}</p>
      {#if result.errors?.length}
        <p class="err">{result.errors.length} objeto(s) no se pudieron escribir (el script lleva un comentario "-- ERROR" en su lugar):</p>
        <ul class="errs">{#each result.errors as e}<li>{e}</li>{/each}</ul>
      {/if}
      <div class="actions"><button class="primary" onclick={oncancel}>Cerrar</button></div>
    {/if}
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 20; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow); padding: 18px 20px; width: 560px; max-width: 94vw; max-height: 92vh; overflow: auto; }
  h3 { margin: 0 0 10px; font-size: 15px; }
  form { display: flex; flex-direction: column; gap: 10px; }
  fieldset { border: 1px solid var(--border); border-radius: var(--radius); padding: 6px 10px 8px; margin: 0; display: flex; flex-direction: column; gap: 6px; }
  fieldset:disabled { opacity: .5; }
  legend { color: var(--muted); font-size: 12px; padding: 0 4px; }
  .radio { display: flex; gap: 8px; align-items: flex-start; flex-direction: row; color: var(--text); }
  .radio input { width: auto; margin-top: 3px; }
  .radio small { display: block; color: var(--muted); }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); }
  label.check { flex-direction: row; align-items: center; gap: 8px; color: var(--text); }
  label.check input { width: auto; }
  .item { padding: 1px 4px; border-radius: 4px; }
  .item:hover { background: var(--hover); }
  .item.off { opacity: .45; }
  .filter { margin-top: 2px; }
  .list { max-height: 210px; overflow: auto; border: 1px solid var(--border); border-radius: var(--radius); padding: 4px 6px; background: var(--bg); }
  .grp { display: flex; justify-content: space-between; align-items: center; margin-top: 6px; font-weight: 600; font-size: 12px; color: var(--muted); }
  .grp small { font-weight: 400; }
  .grp:first-child { margin-top: 0; }
  .all { display: inline-flex; gap: 2px; }
  .all button { padding: 1px 6px; font-size: 11.5px; }
  .count { margin: 0; color: var(--muted); font-size: 12px; }
  .empty { margin: 6px; color: var(--muted); }
  .mono { font-family: var(--mono); font-size: 12px; }
  .err { margin: 0; color: var(--danger); white-space: pre-wrap; word-break: break-word; }
  .errs { margin: 6px 0 0; padding-left: 18px; color: var(--danger); font: 12px var(--mono); max-height: 140px; overflow: auto; word-break: break-word; }
  .ok { margin: 0; color: var(--ok); display: flex; align-items: center; gap: 6px; font-weight: 600; }
  .path { margin: 6px 0 0; font: 12px var(--mono); color: var(--muted); word-break: break-all; }
  .run { display: flex; align-items: center; gap: 10px; margin: 6px 0; word-break: break-all; }
  .bar { height: 5px; border-radius: 3px; background: var(--surface-2); overflow: hidden; }
  .bar span { display: block; height: 100%; background: var(--accent); transition: width .2s; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
</style>
