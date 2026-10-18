<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errMsg, type HistoryEntry, type SavedQuery } from '../lib/api'
  import { ws } from '../lib/workspace.svelte'
  import { ui } from '../lib/ui.svelte'
  import { copyText } from '../lib/clipboard'

  let { onclose }: { onclose: () => void } = $props()

  let view = $state<'history' | 'saved'>('history')
  let search = $state('')
  let entries = $state<HistoryEntry[]>([])
  let saved = $state<SavedQuery[]>([])
  let loading = $state(false)

  const profile = $derived(ws.active?.id ?? '')

  async function loadHistory() {
    if (!profile) return
    loading = true
    try { entries = await api.historyList(profile, search, 200) }
    catch (e) { ui.toast(`No se pudo leer el historial: ${errMsg(e)}`, 'err') }
    loading = false
  }
  async function loadSaved() {
    try { saved = await api.savedList() } catch (e) { ui.toast(`No se pudieron leer las consultas guardadas: ${errMsg(e)}`, 'err') }
  }

  // Se recarga al abrir, al cambiar de conexión, tras cada ejecución y al buscar (con un pequeño retardo).
  $effect(() => {
    void profile; void ws.historyVersion; const q = search
    const h = setTimeout(loadHistory, q ? 200 : 0)
    return () => clearTimeout(h)
  })
  $effect(() => { if (view === 'saved') void loadSaved() })

  /** Hora relativa corta: "hace 5 s", "hace 3 min", "hace 2 h"… y la fecha si pasó más de una semana. */
  function ago(at: unknown): string {
    const d = new Date(at as string)
    const s = Math.max(0, Math.round((Date.now() - d.getTime()) / 1000))
    if (s < 60) return `hace ${s} s`
    if (s < 3600) return `hace ${Math.floor(s / 60)} min`
    if (s < 86400) return `hace ${Math.floor(s / 3600)} h`
    if (s < 7 * 86400) return `hace ${Math.floor(s / 86400)} d`
    return d.toLocaleDateString()
  }
  const preview = (sql: string) => sql.split('\n').slice(0, 3).join('\n')
  const firstLine = (sql: string) => sql.trim().split('\n')[0].slice(0, 40)

  const openEntry = (e: HistoryEntry) => ws.openSql(e.sql, firstLine(e.sql) || 'Consulta')
  const openSaved = (q: SavedQuery) => ws.openSql(q.sql, q.name)

  async function saveFromEntry(e: HistoryEntry) {
    const name = await ui.prompt({ title: 'Guardar consulta', label: 'Nombre', value: firstLine(e.sql), confirmLabel: 'Guardar' })
    if (!name) return
    try { await api.savedSave({ id: '', name, sql: e.sql, driver: ws.active?.driver ?? '' } as SavedQuery); ui.toast(`Guardada como "${name}"`) }
    catch (err) { ui.toast(errMsg(err), 'err') }
  }

  async function saveCurrent() {
    const t = ws.queryTab
    if (!t || !t.sql.trim()) return ui.toast('La pestaña actual no tiene SQL que guardar', 'err')
    const name = await ui.prompt({ title: 'Guardar consulta', label: 'Nombre', value: t.title, confirmLabel: 'Guardar' })
    if (!name) return
    try { await api.savedSave({ id: '', name, sql: t.sql, driver: ws.active?.driver ?? '' } as SavedQuery); ui.toast(`Guardada como "${name}"`); await loadSaved() }
    catch (err) { ui.toast(errMsg(err), 'err') }
  }

  async function renameSaved(q: SavedQuery) {
    const name = await ui.prompt({ title: 'Renombrar consulta', label: 'Nombre', value: q.name, confirmLabel: 'Renombrar' })
    if (!name || name === q.name) return
    try { await api.savedSave({ ...q, name } as SavedQuery); await loadSaved() } catch (err) { ui.toast(errMsg(err), 'err') }
  }

  async function removeSaved(q: SavedQuery) {
    if (!(await ui.confirm({ title: `Eliminar "${q.name}"`, message: 'La consulta guardada se borrará. No afecta a las pestañas ya abiertas.', confirmLabel: 'Eliminar', danger: true }))) return
    try { await api.savedDelete(q.id); await loadSaved() } catch (err) { ui.toast(errMsg(err), 'err') }
  }

  async function removeEntry(e: HistoryEntry) {
    try { await api.historyDelete(e.id); await loadHistory() } catch (err) { ui.toast(errMsg(err), 'err') }
  }

  async function clearAll() {
    if (!(await ui.confirm({ title: 'Borrar el historial', message: `Se borrarán todas las ejecuciones guardadas de "${ws.active?.name}".`, confirmLabel: 'Borrar historial', danger: true }))) return
    try { await api.historyClear(profile); await loadHistory() } catch (err) { ui.toast(errMsg(err), 'err') }
  }

  const copy = async (sql: string) => { await copyText(sql); ui.toast('SQL copiado') }
