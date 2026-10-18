<script lang="ts">
  import Icon from './Icon.svelte'
  import SchemaTree from './SchemaTree.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { ui } from '../lib/ui.svelte'
  import type { Profile } from '../lib/api'

  let { onnew, onedit }: { onnew: () => void; onedit: (p: Profile) => void } = $props()
  let filter = $state('')
  let reloads = $state(0)

  async function remove(p: Profile) {
    const ok = await ui.confirm({
      title: `Eliminar "${p.name}"`,
      message: 'Se borrarán la conexión, su contraseña guardada y sus pestañas. Esto no afecta a la base de datos.',
      confirmLabel: 'Eliminar', danger: true,
    })
    if (ok) await ws.removeProfile(p)
  }
</script>

<aside>
  <section class="conns">
    <header>
      <span class="h">Conexiones</span>
      <button class="ghost icon" title="Nueva conexión" aria-label="Nueva conexión" onclick={onnew}><Icon name="plus" size={15} /></button>
    </header>
    <div class="list" role="listbox" aria-label="Conexiones">
      {#each ws.profiles as p (p.id)}
        <div class="conn" class:sel={ws.active?.id === p.id} role="option" tabindex="0" aria-selected={ws.active?.id === p.id}
             onclick={() => ws.connect(p)} ondblclick={() => onedit(p)}
             onkeydown={(e) => e.key === 'Enter' && ws.connect(p)}>
          <span class="badge">{p.driver}</span>
          <span class="name" title="{p.name}{p.host ? ' — ' + p.host : ''}">{p.name}</span>
          <span class="acts">
            <button class="ghost icon" title="Editar" aria-label="Editar {p.name}" onclick={(e) => { e.stopPropagation(); onedit(p) }}><Icon name="pencil" size={13} /></button>
            <button class="ghost icon danger" title="Eliminar" aria-label="Eliminar {p.name}" onclick={(e) => { e.stopPropagation(); remove(p) }}><Icon name="trash" size={13} /></button>
          </span>
        </div>
      {:else}
        <p class="hint">Aún no hay conexiones.</p>
      {/each}
    </div>
  </section>

  {#if ws.active}
    <section class="schema">
      <header>
        <span class="h">Esquema</span>
        <button class="ghost icon" title="Recargar esquema" aria-label="Recargar esquema" onclick={() => reloads++}><Icon name="refresh" size={14} /></button>
      </header>
      <label class="search">
        <Icon name="search" size={14} />
        <input placeholder="Filtrar tablas…" bind:value={filter} aria-label="Filtrar tablas" />
        {#if filter}<button class="ghost icon" aria-label="Limpiar filtro" onclick={() => (filter = '')}><Icon name="x" size={12} /></button>{/if}
      </label>
      <div class="scroll">
        {#key ws.active.id + ':' + reloads + ':' + ws.schemaVersion}
          <SchemaTree profile={ws.active} {filter} onopen={(d) => ws.setDatabase(d)} ontable={(d, s, t) => ws.openTable(d, s, t)} onstructure={(d, s, t) => ws.openTable(d, s, t, 'structure')}
            oncreate={(d, s) => ui.openSchema({ profile: ws.active!.id, database: d, schema: s, table: '' }, 'create_table')} />
        {/key}
      </div>
    </section>
  {/if}
</aside>

<style>
  aside { display: flex; flex-direction: column; min-height: 0; height: 100%; background: var(--surface); border-right: 1px solid var(--border); }
  header { display: flex; align-items: center; justify-content: space-between; padding: 6px 8px 4px 12px; }
  .h { font-size: 11px; font-weight: 600; letter-spacing: .06em; text-transform: uppercase; color: var(--muted); }
  .conns { flex: none; max-height: 38%; display: flex; flex-direction: column; border-bottom: 1px solid var(--border); }
  .list { overflow: auto; padding: 0 0 6px; }
  .conn { display: flex; align-items: center; gap: 8px; padding: 5px 8px 5px 12px; cursor: pointer; border-left: 2px solid transparent; }
  .conn:hover { background: var(--hover); }
  .conn.sel { background: var(--accent-soft); border-left-color: var(--accent); }
  .name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { font: 600 9.5px var(--mono); text-transform: uppercase; color: var(--accent); background: var(--accent-soft); border-radius: 3px; padding: 1px 5px; letter-spacing: .03em; }
  .acts { display: flex; opacity: 0; }
  .conn:hover .acts, .conn:focus-within .acts { opacity: 1; }
  .acts button { padding: 3px; }
  .hint { margin: 6px 12px 10px; color: var(--muted); font-size: 12px; }
  .schema { flex: 1; min-height: 0; display: flex; flex-direction: column; }
  .search { display: flex; align-items: center; gap: 6px; margin: 0 8px 6px; padding: 0 4px 0 8px; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); color: var(--muted); }
  .search:focus-within { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-soft); }
  .search input { border: none; background: transparent; box-shadow: none; padding: 5px 0; }
  .search input:focus { box-shadow: none; }
  .scroll { flex: 1; overflow: auto; min-height: 0; }
</style>
