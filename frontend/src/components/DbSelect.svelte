<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'

  let open = $state(false)
  let query = $state('')
  let hi = $state(0)
  let wrap: HTMLDivElement
  let listEl: HTMLDivElement | undefined = $state()

  const db = $derived(ws.treeDb)
  const sqlite = $derived(ws.active?.driver === 'sqlite')
  const connected = $derived(!!ws.active && !!db && (sqlite || ws.isDbOpen(ws.active.id, db)))
  const filtered = $derived(ws.databases.filter((d) => d.toLowerCase().includes(query.trim().toLowerCase())))
  // Verde: base conectada. Color de acento: la seleccionada (la que muestra el árbol).
  const dotOf = (d: string) => (d === db ? 'sel' : ws.active && ws.isDbOpen(ws.active.id, d) ? 'on' : '')

  function show() {
    query = ''
    hi = Math.max(0, ws.databases.indexOf(db))
    open = true
  }
  const close = () => { open = false }

  function pick(d: string) {
    close()
    ws.setDatabase(d)
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); hi = Math.min(filtered.length - 1, hi + 1) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); hi = Math.max(0, hi - 1) }
    else if (e.key === 'Enter') { e.preventDefault(); if (filtered[hi]) pick(filtered[hi]) }
    else if (e.key === 'Escape') { e.stopPropagation(); close() }
  }

  $effect(() => { query; hi = 0 }) // al escribir, se resalta la primera coincidencia
  $effect(() => { if (open && listEl) listEl.querySelector('.hi')?.scrollIntoView({ block: 'nearest' }) })

  function focus(node: HTMLInputElement) { node.focus() }
</script>

<svelte:window onpointerdown={(e) => { if (open && !wrap?.contains(e.target as Node)) close() }} onblur={close} />

<div class="dbsel" bind:this={wrap}>
  <button class="btn" class:empty={!db} aria-haspopup="listbox" aria-expanded={open} disabled={ws.databases.length < 2 && !!db}
          title={db ? `Base de datos seleccionada: ${db}${ws.databases.length > 1 ? ' — clic para cambiar' : ''}` : 'Elige la base de datos que quieres explorar'}
          onclick={() => (open ? close() : show())}>
    <Icon name="database" size={14} />
    <span class="name">{db || 'Selecciona una base de datos'}</span>
    {#if db}<span class="state" class:on={connected} title={connected ? 'Conectada' : 'Sin conectar'}></span>{/if}
    {#if ws.databases.length > 1}<Icon name="down" size={12} />{/if}
  </button>
  {#if open}
    <div class="pop">
      <input use:focus bind:value={query} onkeydown={onKey} placeholder="Buscar base de datos…" role="combobox" aria-expanded="true" aria-controls="db-list" aria-label="Buscar base de datos" autocomplete="off" spellcheck="false" />
      <div class="list" id="db-list" role="listbox" bind:this={listEl}>
        {#each filtered as d, i (d)}
          <button class="opt" class:hi={i === hi} role="option" aria-selected={d === db} onmouseenter={() => (hi = i)} onclick={() => pick(d)}>
            <span class="dot {dotOf(d)}"></span>
            <span class="lbl">{d}</span>
            {#if d === db}<Icon name="check" size={13} />{/if}
          </button>
        {:else}
          <p class="none">{ws.databases.length ? 'Ninguna base coincide.' : 'Cargando bases de datos…'}</p>
        {/each}
      </div>
      {#if connected && !sqlite}
        <button class="opt off" onclick={() => { close(); void ws.disconnectDatabase(db) }}><span class="dot"></span><Icon name="power" size={14} /><span class="lbl">Desconectar de {db}</span></button>
      {/if}
    </div>
  {/if}
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

  .pop { position: absolute; z-index: 30; left: 10px; right: 10px; top: calc(100% - 2px); display: flex; flex-direction: column; padding: 6px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 8px; box-shadow: var(--shadow); }
  .pop input { margin-bottom: 4px; }
  .list { max-height: min(320px, calc(100vh - 260px)); overflow-y: auto; overscroll-behavior: contain; }
  .opt { display: flex; align-items: center; gap: 8px; width: 100%; padding: 6px 8px; border: none; background: transparent; border-radius: 6px; color: var(--text); text-align: left; font-size: 12.5px; }
  .opt.hi { background: var(--hover); }
  .opt.off { margin-top: 4px; border-top: 1px solid var(--border); border-radius: 0 0 6px 6px; color: var(--muted); }
  .lbl { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 8px; height: 8px; border-radius: 50%; flex: none; }
  .dot.on { background: var(--ok); }
  .dot.sel { background: var(--accent); }
  .none { margin: 6px 8px; color: var(--muted); font-size: 12px; }
</style>
