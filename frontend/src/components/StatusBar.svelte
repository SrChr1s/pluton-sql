<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { ui } from '../lib/ui.svelte'

  const q = $derived(ws.queryTab)
  const r = $derived(q?.result)
  const dt = $derived(ws.dataTab?.state)

  function exportQuery() {
    if (!q || !ws.active) return
    ui.exporting = { kind: 'query', profile: ws.active.id, database: q.database, sql: q.lastSql || q.sql, name: q.title.replace(/\.sql$/i, '') }
  }
</script>

<footer>
  <div class="left" style:box-shadow={ws.active?.color ? `inset 3px 0 ${ws.active.color}` : undefined}>
    {#if ws.active}
      <Icon name="database" size={13} />
      <span class="name" title={ws.active.name}>{ws.tab?.database || ws.active.name}</span>
      <span class="grow"></span>
      <span class="drv">{ws.active.driver}</span>
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
  .left { background: var(--surface); color: var(--text); font-weight: 500; }
  .right { background: var(--bg); }
  .name { overflow: hidden; text-overflow: ellipsis; }
  .grow { flex: 1; }
  .drv { font-weight: 400; color: var(--muted); }
  .right button { padding: 3px 8px; font-size: 12px; }
</style>
