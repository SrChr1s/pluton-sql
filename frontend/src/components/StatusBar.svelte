<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { theme } from '../lib/theme.svelte'

  const r = $derived(ws.tab?.result)
  const label = $derived({ system: 'Tema: sistema', light: 'Tema: claro', dark: 'Tema: oscuro' }[theme.mode])
  const icon = $derived({ system: 'monitor', light: 'sun', dark: 'moon' }[theme.mode])
</script>

<footer>
  <span class="grp">
    {#if ws.active}
      <Icon name="server" size={13} /> {ws.active.name}
      {#if ws.tab?.database}<span class="sep">›</span><Icon name="database" size={13} /> {ws.tab.database}{/if}
    {:else}
      Sin conexión
    {/if}
  </span>
  <span class="grow"></span>
  {#if ws.tab?.running}
    <span>Ejecutando…</span>
  {:else if r}
    <span>{r.isQuery ? `${r.rows.length} fila(s)${r.truncated ? ' (truncado)' : ''}` : `${r.rowsAffected} afectada(s)`} · {r.durationMs} ms</span>
  {/if}
  <button class="ghost icon" title="{label} (clic para cambiar)" aria-label={label} onclick={() => theme.cycle()}>
    <Icon name={icon} size={14} />
  </button>
</footer>

<style>
  footer { display: flex; align-items: center; gap: 14px; padding: 2px 10px; height: 26px; background: var(--surface); border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; }
  .grp { display: flex; align-items: center; gap: 5px; min-width: 0; white-space: nowrap; overflow: hidden; }
  .sep { opacity: .5; }
  .grow { flex: 1; }
</style>
