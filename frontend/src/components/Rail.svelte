<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { theme } from '../lib/theme.svelte'

  export type Panel = 'connections' | 'entities' | 'saved' | 'history'
  let { panel, onselect }: { panel: Panel; onselect: (p: Panel) => void } = $props()

  const items: { id: Panel; icon: string; label: string; needsConn: boolean }[] = [
    { id: 'connections', icon: 'server', label: 'Conexiones', needsConn: false },
    { id: 'entities', icon: 'database', label: 'Entidades', needsConn: true },
    { id: 'saved', icon: 'bookmark', label: 'Consultas guardadas', needsConn: true },
    { id: 'history', icon: 'clock', label: 'Historial (Ctrl+H)', needsConn: true },
  ]
  const themeLabel = $derived(theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro')
</script>

<nav class="rail" aria-label="Paneles">
  {#each items as it (it.id)}
    {#if !it.needsConn || ws.active}
      <button class="ghost icon" class:on={panel === it.id} title={it.label} aria-label={it.label} aria-pressed={panel === it.id} onclick={() => onselect(it.id)}>
        <Icon name={it.icon} size={19} />
      </button>
    {/if}
  {/each}
  <span class="grow"></span>
  <button class="ghost icon" title={themeLabel} aria-label={themeLabel} onclick={() => theme.toggle()}>
    <Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={18} />
  </button>
</nav>

<style>
  .rail { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 10px 0; background: var(--bg); }
  .rail button { width: 36px; height: 36px; padding: 0; display: grid; place-items: center; color: var(--muted); border-radius: 8px; }
  .rail button:hover { color: var(--text); }
  .rail button.on { background: var(--surface-2); color: var(--text); }
  .grow { flex: 1; }
</style>
