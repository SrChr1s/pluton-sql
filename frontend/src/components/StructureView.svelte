<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws, selectFrom, type DataTab } from '../lib/workspace.svelte'
  import { copyText } from '../lib/clipboard'
  import { ui } from '../lib/ui.svelte'
  import type { ForeignKey } from '../lib/api'

  let { tab }: { tab: DataTab } = $props()
  const t = $derived(tab.state)
  const st = $derived(t.structure)
  let copied = $state(false)

  // Al abrir la vista, o al volver a ella tras un cambio, la estructura puede no estar cargada.
  $effect(() => { if (!t.structure && !t.structureLoading && !t.structureError) void t.loadStructure() })

  async function copyDdl() {
    if (!st?.ddl) return
    await copyText(st.ddl)
    copied = true
    setTimeout(() => (copied = false), 1500)
  }

  /** Abre la tabla referenciada; en MySQL el "esquema" del destino es su base de datos. */
  function openRef(fk: ForeignKey) {
    const d = ws.active?.driver
    if (d === 'mysql') ws.openTable(fk.refSchema || t.loc.database, '', fk.refTable)
    else ws.openTable(t.loc.database, fk.refSchema, fk.refTable)
  }

  /** Abre el diálogo de modificación; antes, descarta (con confirmación) los cambios de datos pendientes. */
  async function change(kind: string, target = '') {
    if (!(await t.canDiscard('modificas la estructura'))) return
    ui.openSchema(t.loc, kind, target, st)
  }

  const ddlAsQuery = () => ws.newTab({ title: `DDL ${t.loc.table}`, sql: st?.ddl ?? '', database: t.loc.database })
  const refName = (fk: ForeignKey) => `${fk.refSchema && ws.active?.driver !== 'mysql' ? fk.refSchema + '.' : ''}${fk.refTable}`
  const hasAction = (a: string) => !!a && a !== 'NO ACTION'
</script>

