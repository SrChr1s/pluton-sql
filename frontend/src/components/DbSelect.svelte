<script lang="ts">
  import Icon from './Icon.svelte'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { ws } from '../lib/workspace.svelte'

  let open = $state(false)
  const db = $derived(ws.treeDb)
  const sqlite = $derived(ws.active?.driver === 'sqlite')
  const connected = $derived(!!ws.active && !!db && (sqlite || ws.isDbOpen(ws.active.id, db)))
  // Verde: base conectada. Color de acento: la seleccionada (la que muestra el árbol).
  const items = $derived<MenuItem[]>([
    ...ws.databases.map((d) => ({
      label: d,
      checked: d === db,
      color: d === db ? 'var(--accent)' : ws.active && ws.isDbOpen(ws.active.id, d) ? 'var(--ok)' : undefined,
      run: () => ws.setDatabase(d),
    })),
    ...(connected && !sqlite ? [{ sep: true, label: '' }, { label: `Desconectar de ${db}`, icon: 'power', run: () => void ws.disconnectDatabase(db) }] : []),
  ])
</script>

<div class="dbsel">
  <button class="btn" class:empty={!db} aria-haspopup="menu" aria-expanded={open} disabled={ws.databases.length < 2 && !!db}
          title={db ? `Base de datos seleccionada: ${db}${ws.databases.length > 1 ? ' — clic para cambiar' : ''}` : 'Elige la base de datos que quieres explorar'}
          onpointerdown={(e) => e.stopPropagation()} onclick={() => (open = !open)}>
    <Icon name="database" size={14} />
    <span class="name">{db || 'Selecciona una base de datos'}</span>
    {#if db}<span class="state" class:on={connected} title={connected ? 'Conectada' : 'Sin conectar'}></span>{/if}
    {#if ws.databases.length > 1}<Icon name="down" size={12} />{/if}
  </button>
  {#if open}<Menu {items} maxHeight={Math.max(160, Math.min(360, innerHeight - 160))} onclose={() => (open = false)} />{/if}
</div>

<style>
  .dbsel { position: relative; padding: 0 10px 6px; }
  .btn { display: flex; align-items: center; gap: 7px; width: 100%; padding: 6px 10px; font-size: 12.5px; color: var(--accent); background: color-mix(in srgb, var(--accent) 12%, transparent); border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent); border-radius: var(--radius); }
  .btn:hover:not(:disabled) { background: color-mix(in srgb, var(--accent) 20%, transparent); }
  .btn:disabled { opacity: 1; cursor: default; }
  .btn.empty { color: var(--muted); background: var(--bg); border-style: dashed; border-color: var(--border); }
  .name { flex: 1; text-align: left; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 600; }
  .btn.empty .name { font-weight: 400; }
  .state { width: 7px; height: 7px; border-radius: 50%; flex: none; border: 1.5px solid var(--muted); }
  .state.on { background: var(--ok); border-color: var(--ok); }
  .dbsel > :global(.menu) { left: 10px; right: 10px; min-width: 0; }
</style>