</script>

<aside class="hp" aria-label="Historial y consultas guardadas">
  <div class="top">
    <div class="seg" role="tablist">
      <button role="tab" class:on={view === 'history'} aria-selected={view === 'history'} onclick={() => (view = 'history')}><Icon name="clock" size={13} /> Historial</button>
      <button role="tab" class:on={view === 'saved'} aria-selected={view === 'saved'} onclick={() => (view = 'saved')}><Icon name="bookmark" size={13} /> Guardadas</button>
    </div>
    <button class="ghost icon" aria-label="Cerrar el panel" title="Cerrar (Ctrl+H)" onclick={onclose}><Icon name="x" size={14} /></button>
  </div>

  {#if view === 'history'}
    <label class="search">
      <Icon name="search" size={14} />
      <input placeholder="Buscar en el historial…" bind:value={search} aria-label="Buscar en el historial" />
      {#if search}<button class="ghost icon" aria-label="Limpiar búsqueda" onclick={() => (search = '')}><Icon name="x" size={12} /></button>{/if}
    </label>
    <div class="list">
      {#each entries as e (e.id)}
        <div class="item" class:fail={!!e.error} role="button" tabindex="0" title="Abrir en una pestaña nueva"
             onclick={() => openEntry(e)} onkeydown={(ev) => ev.key === 'Enter' && openEntry(e)}>
          <pre>{preview(e.sql)}</pre>
          <div class="meta">
            <span>{ago(e.at)}</span>
            <span>{e.durationMs} ms</span>
            {#if e.error}<span class="bad" title={e.error}>error</span>
            {:else}<span>{e.rows} {e.isQuery ? 'fila(s)' : 'afectada(s)'}</span>{/if}
            {#if e.count > 1}<span class="x">×{e.count}</span>{/if}
            <span class="acts">
              <button class="ghost icon" title="Copiar SQL" aria-label="Copiar SQL" onclick={(ev) => { ev.stopPropagation(); copy(e.sql) }}><Icon name="copy" size={12} /></button>
              <button class="ghost icon" title="Guardar como consulta" aria-label="Guardar como consulta" onclick={(ev) => { ev.stopPropagation(); saveFromEntry(e) }}><Icon name="bookmark" size={12} /></button>
              <button class="ghost icon danger" title="Quitar del historial" aria-label="Quitar del historial" onclick={(ev) => { ev.stopPropagation(); removeEntry(e) }}><Icon name="trash" size={12} /></button>
            </span>
          </div>
          {#if e.error}<div class="errline">{e.error}</div>{/if}
        </div>
      {:else}
        <p class="none">{loading ? 'Cargando…' : search ? 'Nada coincide con la búsqueda.' : 'Todavía no has ejecutado consultas en esta conexión.'}</p>
      {/each}
    </div>
    {#if entries.length}<div class="foot"><button class="ghost danger" onclick={clearAll}><Icon name="trash" size={12} /> Borrar historial</button></div>{/if}
  {:else}
    <div class="save"><button onclick={saveCurrent}><Icon name="plus" size={13} /> Guardar la consulta actual…</button></div>
    <div class="list">
      {#each saved as q (q.id)}
        <div class="item" role="button" tabindex="0" title="Abrir en una pestaña nueva" onclick={() => openSaved(q)} onkeydown={(ev) => ev.key === 'Enter' && openSaved(q)}>
          <div class="name">{q.name}{#if q.driver}<span class="drv">{q.driver}</span>{/if}</div>
          <pre>{preview(q.sql)}</pre>
          <div class="meta"><span class="acts always">
            <button class="ghost icon" title="Copiar SQL" aria-label="Copiar SQL" onclick={(ev) => { ev.stopPropagation(); copy(q.sql) }}><Icon name="copy" size={12} /></button>
            <button class="ghost icon" title="Renombrar" aria-label="Renombrar" onclick={(ev) => { ev.stopPropagation(); renameSaved(q) }}><Icon name="pencil" size={12} /></button>
            <button class="ghost icon danger" title="Eliminar" aria-label="Eliminar" onclick={(ev) => { ev.stopPropagation(); removeSaved(q) }}><Icon name="trash" size={12} /></button>
          </span></div>
        </div>
      {:else}
        <p class="none">No hay consultas guardadas. Guarda la actual con el botón de arriba, o desde una entrada del historial.</p>
      {/each}
    </div>
  {/if}
</aside>

<style>
  .hp { width: 340px; flex: none; display: flex; flex-direction: column; min-height: 0; border-left: 1px solid var(--border); background: var(--surface); }
  .top { display: flex; align-items: center; justify-content: space-between; padding: 6px 8px; border-bottom: 1px solid var(--border); }
  .seg { display: flex; border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; }
  .seg button { border: none; border-radius: 0; background: transparent; padding: 3px 10px; }
  .seg button.on { background: var(--accent); color: var(--accent-fg); font-weight: 600; }
  .search { display: flex; align-items: center; gap: 6px; margin: 8px; padding: 0 4px 0 8px; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); color: var(--muted); }
  .search:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
  .search input { border: none; background: transparent; box-shadow: none; padding: 5px 0; }
  .search input:focus { box-shadow: none; }
  .save { padding: 8px; }
  .save button { width: 100%; }
  .list { flex: 1; overflow: auto; min-height: 0; }
  .item { padding: 7px 12px; border-bottom: 1px solid var(--border); cursor: pointer; }
  .item:hover, .item:focus-visible { background: var(--hover); }
  .item.fail { box-shadow: inset 3px 0 var(--danger); }
  pre { margin: 0; font: 11.5px/1.45 var(--mono); white-space: pre-wrap; word-break: break-word; max-height: 4.4em; overflow: hidden; }
  .name { font-weight: 600; margin-bottom: 2px; display: flex; align-items: center; gap: 6px; }
  .drv { font: 600 9.5px var(--mono); text-transform: uppercase; color: var(--accent); background: var(--accent-soft); border-radius: 3px; padding: 0 5px; }
  .meta { display: flex; align-items: center; gap: 10px; margin-top: 3px; color: var(--muted); font-size: 11.5px; min-height: 22px; }
  .bad { color: var(--danger); font-weight: 600; }
  .x { color: var(--accent); font-weight: 600; }
  .acts { margin-left: auto; display: flex; opacity: 0; }
  .item:hover .acts, .item:focus-within .acts, .acts.always { opacity: 1; }
  .acts button { padding: 3px; }
  .errline { color: var(--danger); font-size: 11.5px; margin-top: 2px; word-break: break-word; max-height: 3em; overflow: hidden; }
  .none { color: var(--muted); padding: 14px 12px; margin: 0; }
  .foot { padding: 6px 10px; border-top: 1px solid var(--border); }
</style>
