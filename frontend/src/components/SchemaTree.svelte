<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errMsg, type Profile, type Table, type Column } from '../lib/api'

  let { profile, filter = '', onopen, ontable, onstructure }: {
    profile: Profile
    filter?: string
    onopen: (database: string) => void
    ontable: (database: string, schema: string, table: string) => void
    onstructure: (database: string, schema: string, table: string) => void
  } = $props()

  type Node = { open: boolean; loading: boolean; error: string }
  let dbs = $state<string[]>([])
  let schemas = $state<Record<string, string[]>>({})
  let tables = $state<Record<string, Table[]>>({})
  let columns = $state<Record<string, Column[]>>({})
  let nodes = $state<Record<string, Node>>({})
  let error = $state('')
  let loading = $state(true)

  // Lectura pura (usable en la plantilla): no crea estado. El nodo se crea solo al expandir.
  const IDLE: Node = { open: false, loading: false, error: '' }
  const node = (k: string): Node => nodes[k] ?? IDLE
  const q = $derived(filter.trim().toLowerCase())
  const visible = (list: Table[] = []) => (q ? list.filter((t) => t.name.toLowerCase().includes(q)) : list)

  async function toggle(k: string, load: () => Promise<void>) {
    // Se relee tras asignar: `??=` devuelve el objeto sin envolver y mutarlo no notificaría a la vista.
    if (!nodes[k]) nodes[k] = { open: false, loading: false, error: '' }
    const n = nodes[k]
    n.open = !n.open
    if (n.open && !n.loading) {
      n.loading = true; n.error = ''
      try { await load() } catch (e) { n.error = errMsg(e); n.open = false }
      n.loading = false
    }
  }

  api.databases(profile.id)
    .then((d) => { dbs = d; if (d.length === 1) toggleDb(d[0]) })
    .catch((e) => (error = errMsg(e)))
    .finally(() => (loading = false))

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

  const key = (e: KeyboardEvent, fn: () => void) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fn() } }
</script>

{#snippet row(depth: number, open: boolean | null, busy: boolean, icon: string, label: string, toggleFn: () => void, dbl?: () => void, info?: () => void)}
  <div class="item" style:padding-left="{8 + depth * 14}px" role="treeitem" tabindex="0" aria-selected="false"
       aria-expanded={open === null ? undefined : open} onclick={toggleFn} ondblclick={dbl} onkeydown={(e) => key(e, toggleFn)}>
    <span class="caret">{#if busy}<span class="spin"></span>{:else if open !== null}<Icon name={open ? 'down' : 'right'} size={12} />{/if}</span>
    <Icon name={icon} size={14} /><span class="label" title={label}>{label}</span>
    {#if info}<button class="ghost icon info" title="Ver estructura" aria-label="Ver estructura de {label}" onclick={(e) => { e.stopPropagation(); info() }}><Icon name="layers" size={12} /></button>{/if}
  </div>
{/snippet}

{#snippet tableRows(db: string, s: string, depth: number)}
  {#each visible(tables[`${db}/${s}`]) as t (t.name)}
    {@const k = `${db}/${s}/${t.name}`}
    {@render row(depth, node(k).open, node(k).loading, t.type === 'view' ? 'view' : 'table', t.name, () => toggleTable(db, s, t.name), () => ontable(db, s, t.name), () => onstructure(db, s, t.name))}
    {#if node(k).open}
      {#each columns[k] ?? [] as c (c.name)}
        <div class="col" style:padding-left="{8 + (depth + 1) * 14 + 16}px" title="{c.name} {c.type}{c.nullable ? '' : ' NOT NULL'}{c.default ? ' DEFAULT ' + c.default : ''}">
          <span class="cname">{#if c.primaryKey}<span class="pk"><Icon name="key" size={11} /></span>{/if}{c.name}</span>
          <span class="type">{c.type}{c.nullable ? '' : ' · NN'}</span>
        </div>
      {/each}
    {/if}
  {/each}
  {#if q && tables[`${db}/${s}`] && visible(tables[`${db}/${s}`]).length === 0}
    <p class="empty" style:padding-left="{8 + depth * 14 + 16}px">Sin coincidencias</p>
  {/if}
{/snippet}

<div class="tree" role="tree">
  {#if loading}<p class="empty">Cargando…</p>{/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#each dbs as db (db)}
    {@render row(0, node(db).open, node(db).loading, 'database', db, () => toggleDb(db))}
    {#if node(db).error}<p class="err">{node(db).error}</p>{/if}
    {#if node(db).open}
      {@const ss = schemas[db] ?? []}
      {#if ss.length === 1 && ss[0] === ''}
        {@render tableRows(db, '', 1)}
      {:else}
        {#each ss as s (s)}
          {@render row(1, node(`${db}/${s}`).open, node(`${db}/${s}`).loading, 'layers', s, () => toggleSchema(db, s))}
          {#if node(`${db}/${s}`).open}{@render tableRows(db, s, 2)}{/if}
        {/each}
      {/if}
    {/if}
  {/each}
</div>

<style>
  .tree { font-size: 12.5px; padding: 2px 0 8px; }
  .item, .col { display: flex; align-items: center; gap: 5px; padding-right: 8px; height: 24px; white-space: nowrap; }
  .item { cursor: pointer; }
  .item:hover { background: var(--hover); }
  .item :global(svg) { color: var(--muted); }
  .label { overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .info { opacity: 0; padding: 2px; margin-left: auto; }
  .item:hover .info, .info:focus-visible { opacity: 1; }
  .caret { width: 12px; display: grid; place-items: center; }
  .col { justify-content: space-between; color: var(--muted); height: 21px; gap: 10px; }
  .cname { display: flex; align-items: center; gap: 4px; overflow: hidden; text-overflow: ellipsis; }
  .pk { color: var(--warn); display: inline-flex; }
  .type { font: 11px var(--mono); opacity: .75; }
  .empty { margin: 4px 8px; color: var(--muted); font-size: 12px; }
  .err { color: var(--danger); margin: 2px 8px; word-break: break-word; font-size: 12px; }
  .spin { width: 10px; height: 10px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
