<script lang="ts">
  import Icon from './Icon.svelte'
  import { ui } from '../lib/ui.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { copyText } from '../lib/clipboard'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { api, errMsg, type Profile, type Table, type Column } from '../lib/api'

  let { profile, filter = '', onopen, ontable, onstructure, oncreate, count = $bindable(0), expanded = $bindable(false) }: {
    profile: Profile
    filter?: string
    onopen: (database: string) => void
    ontable: (database: string, schema: string, table: string) => void
    onstructure: (database: string, schema: string, table: string) => void
    oncreate: (database: string, schema: string) => void
    /** Número de tablas y vistas cargadas hasta ahora. */
    count?: number
    /** Hay algún nodo desplegado. */
    expanded?: boolean
  } = $props()

  type Node = { open: boolean; loading: boolean; error: string }
  let dbs = $state<string[]>([])
  let schemas = $state<Record<string, string[]>>({})
  let tables = $state<Record<string, Table[]>>({})
  let columns = $state<Record<string, Column[]>>({})
  let nodes = $state<Record<string, Node>>({})
  let error = $state('')
  let loading = $state(true)

  $effect(() => { expanded = Object.values(nodes).some((n) => n.open) })
  $effect(() => { count = Object.values(tables).reduce((n, l) => n + l.length, 0) })

  /** Contrae todos los nodos del árbol. */
  export function collapseAll() {
    for (const n of Object.values(nodes)) n.open = false
  }

  /** Una base está "conectada" mientras su pool esté abierto (la del perfil, las que se abren en el árbol, consultas…). */
  const isConnected = (db: string) => ws.isDbOpen(profile.id, db)

  /** Olvida lo cargado de una base (esquemas, tablas, columnas y nodos) y la deja contraída. */
  function reset(db: string) {
    delete schemas[db]
    for (const k of Object.keys(nodes)) if (k === db || k.startsWith(db + '/')) delete nodes[k]
    for (const k of Object.keys(tables)) if (k.startsWith(db + '/')) delete tables[k]
    for (const k of Object.keys(columns)) if (k.startsWith(db + '/')) delete columns[k]
  }

  // Si una base se desconecta (desde el menú del árbol o de la barra inferior) se contrae y se olvida su contenido.
  $effect(() => {
    for (const db of dbs) if (!isConnected(db) && (schemas[db] !== undefined || nodes[db])) reset(db)
  })

  const MAX_TABLES = 300
  /**
   * Despliega todo en las bases a las que ya estamos conectados (más la preferida o la primera si aún no hay
   * ninguna): base → esquemas → tablas con sus columnas. No abre bases a las que no estamos conectados.
   */
  export async function expandAll(preferred: string) {
    const first = dbs.includes(preferred) ? preferred : dbs[0]
    const targets = dbs.filter((d) => isConnected(d) || d === first)
    let tableKeys: [string, string, string][] = []
    for (const db of targets) {
      if (!node(db).open) await toggleDb(db)
      const ss = schemas[db] ?? []
      for (const s of ss) {
        if (s !== '' && !node(`${db}/${s}`).open) await toggleSchema(db, s)
        for (const t of tables[`${db}/${s}`] ?? []) tableKeys.push([db, s, t.name])
      }
    }
    tableKeys = tableKeys.filter(([d, s, t]) => !node(`${d}/${s}/${t}`).open)
    if (tableKeys.length > MAX_TABLES) {
      ui.toast(`Hay ${tableKeys.length} tablas: se desplegaron las bases y esquemas, pero no las columnas (máximo ${MAX_TABLES}).`)
      return
    }
    for (let i = 0; i < tableKeys.length; i += 8) {
      await Promise.all(tableKeys.slice(i, i + 8).map(([d, s, t]) => toggleTable(d, s, t)))
    }
  }

  /** Dónde crear una tabla nueva: la base de la pestaña actual (o la primera) y su esquema por defecto. */
  export async function defaultTarget(preferred: string): Promise<{ database: string; schema: string } | null> {
    const database = dbs.includes(preferred) ? preferred : dbs[0]
    if (!database) return null
    const list = schemas[database] ?? (schemas[database] = await api.schemas(profile.id, database))
    const schema = ['public', 'dbo'].find((s) => list.includes(s)) ?? list[0] ?? ''
    return { database, schema }
  }

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

  // Enter ejecuta la acción principal; Espacio, la de la flecha si la fila tiene (expandir columnas).
  let ctx = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  const showMenu = (e: MouseEvent, items: MenuItem[]) => { e.preventDefault(); e.stopPropagation(); ctx = { x: e.clientX, y: e.clientY, items } }
  const locOf = (database: string, schema: string, table = '') => ({ profile: profile.id, database, schema, table })

  async function reloadDb(db: string) {
    reset(db)
    await toggleDb(db)
  }

  function dbMenu(e: MouseEvent, db: string) {
    const ss = schemas[db]
    const items: MenuItem[] = [
      { label: 'Nueva consulta', icon: 'sql', run: () => { ws.newTab({ database: db }) } },
      { label: 'Nueva tabla', icon: 'table', run: async () => {
        const list = ss ?? (schemas[db] = await api.schemas(profile.id, db))
        const schema = ['public', 'dbo'].find((s) => list.includes(s)) ?? list[0] ?? ''
        oncreate(db, schema)
      } },
      { label: 'Recargar', icon: 'refresh', disabled: !isConnected(db) && profile.driver !== 'sqlite', run: () => void reloadDb(db) },
    ]
    if (isConnected(db)) items.push({ label: profile.driver === 'sqlite' ? 'Desconectar' : 'Desconectar de la base de datos', icon: 'power', run: () => void ws.disconnectDatabase(db) })
    showMenu(e, items)
  }

  function schemaMenu(e: MouseEvent, db: string, s: string) {
    showMenu(e, [
      { label: 'Nueva tabla', icon: 'table', run: () => oncreate(db, s) },
      { label: 'Recargar', icon: 'refresh', run: () => void loadTables(db, s) },
    ])
  }

  function tableMenu(e: MouseEvent, db: string, s: string, t: Table) {
    const items: MenuItem[] = [
      { label: 'Abrir datos', icon: 'table', run: () => ontable(db, s, t.name) },
      { label: 'Ver estructura', icon: 'layers', run: () => onstructure(db, s, t.name) },
      { label: 'Consultar (SELECT)', icon: 'sql', run: () => ws.openTableAsSql(db, s, t.name) },
      { label: 'Copiar nombre', icon: 'copy', run: () => { void copyText(t.name); ui.toast('Nombre copiado') } },
    ]
    if (t.type !== 'view') {
      items.push(
        { label: 'Renombrar tabla', icon: 'pencil', run: () => ui.openSchema(locOf(db, s, t.name), 'rename_table', t.name) },
        { label: 'Eliminar tabla', icon: 'trash', run: () => ui.openSchema(locOf(db, s, t.name), 'drop_table', t.name) },
      )
    }
    showMenu(e, items)
  }

  const key = (e: KeyboardEvent, fn: () => void, caret?: () => void) => {
    if (e.key === 'Enter') { e.preventDefault(); fn() }
    else if (e.key === ' ') { e.preventDefault(); (caret ?? fn)() }
  }
