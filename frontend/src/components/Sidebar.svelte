<script lang="ts">
  import Icon from './Icon.svelte'
  import SchemaTree from './SchemaTree.svelte'
  import HistoryPanel from './HistoryPanel.svelte'
  import Menu from './Menu.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { ui } from '../lib/ui.svelte'
  import type { Profile } from '../lib/api'
  import type { Panel } from './Rail.svelte'

  let { panel, onnew, onedit, onconnect }: { panel: Panel; onnew: () => void; onedit: (p: Profile) => void; onconnect: () => void } = $props()
  let filter = $state('')
  let connFilter = $state('')
  let reloads = $state(0)
  let count = $state(0)
  const canReload = $derived(!!ws.active && ws.hasOpenDb(ws.active))
  let expanded = $state(false)
  let tree = $state<{ collapseAll: () => void; expandAll: (d: string) => Promise<void>; defaultTarget: (d: string) => Promise<{ database: string; schema: string } | null> }>()
  let addMenu = $state(false)

  async function newTable() {
    const t = await tree?.defaultTarget(ws.tab?.database ?? '')
    if (t && ws.active) ui.openSchema({ profile: ws.active.id, database: t.database, schema: t.schema, table: '' }, 'create_table')
  }

  const shown = $derived(ws.profiles.filter((p) => p.name.toLowerCase().includes(connFilter.trim().toLowerCase())))
  const base = (path: string) => path.split(/[\\/]/).pop() ?? path
  const detail = (p: Profile) => (p.driver === 'sqlite' ? base(p.filePath) : p.host + (p.database ? ` / ${p.database}` : ''))

  function open(p: Profile) {
    ws.connect(p)
    onconnect()
  }

  async function remove(p: Profile) {
    const ok = await ui.confirm({
      title: `Eliminar "${p.name}"`,
      message: 'Se borrarán la conexión, su contraseña guardada y sus pestañas. Esto no afecta a la base de datos.',
      confirmLabel: 'Eliminar', danger: true,
    })
    if (ok) await ws.removeProfile(p)
  }
</script>

