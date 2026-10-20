<script lang="ts">
  import { untrack } from 'svelte'
  import Icon from './Icon.svelte'
  import { ui } from '../lib/ui.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { copyText } from '../lib/clipboard'
  import Menu, { type MenuItem } from './Menu.svelte'
  import { api, errMsg, type Profile, type Table, type Column } from '../lib/api'

  // Se explora una sola base de datos a la vez: la elegida en el selector. Cambiarla remonta el árbol.
  let { profile, database, filter = '', ontable, onstructure, oncreate, count = $bindable(0), expanded = $bindable(false) }: {
    profile: Profile
    database: string
    filter?: string
    ontable: (database: string, schema: string, table: string) => void
    onstructure: (database: string, schema: string, table: string) => void
    oncreate: (database: string, schema: string) => void
    /** Número de tablas y vistas cargadas hasta ahora. */
    count?: number
    /** Hay algún nodo desplegado. */
    expanded?: boolean
  } = $props()

  type Node = { open: boolean; loading: boolean; error: string }
  const db = untrack(() => database)
  let schemas = $state<string[] | null>(null)
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

  /** Con un solo esquema (o sin nivel de esquema) las tablas cuelgan directamente de la raíz. */
  const flat = $derived(schemas !== null && schemas.length <= 1)

  const MAX_TABLES = 300
  /** Despliega todos los esquemas y, mientras no sean demasiadas, las columnas de cada tabla. */
  export async function expandAll() {
    let tableKeys: [string, string][] = []
    for (const s of schemas ?? []) {
      if (s !== '' && !flat && !node(s).open) await toggleSchema(s)
      for (const t of tables[s] ?? []) tableKeys.push([s, t.name])
    }
    tableKeys = tableKeys.filter(([s, t]) => !node(`${s}/${t}`).open)
    if (tableKeys.length > MAX_TABLES) {
      ui.toast(`Hay ${tableKeys.length} tablas: se desplegaron los esquemas, pero no las columnas (máximo ${MAX_TABLES}).`)
      return
    }
    for (let i = 0; i < tableKeys.length; i += 8) {
      await Promise.all(tableKeys.slice(i, i + 8).map(([s, t]) => toggleTable(s, t)))
    }
  }

  /** Dónde crear una tabla nueva: el esquema por defecto de la base elegida. */
  export function defaultTarget(): { database: string; schema: string } | null {
    if (!db || !schemas) return null
    return { database: db, schema: ['public', 'dbo'].find((s) => schemas!.includes(s)) ?? schemas[0] ?? '' }
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

  async function loadTables(schema: string) {
    tables[schema] = await api.tables(profile.id, db, schema)
  }

  if (db) {
    api.schemas(profile.id, db)
      .then(async (s) => {
        schemas = s
        if (s.length <= 1) await loadTables(s[0] ?? '')
      })
      .catch((e) => (error = errMsg(e)))
      .finally(() => (loading = false))
  } else {
    loading = false
  }

  const toggleSchema = (s: string) => toggle(s, () => loadTables(s))
  const toggleTable = (s: string, t: string) =>
    toggle(`${s}/${t}`, async () => { columns[`${s}/${t}`] = await api.columns(profile.id, db, s, t) })

  // Enter ejecuta la acción principal; Espacio, la de la flecha si la fila tiene (expandir columnas).
  let ctx = $state<{ x: number; y: number; items: MenuItem[] } | null>(null)
  const showMenu = (e: MouseEvent, items: MenuItem[]) => { e.preventDefault(); e.stopPropagation(); ctx = { x: e.clientX, y: e.clientY, items } }
  const locOf = (schema: string, table = '') => ({ profile: profile.id, database: db, schema, table })

  function schemaMenu(e: MouseEvent, s: string) {
    showMenu(e, [
      { label: 'Nueva tabla', icon: 'table', run: () => oncreate(db, s) },
      { label: 'Recargar', icon: 'refresh', run: () => void loadTables(s) },
    ])
  }

  function tableMenu(e: MouseEvent, s: string, t: Table) {
    const items: MenuItem[] = [
      { label: 'Abrir datos', icon: 'table', run: () => ontable(db, s, t.name) },
      { label: 'Ver estructura', icon: 'layers', run: () => onstructure(db, s, t.name) },
      { label: 'Consultar (SELECT)', icon: 'sql', run: () => ws.openTableAsSql(db, s, t.name) },
      { label: 'Copiar nombre', icon: 'copy', run: () => { void copyText(t.name); ui.toast('Nombre copiado') } },
    ]
    if (t.type !== 'view') {
      items.push(
        { label: 'Renombrar tabla', icon: 'pencil', run: () => ui.openSchema(locOf(s, t.name), 'rename_table', t.name) },
        { label: 'Eliminar tabla', icon: 'trash', run: () => ui.openSchema(locOf(s, t.name), 'drop_table', t.name) },
      )
    }
    showMenu(e, items)
  }

  const key = (e: KeyboardEvent, fn: () => void, caret?: () => void) => {
    if (e.key === 'Enter') { e.preventDefault(); fn() }
    else if (e.key === ' ') { e.preventDefault(); (caret ?? fn)() }
  }
</script>

{#snippet row(depth: number, open: boolean | null, busy: boolean, icon: string, label: string, toggleFn: () => void, dbl?: () => void, info?: () => void, create?: () => void, caretFn?: () => void, oncontext?: (e: MouseEvent) => void)}
  <div class="item" style:padding-left="{8 + depth * 14}px" role="treeitem" data-kind={icon} tabindex="0" aria-selected="false"
       aria-expanded={open === null ? undefined : open} onclick={toggleFn} ondblclick={dbl} oncontextmenu={oncontext} onkeydown={(e) => key(e, toggleFn, caretFn)}>
    <span class="caret" onclick={caretFn ? (e) => { e.stopPropagation(); caretFn() } : undefined} role="presentation">{#if busy}<span class="spin"></span>{:else if open !== null}<Icon name={open ? 'down' : 'right'} size={12} />{/if}</span>
    <Icon name={icon} size={14} /><span class="label" title={label}>{label}</span>
    {#if create}<button class="ghost icon info" title="Nueva tabla" aria-label="Nueva tabla en {label}" onclick={(e) => { e.stopPropagation(); create() }}><Icon name="plus" size={13} /></button>{/if}
    {#if info}<button class="ghost icon info" title="Ver estructura" aria-label="Ver estructura de {label}" onclick={(e) => { e.stopPropagation(); info() }}><Icon name="layers" size={12} /></button>{/if}
  </div>
{/snippet}

{#snippet tableRows(s: string, depth: number)}
  {#each visible(tables[s]) as t (t.name)}
    {@const k = `${s}/${t.name}`}
    {@render row(depth, node(k).open, node(k).loading, t.type === 'view' ? 'view' : 'table', t.name, () => { ontable(db, s, t.name); void toggleTable(s, t.name) }, undefined, () => onstructure(db, s, t.name), undefined, () => toggleTable(s, t.name), (e) => tableMenu(e, s, t))}
    {#if node(k).open}
      {#each columns[k] ?? [] as c (c.name)}
        <div class="col" style:padding-left="{8 + (depth + 1) * 14 + 16}px" title="{c.name} {c.type}{c.nullable ? '' : ' NOT NULL'}{c.default ? ' DEFAULT ' + c.default : ''}">
          <span class="cname">{#if c.primaryKey}<span class="pk"><Icon name="key" size={11} /></span>{/if}{c.name}</span>
          <span class="type">{c.type}{c.nullable ? '' : ' · NN'}</span>
        </div>
      {/each}
    {/if}
  {/each}
  {#if q && tables[s] && visible(tables[s]).length === 0}
    <p class="empty" style:padding-left="{8 + depth * 14 + 16}px">Sin coincidencias</p>
  {/if}
{/snippet}

<div class="tree" role="tree">
  {#if ctx}<Menu at={{ x: ctx.x, y: ctx.y }} items={ctx.items} onclose={() => (ctx = null)} />{/if}
  {#if !db}<p class="empty">Elige una base de datos en el selector de arriba.</p>{/if}
  {#if loading && db}<p class="empty"><span class="orbit"></span> Conectando…</p>{/if}
  {#if error}<p class="err">{error}</p>{/if}
  {#if schemas}
    {#if flat}
      {@render tableRows(schemas[0] ?? '', 0)}
      {#if tables[schemas[0] ?? '']?.length === 0}<p class="empty">Esta base no tiene tablas.</p>{/if}
    {:else}
      {#each schemas as s (s)}
        {@render row(0, node(s).open, node(s).loading, 'layers', s, () => toggleSchema(s), undefined, undefined, () => oncreate(db, s), undefined, (e) => schemaMenu(e, s))}
        {#if node(s).error}<p class="err">{node(s).error}</p>{/if}
        {#if node(s).open}{@render tableRows(s, 1)}{/if}
      {/each}
    {/if}
  {/if}
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