</script>

{#snippet row(depth: number, open: boolean | null, busy: boolean, icon: string, label: string, toggleFn: () => void, dbl?: () => void, info?: () => void, create?: () => void, caretFn?: () => void, live?: boolean, oncontext?: (e: MouseEvent) => void)}
  <div class="item" style:padding-left="{8 + depth * 14}px" role="treeitem" data-kind={icon} tabindex="0" aria-selected="false"
       aria-expanded={open === null ? undefined : open} onclick={toggleFn} ondblclick={dbl} oncontextmenu={oncontext} onkeydown={(e) => key(e, toggleFn, caretFn)}>
    <span class="caret" onclick={caretFn ? (e) => { e.stopPropagation(); caretFn() } : undefined} role="presentation">{#if busy}<span class="spin"></span>{:else if open !== null}<Icon name={open ? 'down' : 'right'} size={12} />{/if}</span>
    <Icon name={icon} size={14} /><span class="label" title={label}>{label}</span>
    {#if live}<span class="live" title="Conectado a esta base de datos" aria-label="Conectado"></span>{/if}
    {#if create}<button class="ghost icon info" title="Nueva tabla" aria-label="Nueva tabla en {label}" onclick={(e) => { e.stopPropagation(); create() }}><Icon name="plus" size={13} /></button>{/if}
    {#if info}<button class="ghost icon info" title="Ver estructura" aria-label="Ver estructura de {label}" onclick={(e) => { e.stopPropagation(); info() }}><Icon name="layers" size={12} /></button>{/if}
  </div>
{/snippet}

{#snippet tableRows(db: string, s: string, depth: number)}
  {#each visible(tables[`${db}/${s}`]) as t (t.name)}
    {@const k = `${db}/${s}/${t.name}`}
    {@render row(depth, node(k).open, node(k).loading, t.type === 'view' ? 'view' : 'table', t.name, () => { ontable(db, s, t.name); if (!node(k).open) void toggleTable(db, s, t.name) }, undefined, () => onstructure(db, s, t.name), undefined, () => toggleTable(db, s, t.name), undefined, (e) => tableMenu(e, db, s, t))}
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
  {#if ctx}<Menu at={{ x: ctx.x, y: ctx.y }} items={ctx.items} onclose={() => (ctx = null)} />{/if}
  {#if loading}<p class="empty">Cargando…</p>{/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#each dbs as db (db)}
    {@render row(0, node(db).open, node(db).loading, 'database', db, () => toggleDb(db), undefined, undefined, (schemas[db]?.length === 1 && schemas[db][0] === '') ? () => oncreate(db, '') : undefined, undefined, profile.driver !== 'sqlite' && isConnected(db), (e) => dbMenu(e, db))}
    {#if node(db).error}<p class="err">{node(db).error}</p>{/if}
    {#if node(db).open}
      {@const ss = schemas[db] ?? []}
      {#if ss.length === 1 && ss[0] === ''}
        {@render tableRows(db, '', 1)}
      {:else}
        {#each ss as s (s)}
          {@render row(1, node(`${db}/${s}`).open, node(`${db}/${s}`).loading, 'layers', s, () => toggleSchema(db, s), undefined, undefined, () => oncreate(db, s), undefined, undefined, (e) => schemaMenu(e, db, s))}
          {#if node(`${db}/${s}`).open}{@render tableRows(db, s, 2)}{/if}
        {/each}
      {/if}
    {/if}
  {/each}
</div>

<style>
  .tree { font-size: 12.5px; padding: 2px 0 8px; }
  .item, .col { display: flex; align-items: center; gap: 5px; padding-right: 8px; height: 22px; white-space: nowrap; }
  .item { cursor: pointer; }
  .item:hover { background: var(--hover); }
  .item :global(svg) { color: var(--muted); }
  .item[data-kind="table"] :global(svg) { color: var(--accent); }
  .item[data-kind="view"] :global(svg) { color: var(--ok); }
  .item[data-kind="database"] :global(svg) { color: var(--text); }
  .live { width: 7px; height: 7px; border-radius: 50%; background: var(--ok); flex: none; box-shadow: 0 0 0 3px color-mix(in srgb, var(--ok) 22%, transparent); }
  .label { overflow: hidden; text-overflow: ellipsis; flex: 1; }
  .info { opacity: 0; padding: 2px; margin-left: auto; }
  .item:hover .info, .info:focus-visible { opacity: 1; }
  .caret { width: 16px; height: 18px; margin: 0 -2px; display: grid; place-items: center; border-radius: 3px; }
  .caret:hover { background: var(--hover); }
  .col { justify-content: space-between; color: var(--muted); height: 20px; gap: 10px; }
  .cname { display: flex; align-items: center; gap: 4px; overflow: hidden; text-overflow: ellipsis; }
  .pk { color: var(--warn); display: inline-flex; }
  .type { font: 11px var(--mono); opacity: .75; }
  .empty { margin: 4px 8px; color: var(--muted); font-size: 12px; }
  .err { color: var(--danger); margin: 2px 8px; word-break: break-word; font-size: 12px; }
  .spin { width: 10px; height: 10px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
