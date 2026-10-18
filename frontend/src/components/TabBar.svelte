<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
</script>

<div class="tabs" role="tablist" aria-label="Consultas">
  {#each ws.tabs as t, i (t.id)}
    <div class="tab" class:sel={ws.tab?.id === t.id} role="tab" tabindex="0" aria-selected={ws.tab?.id === t.id}
         onclick={() => ws.select(t.id)} onauxclick={(e) => e.button === 1 && ws.close(t.id)}
         onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && ws.select(t.id)} title="{t.title}{i < 9 ? ` (Alt+${i + 1})` : ''}">
      <span class="dot" class:running={t.running} class:err={!!t.error}></span>
      <span class="title">{t.title}</span>
      <button class="ghost icon close" aria-label="Cerrar pestaña" onclick={(e) => { e.stopPropagation(); ws.close(t.id) }}>
        <Icon name="x" size={13} />
      </button>
    </div>
  {/each}
  <button class="ghost icon add" title="Nueva consulta (Ctrl+T)" aria-label="Nueva consulta" onclick={() => ws.newTab()}>
    <Icon name="plus" size={15} />
  </button>
</div>

<style>
  .tabs { display: flex; align-items: flex-end; gap: 1px; padding: 6px 8px 0; background: var(--bg); border-bottom: 1px solid var(--border); overflow-x: auto; scrollbar-width: none; }
  .tab { display: flex; align-items: center; gap: 7px; max-width: 220px; padding: 5px 4px 5px 10px; cursor: pointer; border: 1px solid transparent; border-bottom: none; border-radius: var(--radius) var(--radius) 0 0; color: var(--muted); user-select: none; }
  .tab:hover { background: var(--hover); color: var(--text); }
  .tab.sel { background: var(--surface); border-color: var(--border); color: var(--text); margin-bottom: -1px; padding-bottom: 6px; }
  .title { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: transparent; flex: none; }
  .dot.running { background: var(--accent); animation: pulse 1s ease-in-out infinite; }
  .dot.err { background: var(--danger); }
  .close { opacity: 0; padding: 2px; }
  .tab:hover .close, .tab.sel .close, .close:focus-visible { opacity: 1; }
  .add { margin: 0 0 4px 4px; color: var(--muted); }
  @keyframes pulse { 50% { opacity: .3; } }
  @media (prefers-reduced-motion: reduce) { .dot.running { animation: none; } }
</style>
