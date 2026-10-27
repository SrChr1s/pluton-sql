<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { ui } from '../lib/ui.svelte'
  import ConnMenu from './ConnMenu.svelte'

  const q = $derived(ws.queryTab)
  const r = $derived(q?.result)
  const dt = $derived(ws.dataTab?.state)

  // Base de datos y tabla de la pestaña: solo se usan en el tooltip y en el menú.
  const dbName = $derived(ws.currentDb)
  const tableName = $derived(ws.dataTab?.state.loc.table ?? '')
  // Servidor de la conexión (en SQLite, el archivo).
  const hostName = $derived(!ws.active ? '' : ws.active.driver === 'sqlite' ? (ws.active.filePath.split(/[\\/]/).pop() ?? ws.active.filePath) : ws.active.host || ws.active.name)

  let menuOpen = $state(false)

  function exportQuery() {
    if (!q || !ws.active) return
    ui.exporting = { kind: 'query', profile: ws.active.id, database: q.database, sql: q.lastSql || q.sql, name: q.title.replace(/\.sql$/i, '') }
  }
</script>

<footer>
  <div class="left" style:box-shadow={ws.active?.color ? `inset 3px 0 ${ws.active.color}` : undefined}>
    {#if ws.active}
      <button class="conn" aria-haspopup="menu" aria-expanded={menuOpen} title="Opciones de la conexión" onpointerdown={(e) => e.stopPropagation()} onclick={() => (menuOpen = !menuOpen)} oncontextmenu={(e) => { e.preventDefault(); menuOpen = true }}>
        <Icon name="link" size={14} />
        <span class="name" title="{ws.active.name}{dbName ? ` — ${dbName}` : ''}{tableName ? ` › ${tableName}` : ''}">{hostName}</span>
        <span class="grow"></span>
        <span class="drv">{ws.active.driver}</span>
      </button>
      {#if menuOpen}<ConnMenu up onclose={() => (menuOpen = false)} />{/if}
    {:else}
      <span class="name">Sin conexión</span>
    {/if}
  </div>
  <div class="right">
    {#if dt}
      <span>{dt.page ? `${dt.page.rows.length} fila(s)${dt.total !== null ? ` de ${dt.total}` : dt.page.hasMore ? '+' : ''}` : ''}{dt.pending ? ` · ${dt.pending} cambio(s) pendiente(s)` : ''}</span>
    {:else if q?.running}
      <span>Ejecutando…</span>
    {:else if r}
      <span>{r.isQuery ? `${r.rows.length} fila(s)${r.truncated ? ' (truncado)' : ''}` : `${r.rowsAffected} afectada(s)`} · {r.durationMs} ms</span>
    {:else if q}
      <span>Sin datos</span>
    {/if}
    <span class="grow"></span>
    <span class="ver" title="Versión instalada de Plutón SQL">v{__APP_VERSION__}</span>
    {#if q}
      <button class="ghost" title="Exportar el resultado a CSV, JSON o INSERT (se vuelve a ejecutar y se exportan todas las filas)" disabled={!r?.isQuery} onclick={exportQuery}>
        <Icon name="download" size={13} /> Exportar
      </button>
    {/if}
  </div>
</footer>

<style>
  footer { display: contents; }
  .left, .right { display: flex; align-items: center; gap: 8px; height: 100%; padding: 0 12px; font-size: 12px; color: var(--muted); min-width: 0; white-space: nowrap; }
  .left { background: var(--surface); color: var(--text); font-weight: 500; position: relative; padding: 0; }
  .conn { display: flex; align-items: center; gap: 8px; width: 100%; height: 100%; padding: 0 12px; border: none; border-radius: 0; background: transparent; color: inherit; font: inherit; }
  .conn:hover { background: var(--hover); }
  .left > :global(.menu) { left: 8px; }
  .right { background: var(--surface); border-left: 1px solid var(--border); }
  .name { overflow: hidden; text-overflow: ellipsis; }
  .grow { flex: 1; }
  .drv { font-weight: 500; color: var(--accent); }
  .ver { color: var(--accent); font-variant-numeric: tabular-nums; }
  .right button { padding: 3px 8px; font-size: 12px; }
</style>