<div class="sv">
  <div class="bar">
    <button class="ghost icon" title="Recargar estructura" aria-label="Recargar estructura" onclick={() => t.loadStructure()} disabled={t.structureLoading}><Icon name="refresh" size={15} /></button>
    <span class="grow"></span>
    {#if st}
      <button class="ghost" onclick={() => change('rename_table')}><Icon name="pencil" size={13} /> Renombrar</button>
      <button class="ghost danger" onclick={() => change('drop_table')}><Icon name="trash" size={13} /> {st.isView ? 'Eliminar vista' : 'Eliminar tabla'}</button>
    {/if}
    <button class="ghost" onclick={() => ws.newTab({ title: t.loc.table, database: t.loc.database, sql: selectFrom(ws.active?.driver ?? '', t.loc.schema, t.loc.table) })}><Icon name="sql" size={14} /> SELECT</button>
  </div>

  {#if t.structureError}
    <div class="banner err" role="alert"><pre>{t.structureError}</pre></div>
  {/if}

  <div class="body">
    {#if !st}
      <div class="empty">{t.structureLoading ? 'Cargando…' : 'Sin datos'}</div>
    {:else}
      <section>
        <h3>Columnas <span class="count">{st.columns.length}</span><span class="grow"></span>{#if !st.isView}<button onclick={() => change('add_column')}><Icon name="plus" size={12} /> Columna</button>{/if}</h3>
        <table>
          <thead><tr><th class="n">#</th><th>Nombre</th><th>Tipo</th><th>Nulo</th><th>Por defecto</th><th></th>{#if !st.isView}<th></th>{/if}</tr></thead>
          <tbody>
            {#each st.columns as c, i (c.name)}
              <tr>
                <td class="n">{i + 1}</td>
                <td class="name">{c.name}</td>
                <td class="mono">{c.type}</td>
                <td>{c.nullable ? 'sí' : 'NO'}</td>
                <td class="mono dflt" title={c.default}>{c.default}</td>
                <td class="flags">
                  {#if c.primaryKey}<span class="tag pk"><Icon name="key" size={10} /> PK</span>{/if}
                  {#if c.autoIncrement}<span class="tag">AUTO</span>{/if}
                </td>
                {#if !st.isView}
                  <td class="rowacts"><button class="ghost icon" title="Modificar, renombrar o eliminar" aria-label="Editar la columna {c.name}" onclick={() => change('alter_column', c.name)}><Icon name="pencil" size={13} /></button></td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </section>

      {#if !st.isView}
        <section>
          <h3>Índices <span class="count">{st.indexes.length}</span><span class="grow"></span><button onclick={() => change('create_index')}><Icon name="plus" size={12} /> Índice</button></h3>
          {#if st.indexes.length === 0}
            <p class="none">Sin índices.</p>
          {:else}
            <table>
              <thead><tr><th>Nombre</th><th>Columnas</th><th></th><th></th></tr></thead>
              <tbody>
                {#each st.indexes as ix (ix.name)}
                  <tr>
                    <td class="name">{ix.name}</td>
                    <td class="mono">{ix.columns.join(', ')}</td>
                    <td class="flags">
                      {#if ix.primary}<span class="tag pk">PRIMARIA</span>{:else if ix.unique}<span class="tag">ÚNICO</span>{/if}
                    </td>
                    <td class="rowacts">{#if !ix.primary}<button class="ghost icon danger" title="Eliminar índice" aria-label="Eliminar el índice {ix.name}" onclick={() => change('drop_index', ix.name)}><Icon name="trash" size={13} /></button>{/if}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </section>

        <section>
          <h3>Claves foráneas <span class="count">{st.foreignKeys.length}</span><span class="grow"></span><button onclick={() => change('add_fk')} disabled={ws.active?.driver === 'sqlite'} title={ws.active?.driver === 'sqlite' ? 'SQLite no permite añadir claves foráneas a una tabla existente' : ''}><Icon name="plus" size={12} /> Clave foránea</button></h3>
          {#if st.foreignKeys.length === 0}
            <p class="none">Sin claves foráneas.</p>
          {:else}
            <table>
              <thead><tr><th>Nombre</th><th>Columnas</th><th>Referencia</th><th>Acciones</th><th></th></tr></thead>
              <tbody>
                {#each st.foreignKeys as fk (fk.name)}
                  <tr>
                    <td class="name">{fk.name}</td>
                    <td class="mono">{fk.columns.join(', ')}</td>
                    <td class="mono">
                      <button class="link" title="Abrir {refName(fk)}" onclick={() => openRef(fk)}>{refName(fk)}</button>
                      ({fk.refColumns.join(', ')})
                    </td>
                    <td class="acts">
                      {#if hasAction(fk.onDelete)}<span class="tag">ON DELETE {fk.onDelete}</span>{/if}
                      {#if hasAction(fk.onUpdate)}<span class="tag">ON UPDATE {fk.onUpdate}</span>{/if}
                    </td>
                    <td class="rowacts">{#if ws.active?.driver !== 'sqlite'}<button class="ghost icon danger" title="Eliminar clave foránea" aria-label="Eliminar la clave {fk.name}" onclick={() => change('drop_fk', fk.name)}><Icon name="trash" size={13} /></button>{/if}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          {/if}
        </section>
      {/if}

      <section>
        <h3>
          DDL
          <span class="grow"></span>
          {#if st.ddl}
            <button onclick={copyDdl}>{copied ? 'Copiado' : 'Copiar'}</button>
            <button onclick={ddlAsQuery} title="Abrir el DDL en una pestaña de consulta (no se ejecuta)"><Icon name="sql" size={13} /> Abrir en consulta</button>
          {/if}
        </h3>
        {#if st.ddlError}
          <div class="banner err" role="alert"><pre>No se pudo obtener el DDL: {st.ddlError}</pre></div>
        {:else}
          <pre class="ddl">{st.ddl}</pre>
        {/if}
      </section>
    {/if}
  </div>
</div>

<style>
  .sv { display: flex; flex-direction: column; flex: 1; min-height: 0; background: var(--surface); }
  .bar { display: flex; align-items: center; gap: 6px; padding: 6px 10px; border-bottom: 1px solid var(--border); }
  .grow { flex: 1; }
  .body { flex: 1; overflow: auto; padding: 14px 18px 24px; display: flex; flex-direction: column; gap: 22px; }
  section h3 { display: flex; align-items: center; gap: 8px; margin: 0 0 8px; font-size: 12px; text-transform: uppercase; letter-spacing: .06em; color: var(--muted); }
  .count { font-weight: 400; background: var(--surface-2); border-radius: 9px; padding: 0 7px; letter-spacing: 0; }
  h3 button { text-transform: none; letter-spacing: 0; font-weight: 400; font-size: 12px; }
  table { border-collapse: separate; border-spacing: 0; width: 100%; max-width: 980px; border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; }
  th { text-align: left; font: 600 11.5px var(--sans); color: var(--muted); background: var(--surface-2); padding: 5px 12px; border-bottom: 1px solid var(--border); }
  td { padding: 4px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
  tr:last-child td { border-bottom: none; }
  td.n, th.n { width: 1%; color: var(--muted); text-align: right; }
  .name { font-weight: 600; }
  .mono { font: 12px var(--mono); }
  .dflt { color: var(--muted); max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .flags, .acts { white-space: nowrap; }
  .rowacts { width: 1%; text-align: right; padding: 2px 6px; }
  .rowacts button { opacity: 0; padding: 3px; }
  tr:hover .rowacts button, .rowacts button:focus-visible { opacity: 1; }
  .tag { display: inline-flex; align-items: center; gap: 3px; font: 600 10px var(--mono); background: var(--accent-soft); color: var(--accent); border-radius: 3px; padding: 1px 6px; margin-right: 4px; }
  .tag.pk { background: color-mix(in srgb, var(--warn) 16%, transparent); color: var(--warn); }
  .link { background: none; border: none; padding: 0; color: var(--accent); font: inherit; text-decoration: underline; text-underline-offset: 2px; }
  .link:hover:not(:disabled) { background: none; filter: brightness(1.15); }
  .none, .empty { color: var(--muted); margin: 0; }
  .empty { padding: 28px; text-align: center; }
  .ddl { margin: 0; padding: 12px 14px; max-width: 980px; overflow: auto; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); font: 12px/1.55 var(--mono); white-space: pre; }
  .banner { padding: 7px 12px; font-size: 12.5px; border-radius: var(--radius); }
  .banner pre { margin: 0; white-space: pre-wrap; font: 12px var(--mono); }
  .banner.err { background: color-mix(in srgb, var(--danger) 12%, var(--surface)); color: var(--danger); }
</style>
