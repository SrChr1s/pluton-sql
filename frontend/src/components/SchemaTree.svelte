<script lang="ts">
  import { api, errMsg, type Profile, type Table, type Column } from '../lib/api'

  let { profile, onopen, ontable }: {
    profile: Profile
    onopen: (database: string) => void
    ontable: (database: string, schema: string, table: string) => void
  } = $props()

  type Node = { open: boolean; loading: boolean; error: string }
  let dbs = $state<string[]>([])
  let schemas = $state<Record<string, string[]>>({})
  let tables = $state<Record<string, Table[]>>({})
  let columns = $state<Record<string, Column[]>>({})
  let nodes = $state<Record<string, Node>>({})
  let error = $state('')

  const node = (k: string): Node => (nodes[k] ??= { open: false, loading: false, error: '' })

  async function toggle(k: string, load: () => Promise<void>) {
    const n = node(k)
    n.open = !n.open
    if (n.open && !n.loading) {
      n.loading = true; n.error = ''
      try { await load() } catch (e) { n.error = errMsg(e); n.open = false }
      n.loading = false
    }
  }

  api.databases(profile.id).then((d) => {
    dbs = d
    if (d.length === 1) toggleDb(d[0])
  }).catch((e) => (error = errMsg(e)))

  const toggleDb = (db: string) => {
    onopen(db)
    return toggle(db, async () => {
      const s = await api.schemas(profile.id, db)
      schemas[db] = s
      if (s.length === 1) await loadTables(db, s[0]) // sin nivel de esquema
    })
  }
  async function loadTables(db: string, schema: string) {
    tables[`${db}/${schema}`] = await api.tables(profile.id, db, schema)
  }
  const toggleSchema = (db: string, s: string) => toggle(`${db}/${s}`, () => loadTables(db, s))
  const toggleTable = (db: string, s: string, t: string) =>
    toggle(`${db}/${s}/${t}`, async () => { columns[`${db}/${s}/${t}`] = await api.columns(profile.id, db, s, t) })
</script>

{#snippet tableRows(db: string, s: string, indent: number)}
  {#each tables[`${db}/${s}`] ?? [] as t}
    {@const k = `${db}/${s}/${t.name}`}
    <div class="item" style:padding-left="{indent}px" onclick={() => toggleTable(db, s, t.name)}
         ondblclick={() => ontable(db, s, t.name)} role="treeitem" tabindex="0" aria-selected="false"
         onkeydown={(e) => e.key === 'Enter' && ontable(db, s, t.name)}>
      <span class="caret">{node(k).open ? '▾' : '▸'}</span>
      <span class="ico">{t.type === 'view' ? '◫' : '▦'}</span>{t.name}
    </div>
    {#if node(k).open}
      {#each columns[k] ?? [] as c}
        <div class="col" style:padding-left="{indent + 28}px">
          <span class:pk={c.primaryKey}>{c.primaryKey ? '🔑' : ''}{c.name}</span>
          <span class="type">{c.type}{c.nullable ? '' : ' ·NN'}</span>
        </div>
      {/each}
    {/if}
  {/each}
{/snippet}

<div class="tree">
  {#if error}<p class="err">{error}</p>{/if}
  {#each dbs as db}
    <div class="item" onclick={() => toggleDb(db)} role="treeitem" tabindex="0" aria-selected="false" onkeydown={(e) => e.key === 'Enter' && toggleDb(db)}>
      <span class="caret">{node(db).loading ? '…' : node(db).open ? '▾' : '▸'}</span><span class="ico">⛁</span>{db}
    </div>
    {#if node(db).error}<p class="err">{node(db).error}</p>{/if}
    {#if node(db).open}
      {@const ss = schemas[db] ?? []}
      {#if ss.length === 1 && ss[0] === ''}
        {@render tableRows(db, '', 22)}
      {:else}
        {#each ss as s}
          <div class="item" style:padding-left="22px" onclick={() => toggleSchema(db, s)} role="treeitem" tabindex="0" aria-selected="false" onkeydown={(e) => e.key === 'Enter' && toggleSchema(db, s)}>
            <span class="caret">{node(`${db}/${s}`).open ? '▾' : '▸'}</span><span class="ico">▤</span>{s}
          </div>
          {#if node(`${db}/${s}`).open}{@render tableRows(db, s, 40)}{/if}
        {/each}
      {/if}
    {/if}
  {/each}
</div>

<style>
  .tree { font-size: 12.5px; }
  .item, .col { display: flex; align-items: center; gap: 4px; padding: 2px 8px; cursor: pointer; white-space: nowrap; }
  .item:hover { background: var(--panel2); }
  .col { cursor: default; color: var(--muted); justify-content: space-between; padding-right: 10px; }
  .caret { width: 10px; color: var(--muted); }
  .ico { color: var(--accent); }
  .type { font-size: 11px; opacity: .7; }
  .pk { color: var(--text); }
  .err { color: var(--danger); margin: 2px 8px; word-break: break-word; }
</style>
