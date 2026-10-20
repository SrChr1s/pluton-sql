<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
  let { ontoggle }: { ontoggle: () => void } = $props()
</script>

<div class="bar">
  <button class="ghost icon side" title="Mostrar/ocultar panel lateral (Ctrl+B)" aria-label="Panel lateral" onclick={ontoggle}><Icon name="sidebar" size={16} /></button>
<div class="tabs" role="tablist" aria-label="Pestañas">
  {#each ws.tabs as t, i (t.id)}
    <div class="tab" class:sel={ws.tab?.id === t.id} role="tab" tabindex="0" aria-selected={ws.tab?.id === t.id}
         onclick={() => ws.select(t.id)} onauxclick={(e) => e.button === 1 && ws.close(t.id)}
         onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && ws.select(t.id)} title="{t.kind === 'query' && t.filePath ? t.filePath : t.title}{i < 9 ? ` (Alt+${i + 1})` : ''}">
      {#if t.kind === 'query' && t.filePath}
        <span class="kind" title={t.filePath}><Icon name="file" size={13} /></span>
      {:else if t.kind === 'query'}
        <span class="kind"><Icon name="sql" size={13} /></span>
        <span class="dot" class:running={t.running} class:err={!!t.error}></span>
      {:else}
        <span class="kind"><Icon name="table" size={13} /></span>
      {/if}
      <span class="title">{t.title}</span>
      {#if ws.databases.length > 1}<span class="tdb" title="Base de datos de esta pestaña">{ws.dbOf(t)}</span>{/if}
      {#if t.kind === 'query' && ws.isDirty(t)}<span class="pend" title="Cambios sin guardar en {t.filePath}"></span>{/if}
      {#if t.kind === 'table' && t.state.pending > 0}<span class="pend" title="{t.state.pending} cambio(s) sin aplicar"></span>{/if}
      <button class="ghost icon close" aria-label="Cerrar pestaña" onclick={(e) => { e.stopPropagation(); ws.close(t.id) }}>
        <Icon name="x" size={13} />
      </button>
    </div>
  {/each}
  <button class="ghost icon add" title="Nueva consulta (Ctrl+T)" aria-label="Nueva consulta" onclick={() => ws.newTab()}>
    <Icon name="plus" size={15} />
  </button>
</div>
</div>

<style>
  .bar { display: flex; align-items: stretch; background: var(--bg); border-bottom: 1px solid var(--border); min-height: 34px; }
  .tabs { display: flex; align-items: stretch; flex: 1; min-width: 0; padding: 0; overflow-x: auto; scrollbar-width: none; }
  .tdb { flex: none; max-width: 70px; overflow: hidden; text-overflow: ellipsis; padding: 0 5px; font-size: 10.5px; line-height: 16px; border-radius: 8px; color: var(--muted); background: var(--hover); }
  .tab.sel .tdb { color: var(--accent); background: color-mix(in srgb, var(--accent) 14%, transparent); }
  .tab { display: flex; align-items: center; gap: 7px; min-width: 110px; max-width: 220px; padding: 0 4px 0 12px; cursor: pointer; border-right: 1px solid var(--border); border-top: 2px solid transparent; color: var(--muted); user-select: none; }
  .tab:hover { background: var(--hover); color: var(--text); }
  .tab.sel { background: var(--surface); border-top-color: var(--accent); color: var(--text); margin-bottom: -1px; }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: transparent; flex: none; }
  .dot.running { background: var(--accent); animation: pulse 1s ease-in-out infinite; }
  .dot.err { background: var(--danger); }
  .close { opacity: 0; padding: 2px; }
  .tab:hover .close, .tab.sel .close, .close:focus-visible { opacity: 1; }
  .add { align-self: center; margin: 0 4px; color: var(--muted); }
  .side { align-self: center; flex: none; margin: 0 4px; color: var(--muted); }
  .kind { display: inline-flex; color: var(--muted); }
  .tab.sel .kind { color: var(--accent); }
  .pend { width: 7px; height: 7px; border-radius: 50%; background: var(--accent); flex: none; }
  @keyframes pulse { 50% { opacity: .3; } }
  @media (prefers-reduced-motion: reduce) { .dot.running { animation: none; } }
</style>
