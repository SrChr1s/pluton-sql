<script lang="ts">
  import { api, errMsg, type Profile, type QueryResult } from './lib/api'
  import ConnectionForm from './components/ConnectionForm.svelte'
  import SchemaTree from './components/SchemaTree.svelte'
  import QueryEditor from './components/QueryEditor.svelte'
  import ResultGrid from './components/ResultGrid.svelte'

  let profiles = $state<Profile[]>([])
  let active = $state<Profile | null>(null)
  let database = $state('')
  let form = $state<{ profile: Profile | null } | null>(null)
  let sqlText = $state('SELECT 1;')
  let result = $state<QueryResult | null>(null)
  let error = $state('')
  let running = $state(false)
  let queryId = ''
  let editorPct = $state(40)

  const load = async () => (profiles = await api.listConnections())
  load()

  const connect = (p: Profile) => { active = p; database = ''; result = null; error = '' }

  async function remove(p: Profile) {
    if (!confirm(`¿Eliminar la conexión "${p.name}"?`)) return
    await api.deleteConnection(p.id)
    if (active?.id === p.id) active = null
    load()
  }

  async function run(text: string) {
    if (!active || !text.trim() || running) return
    running = true; error = ''
    queryId = crypto.randomUUID()
    try { result = await api.execute(queryId, active.id, database, text) }
    catch (e) { error = errMsg(e); result = null }
    running = false
  }
  const cancel = () => api.cancel(queryId)

  const quote = (s: string) => (active?.driver === 'mysql' ? '`' + s + '`' : active?.driver === 'mssql' ? `[${s}]` : `"${s}"`)
  function openTable(db: string, schema: string, table: string) {
    database = db
    sqlText = `SELECT * FROM ${schema ? quote(schema) + '.' : ''}${quote(table)}${active?.driver === 'mssql' ? '' : ' LIMIT 100'};`
    if (active?.driver === 'mssql') sqlText = sqlText.replace('SELECT', 'SELECT TOP 100')
    run(sqlText)
  }

  function drag(e: PointerEvent) {
    const main = (e.currentTarget as HTMLElement).parentElement!
    const move = (ev: PointerEvent) => {
      const r = main.getBoundingClientRect()
      editorPct = Math.min(80, Math.max(15, ((ev.clientY - r.top) / r.height) * 100))
    }
    const up = () => { removeEventListener('pointermove', move); removeEventListener('pointerup', up) }
    addEventListener('pointermove', move); addEventListener('pointerup', up)
  }
</script>

<div class="app">
  <aside>
    <header>
      <b>Conexiones</b>
      <button onclick={() => (form = { profile: null })} title="Nueva conexión">＋</button>
    </header>
    <div class="list">
      {#each profiles as p (p.id)}
        <div class="conn" class:sel={active?.id === p.id} onclick={() => connect(p)} ondblclick={() => (form = { profile: p })}
             role="button" tabindex="0" onkeydown={(e) => e.key === 'Enter' && connect(p)}>
          <span class="badge">{p.driver}</span>
          <span class="name">{p.name}</span>
          <button class="x" title="Editar" onclick={(e) => { e.stopPropagation(); form = { profile: p } }}>✎</button>
          <button class="x danger" title="Eliminar" onclick={(e) => { e.stopPropagation(); remove(p) }}>✕</button>
        </div>
      {:else}
        <p class="hint">Sin conexiones. Pulsa ＋ para crear una.</p>
      {/each}
    </div>
    {#if active}
      <header><b>{active.name}</b></header>
      <div class="treebox">
        {#key active.id}
          <SchemaTree profile={active} onopen={(d) => (database = d)} ontable={openTable} />
        {/key}
      </div>
    {/if}
  </aside>

  <main>
    {#if active}
      <div class="toolbar">
        <button class="primary" onclick={() => run(sqlText)} disabled={running}>▶ Ejecutar</button>
        <button onclick={cancel} disabled={!running}>■ Cancelar</button>
        <span class="ctx">{active.name}{database ? ` › ${database}` : ''}</span>
        <span class="hint">Ctrl+Enter ejecuta la selección o todo</span>
      </div>
      <div class="editor" style:height="{editorPct}%">
        <QueryEditor bind:value={sqlText} driver={active.driver} onrun={run} />
      </div>
      <div class="split" onpointerdown={drag} role="separator" aria-orientation="horizontal"></div>
      <div class="results"><ResultGrid {result} {error} {running} /></div>
    {:else}
      <div class="empty">Selecciona o crea una conexión para empezar.</div>
    {/if}
  </main>
</div>

{#if form}
  <ConnectionForm profile={form.profile} oncancel={() => (form = null)}
    onsaved={async (p) => { form = null; await load(); if (active?.id === p.id || !active) connect(p) }} />
{/if}

<style>
  .app { display: grid; grid-template-columns: 280px 1fr; height: 100%; }
  aside { background: var(--panel); border-right: 1px solid var(--border); display: flex; flex-direction: column; min-height: 0; }
  header { display: flex; justify-content: space-between; align-items: center; padding: 8px 10px; border-bottom: 1px solid var(--border); }
  .list { max-height: 35%; overflow: auto; padding: 4px 0; border-bottom: 1px solid var(--border); }
  .conn { display: flex; align-items: center; gap: 6px; padding: 5px 10px; cursor: pointer; }
  .conn:hover, .conn.sel { background: var(--panel2); }
  .conn.sel { box-shadow: inset 2px 0 var(--accent); }
  .name { flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge { font-size: 10px; text-transform: uppercase; color: var(--accent); border: 1px solid var(--border); border-radius: 3px; padding: 0 4px; }
  .x { border: none; background: none; padding: 0 3px; color: var(--muted); visibility: hidden; }
  .conn:hover .x { visibility: visible; }
  .treebox { flex: 1; overflow: auto; padding: 4px 0; }
  main { display: flex; flex-direction: column; min-width: 0; min-height: 0; }
  .toolbar { display: flex; gap: 8px; align-items: center; padding: 6px 10px; border-bottom: 1px solid var(--border); background: var(--panel); }
  .ctx { color: var(--accent); margin-left: 8px; }
  .hint { color: var(--muted); font-size: 12px; margin: 6px 10px; }
  .toolbar .hint { margin: 0 0 0 auto; }
  .editor { min-height: 0; }
  .split { height: 5px; cursor: row-resize; background: var(--border); flex: none; }
  .split:hover { background: var(--accent); }
  .results { flex: 1; min-height: 0; }
  .empty { margin: auto; color: var(--muted); }
</style>