{#snippet row(p: Profile)}
        <div class="conn" class:sel={ws.active?.id === p.id} role="option" tabindex="0" aria-selected={ws.active?.id === p.id}
             onclick={() => open(p)} ondblclick={() => onedit(p)}
             onkeydown={(e) => e.key === 'Enter' && open(p)}>
          <span class="bar" style:background={p.color || undefined}></span>
          <span class="txt">
            <span class="name" title={p.name}>{p.name}</span>
            <span class="detail" title={detail(p)}>{detail(p)}</span>
          </span>
          <span class="acts">
            {#if ws.active?.id === p.id}
              <button class="ghost icon" title="Desconectar" aria-label="Desconectar {p.name}" onclick={(e) => { e.stopPropagation(); ws.disconnect() }}><Icon name="power" size={13} /></button>
            {/if}
            <button class="ghost icon" title="Editar" aria-label="Editar {p.name}" onclick={(e) => { e.stopPropagation(); onedit(p) }}><Icon name="pencil" size={13} /></button>
            <button class="ghost icon danger" title="Eliminar" aria-label="Eliminar {p.name}" onclick={(e) => { e.stopPropagation(); remove(p) }}><Icon name="trash" size={13} /></button>
          </span>
          <span class="badge">{p.driver}</span>
        </div>
{/snippet}

<aside>
  {#if panel === 'connections'}
    <div class="top"><button class="newbtn" onclick={onnew}><Icon name="plus" size={14} /> Nueva conexión</button></div>
    <label class="search">
      <input placeholder="Filtrar…" bind:value={connFilter} aria-label="Filtrar conexiones" />
      {#if connFilter}<button class="ghost icon" aria-label="Limpiar filtro" onclick={() => (connFilter = '')}><Icon name="x" size={12} /></button>{/if}
    </label>
    <div class="label">Guardadas <span class="count">{ws.profiles.length}</span></div>
    <div class="list" role="listbox" aria-label="Conexiones">
      {#each shown as p (p.id)}
        {@render row(p)}
      {:else}
        <p class="hint">{ws.profiles.length ? 'Nada coincide con el filtro.' : 'Aún no hay conexiones.'}</p>
      {/each}
    </div>
    {#if ws.recents.length}
      <div class="recent">
        <div class="label">Recientes <span class="count">{ws.recents.length}</span></div>
        {#each ws.recents as p (p.id)}
          {@render row(p)}
        {/each}
      </div>
    {/if}
  {:else if ws.active}
    {#if panel === 'entities'}
      <div class="top head">
        <Icon name="server" size={14} />
        <span class="cname" title={ws.active.name}>{ws.active.name}</span>
      </div>
      <label class="search">
        <input placeholder="Filtrar tablas…" bind:value={filter} aria-label="Filtrar tablas" />
        {#if filter}<button class="ghost icon" aria-label="Limpiar filtro" onclick={() => (filter = '')}><Icon name="x" size={12} /></button>{/if}
      </label>
      <div class="label ent">
        <span>Entidades <span class="count">{count}</span></span>
        <span class="tools">
          <button class="ghost icon" title={expanded ? 'Contraer todo' : 'Expandir todo'} aria-label={expanded ? 'Contraer todo' : 'Expandir todo'} onclick={() => (expanded ? tree?.collapseAll() : void tree?.expandAll(ws.tab?.database ?? ''))}><Icon name={expanded ? 'collapse' : 'chevrons'} size={14} /></button>
          <button class="ghost icon" title={canReload ? 'Recargar esquema' : 'Recargar esquema (abre antes una base de datos)'} aria-label="Recargar esquema" disabled={!canReload} onclick={() => { reloads++; ws.invalidateIndex() }}><Icon name="refresh" size={14} /></button>
          <span class="addwrap">
            <button class="ghost icon" title="Nuevo…" aria-label="Nuevo…" aria-haspopup="menu" aria-expanded={addMenu} onpointerdown={(e) => e.stopPropagation()} onclick={() => (addMenu = !addMenu)}>
              <Icon name="plus" size={14} /><Icon name="down" size={10} />
            </button>
            {#if addMenu}
              <Menu right onclose={() => (addMenu = false)} items={[
                { label: 'Nueva tabla', icon: 'table', run: () => void newTable() },
                { label: 'Nueva consulta', icon: 'sql', run: () => { ws.newTab() } },
              ]} />
            {/if}
          </span>
        </span>
      </div>
      <div class="scroll">
        {#key ws.active.id + ':' + reloads + ':' + ws.schemaVersion}
          <SchemaTree bind:this={tree} bind:count bind:expanded profile={ws.active} {filter} onopen={(d) => ws.setDatabase(d)} ontable={(d, s, t) => ws.openTable(d, s, t)} onstructure={(d, s, t) => ws.openTable(d, s, t, 'structure')}
            oncreate={(d, s) => ui.openSchema({ profile: ws.active!.id, database: d, schema: s, table: '' }, 'create_table')} />
        {/key}
      </div>
    {:else}
      <div class="label first">{panel === 'saved' ? 'Consultas guardadas' : 'Historial'}</div>
      <HistoryPanel mode={panel === 'saved' ? 'saved' : 'history'} />
    {/if}
  {/if}
</aside>

<style>
  aside { display: flex; flex-direction: column; min-height: 0; height: 100%; background: var(--surface); }
  .top { padding: 10px 10px 8px; }
  .newbtn { width: 100%; justify-content: center; background: var(--surface-2); font-weight: 500; padding: 6px; }
  .head { display: flex; align-items: center; gap: 8px; color: var(--muted); padding: 8px 8px 6px 12px; }
  .cname { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); font-weight: 500; }
  .label { padding: 10px 12px 4px; font-size: 11px; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; color: var(--text); }
  .label.ent { display: flex; align-items: center; justify-content: space-between; padding-right: 8px; }
  .tools { display: flex; align-items: center; gap: 2px; text-transform: none; }
  .tools button { padding: 3px; color: var(--muted); display: inline-flex; align-items: center; gap: 1px; }
  .addwrap { position: relative; display: inline-flex; }
  .label.first { padding-top: 12px; }
  .count { display: inline-block; margin-left: 4px; padding: 0 6px; border-radius: 8px; background: var(--accent-soft); color: var(--accent); font-weight: 600; }
  .search { display: flex; align-items: center; margin: 0 10px 2px; padding: 0 4px 0 0; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); }
  .search:focus-within { border-color: var(--accent); }
  .search input { border: none; background: transparent; box-shadow: none; padding: 6px 10px; }
  .search input:focus { box-shadow: none; }
  .recent { flex: none; max-height: 40%; overflow: auto; border-top: 1px solid var(--border); padding-bottom: 6px; }
  .list, .scroll { flex: 1; overflow: auto; min-height: 0; padding-bottom: 6px; }
  .conn { display: flex; align-items: center; gap: 8px; padding: 6px 10px 6px 0; cursor: pointer; }
  .conn:hover { background: var(--hover); }
  .conn.sel { background: var(--accent-soft); }
  .bar { align-self: stretch; width: 3px; margin: 2px 7px 2px 10px; border-radius: 2px; background: var(--border); }
  .conn.sel .bar:not([style]) { background: var(--accent); }
  .txt { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .name { font-size: 14px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .detail { font-size: 11.5px; color: var(--muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { font: 600 10.5px var(--mono); color: var(--accent); background: var(--accent-soft); border-radius: 10px; padding: 1px 8px; }
  .acts { display: none; }
  .conn:hover .acts, .conn:focus-within .acts { display: flex; }
  .conn:hover .badge, .conn:focus-within .badge { display: none; }
  .acts button { padding: 3px; }
  .hint { margin: 6px 12px 10px; color: var(--muted); font-size: 12px; }
</style>
