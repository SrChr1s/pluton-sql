<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errMsg, type SchemaOp, type SchemaPlan, type TableStructure, type Table } from '../lib/api'
  import { ws } from '../lib/workspace.svelte'
  import type { TableLocation } from '../lib/table.svelte'

  let { loc, kind: initialKind, target = '', structure, oncancel, ondone }: {
    loc: TableLocation
    kind: string
    target?: string
    structure: TableStructure | null
    oncancel: () => void
    ondone: (loc: TableLocation, op: { kind: string; newName?: string }) => void
  } = $props()

  const driver = ws.active?.driver ?? ''
  const mysql = driver === 'mysql'
  const cols = structure?.columns ?? []
  const current = cols.find((c) => c.name === target)

  let kind = $state(initialKind)
  let step = $state<'form' | 'preview'>('form')
  let busy = $state(false)
  let error = $state('')
  let plan = $state<SchemaPlan | null>(null)

  // --- campos del formulario ---
  let colName = $state(kind === 'add_column' ? '' : target)
  let colType = $state(current?.type ?? '')
  let colNullable = $state(current ? current.nullable : true)
  let colDefault = $state(current?.default ?? '')
  let newName = $state(kind === 'rename_column' || kind === 'rename_table' ? (kind === 'rename_table' ? loc.table : target) : '')
  let idxName = $state('')
  let idxCols = $state<string[]>([])
  let idxUnique = $state(false)
  let fkName = $state('')
  let fkCols = $state<string[]>([])
  let fkRefTable = $state('')
  let fkRefCols = $state<string[]>([])
  let fkOnDelete = $state('')
  let fkOnUpdate = $state('')
  let refTables = $state<Table[]>([])
  let refColumns = $state<string[]>([])
  type NewCol = { name: string; type: string; nullable: boolean; default: string; primaryKey: boolean; autoIncrement: boolean }
  let tableName = $state('')
  const intType = driver === 'sqlite' ? 'INTEGER' : 'int'
  let newCols = $state<NewCol[]>([{ name: 'id', type: intType, nullable: false, default: '', primaryKey: true, autoIncrement: true }])

  const types: Record<string, string[]> = {
    postgres: ['integer', 'bigint', 'smallint', 'text', 'varchar(255)', 'boolean', 'numeric(10,2)', 'date', 'timestamp', 'timestamptz', 'uuid', 'jsonb', 'bytea'],
    mysql: ['int', 'bigint', 'tinyint(1)', 'varchar(255)', 'text', 'decimal(10,2)', 'date', 'datetime', 'timestamp', 'json', 'blob'],
    sqlite: ['INTEGER', 'TEXT', 'REAL', 'BLOB', 'NUMERIC'],
    mssql: ['int', 'bigint', 'bit', 'nvarchar(255)', 'varchar(255)', 'nvarchar(max)', 'decimal(10,2)', 'date', 'datetime2', 'uniqueidentifier', 'varbinary(max)'],
  }
  const actions = ['', 'NO ACTION', 'CASCADE', 'SET NULL', 'SET DEFAULT', 'RESTRICT']

  const isColumnKind = ['alter_column', 'rename_column', 'drop_column'].includes(initialKind)
  const titles: Record<string, string> = {
    add_column: 'Añadir columna', alter_column: 'Modificar columna', rename_column: 'Renombrar columna', drop_column: 'Eliminar columna',
    create_index: 'Crear índice', drop_index: 'Eliminar índice', add_fk: 'Añadir clave foránea', drop_fk: 'Eliminar clave foránea',
    rename_table: 'Renombrar tabla', drop_table: loc.table && structure?.isView ? 'Eliminar vista' : 'Eliminar tabla', create_table: 'Nueva tabla',
  }

  // Al elegir la tabla referenciada se cargan sus columnas.
  $effect(() => {
    if (kind !== 'add_fk' || !fkRefTable) return
    refColumns = []
    fkRefCols = []
    api.columns(loc.profile, loc.database, loc.schema, fkRefTable).then((c) => (refColumns = c.map((x) => x.name))).catch(() => {})
  })
  $effect(() => {
    if (initialKind !== 'add_fk') return
    api.tables(loc.profile, loc.database, loc.schema).then((t) => {
      refTables = t.filter((x) => x.type !== 'view')
      if (!fkRefTable && refTables.length) fkRefTable = refTables[0].name
    }).catch(() => {})
  })
  // Nombre sugerido para índices y claves foráneas: se actualiza mientras el usuario no lo escriba a mano.
  let idxNameTouched = $state(false)
  let fkNameTouched = $state(false)
  $effect(() => { if (kind === 'create_index' && !idxNameTouched && idxCols.length) idxName = `ix_${loc.table}_${idxCols.join('_')}` })
  $effect(() => { if (kind === 'add_fk' && !fkNameTouched && fkCols.length && fkRefTable) fkName = `fk_${loc.table}_${fkCols.join('_')}` })

  const toggle = (list: string[], v: string) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v])

  function buildOp(): SchemaOp {
    const base = { kind, target: '', newName: '' } as Record<string, unknown>
    switch (kind) {
      case 'add_column': base.column = { name: colName, type: colType, nullable: colNullable, default: colDefault, primaryKey: false, autoIncrement: false }; break
      case 'alter_column': base.target = target; base.column = { name: target, type: colType, nullable: colNullable, default: colDefault, primaryKey: false, autoIncrement: false }; break
      case 'rename_column': base.target = target; base.newName = newName; break
      case 'drop_column': base.target = target; break
      case 'rename_table': base.newName = newName; break
      case 'create_index': base.index = { name: idxName, columns: idxCols, unique: idxUnique }; break
      case 'drop_index': case 'drop_fk': base.target = target; break
      case 'add_fk':
        base.fk = { name: fkName, columns: fkCols, refSchema: mysql ? '' : loc.schema, refTable: fkRefTable, refColumns: fkRefCols, onDelete: fkOnDelete, onUpdate: fkOnUpdate }
        break
      case 'create_table': base.columns = newCols.map((c) => ({ ...c })); break
    }
    return base as unknown as SchemaOp
  }

  const ref = () => ({ ...loc, table: kind === 'create_table' ? tableName : loc.table })
  const doneLoc = () => ref()

  async function preview() {
    busy = true
    error = ''
    try {
      plan = await api.previewSchema(ref(), buildOp())
      plan.statements = plan.statements ?? []
      plan.warnings = plan.warnings ?? []
      step = 'preview'
    } catch (e) { error = errMsg(e) }
    busy = false
  }

  async function execute() {
    busy = true
    error = ''
    try {
      const op = buildOp()
      await api.applySchema(ref(), op)
      ondone(doneLoc(), { kind, newName: (op as { newName?: string }).newName })
    } catch (e) { error = errMsg(e); busy = false }
  }

  function addNewCol() { newCols.push({ name: '', type: types[driver]?.[0] ?? '', nullable: true, default: '', primaryKey: false, autoIncrement: false }) }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && !busy && oncancel()} />

<div class="overlay" role="presentation" onclick={() => !busy && oncancel()}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" class:wide={kind === 'create_table'} role="dialog" aria-modal="true" aria-labelledby="sc-title" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <h3 id="sc-title">{titles[kind]}{loc.table && kind !== 'create_table' ? ` · ${loc.table}` : ''}</h3>

    {#if step === 'form'}
      <form onsubmit={(e) => { e.preventDefault(); preview() }}>
        {#if isColumnKind}
          <div class="seg" role="tablist" aria-label="Operación sobre la columna {target}">
            {#each [['alter_column', 'Modificar'], ['rename_column', 'Renombrar'], ['drop_column', 'Eliminar']] as [k, label]}
              <button type="button" role="tab" class:on={kind === k} aria-selected={kind === k} onclick={() => { kind = k; error = '' }}>{label}</button>
            {/each}
          </div>
          <p class="sub">Columna <b>{target}</b>{current ? ` · ${current.type}` : ''}</p>
        {/if}

        {#if kind === 'add_column' || kind === 'alter_column'}
          {#if kind === 'add_column'}<label>Nombre <input bind:value={colName} required /></label>{/if}
          <label>Tipo <input bind:value={colType} list="sc-types" required placeholder="p. ej. varchar(50)" /></label>
          <datalist id="sc-types">{#each types[driver] ?? [] as t}<option value={t}></option>{/each}</datalist>
          <label class="check"><input type="checkbox" bind:checked={colNullable} /> Admite NULL</label>
          <label>Valor por defecto <input bind:value={colDefault} placeholder="expresión SQL, p. ej. 0, 'x' o now() — vacío = ninguno" class="mono" /></label>
        {:else if kind === 'rename_column' || kind === 'rename_table'}
          <label>Nombre nuevo <input bind:value={newName} required /></label>
        {:else if kind === 'drop_column'}
          <p class="note danger">Se borrará la columna <b>{target}</b> con todos sus datos.</p>
        {:else if kind === 'drop_table'}
          <p class="note danger">Se eliminará <b>{loc.table}</b>{structure?.isView ? '' : ' y todos sus datos'}.</p>
        {:else if kind === 'drop_index'}
          <p class="note">Se eliminará el índice <b>{target}</b>.</p>
        {:else if kind === 'drop_fk'}
          <p class="note">Se eliminará la clave foránea <b>{target}</b>.</p>
        {:else if kind === 'create_index'}
          <label>Nombre <input bind:value={idxName} oninput={() => (idxNameTouched = true)} required /></label>
          <fieldset><legend>Columnas (el orden de selección importa)</legend>
            <div class="chips">{#each cols as c (c.name)}
              <button type="button" class="chip" class:on={idxCols.includes(c.name)} onclick={() => (idxCols = toggle(idxCols, c.name))}>
                {#if idxCols.includes(c.name)}<b>{idxCols.indexOf(c.name) + 1}</b>{/if}{c.name}
              </button>{/each}</div>
          </fieldset>
          <label class="check"><input type="checkbox" bind:checked={idxUnique} /> Único</label>
        {:else if kind === 'add_fk'}
          <label>Nombre <input bind:value={fkName} oninput={() => (fkNameTouched = true)} required placeholder="fk_{loc.table}_…" /></label>
          <fieldset><legend>Columnas de esta tabla</legend>
            <div class="chips">{#each cols as c (c.name)}
              <button type="button" class="chip" class:on={fkCols.includes(c.name)} onclick={() => (fkCols = toggle(fkCols, c.name))}>
                {#if fkCols.includes(c.name)}<b>{fkCols.indexOf(c.name) + 1}</b>{/if}{c.name}
              </button>{/each}</div>
          </fieldset>
          <label>Tabla referenciada
            <select bind:value={fkRefTable}>{#each refTables as t (t.name)}<option value={t.name}>{t.name}</option>{/each}</select>
          </label>
          <fieldset><legend>Columnas referenciadas (mismo número y orden)</legend>
            <div class="chips">{#each refColumns as c (c)}
              <button type="button" class="chip" class:on={fkRefCols.includes(c)} onclick={() => (fkRefCols = toggle(fkRefCols, c))}>
                {#if fkRefCols.includes(c)}<b>{fkRefCols.indexOf(c) + 1}</b>{/if}{c}
              </button>{/each}</div>
          </fieldset>
          <div class="row">
            <label>ON DELETE <select bind:value={fkOnDelete}>{#each actions as a}<option value={a}>{a || '(por defecto)'}</option>{/each}</select></label>
            <label>ON UPDATE <select bind:value={fkOnUpdate}>{#each actions as a}<option value={a}>{a || '(por defecto)'}</option>{/each}</select></label>
          </div>
        {:else if kind === 'create_table'}
          <label>Nombre de la tabla <input bind:value={tableName} required /></label>
          <p class="sub">en {loc.schema ? `${loc.schema} · ` : ''}{loc.database}</p>
          <div class="grid">
            <div class="gh">Nombre</div><div class="gh">Tipo</div><div class="gh c">NULL</div><div class="gh">Por defecto</div><div class="gh c">PK</div><div class="gh c">AUTO</div><div class="gh"></div>
            {#each newCols as c, i (i)}
              <input bind:value={c.name} aria-label="Nombre de la columna {i + 1}" required />
              <input bind:value={c.type} list="sc-types" aria-label="Tipo de la columna {i + 1}" required />
              <input type="checkbox" bind:checked={c.nullable} disabled={c.primaryKey} aria-label="Admite NULL" />
              <input bind:value={c.default} class="mono" aria-label="Valor por defecto" />
              <input type="checkbox" bind:checked={c.primaryKey} onchange={() => { if (c.primaryKey) c.nullable = false }} aria-label="Clave primaria" />
              <input type="checkbox" bind:checked={c.autoIncrement} aria-label="Autoincremento" />
              <button type="button" class="ghost icon" aria-label="Quitar columna" onclick={() => newCols.splice(i, 1)} disabled={newCols.length === 1}><Icon name="x" size={13} /></button>
            {/each}
          </div>
          <datalist id="sc-types">{#each types[driver] ?? [] as t}<option value={t}></option>{/each}</datalist>
          <button type="button" class="ghost add" onclick={addNewCol}><Icon name="plus" size={13} /> Columna</button>
        {/if}

        {#if error}<p class="err" role="alert">{error}</p>{/if}
        <div class="actions">
          <button type="button" onclick={oncancel}>Cancelar</button>
          <button type="submit" class="primary" disabled={busy}>{busy ? 'Generando…' : 'Ver SQL'}</button>
        </div>
      </form>
    {:else if plan}
      <p class="sub">Este es el SQL que se ejecutará. Revísalo antes de confirmar.</p>
      <pre class="sql">{plan.statements.join(';\n\n')};</pre>
      {#each plan.warnings as w}<p class="warn">⚠ {w}</p>{/each}
      {#if plan.destructive}<p class="note danger">Esta operación es destructiva y no se puede deshacer.</p>{/if}
      {#if !plan.transactional}<p class="warn">⚠ Se ejecuta sentencia a sentencia, sin transacción.</p>{:else if plan.statements.length > 1}<p class="hint">Todas las sentencias se ejecutan en una transacción.</p>{/if}
      {#if error}<p class="err" role="alert">{error}</p>{/if}
      <div class="actions">
        <button type="button" onclick={() => { step = 'form'; error = '' }} disabled={busy}>Atrás</button>
        <button type="button" class={plan.destructive ? 'primary dng' : 'primary'} onclick={execute} disabled={busy}>{busy ? 'Ejecutando…' : 'Ejecutar'}</button>
      </div>
    {/if}
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 20; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow); padding: 18px 20px; width: 520px; max-width: 94vw; max-height: 92vh; overflow: auto; }
  .modal.wide { width: 860px; }
  h3 { margin: 0 0 10px; font-size: 15px; }
  form { display: flex; flex-direction: column; gap: 10px; }
  label { display: flex; flex-direction: column; gap: 3px; color: var(--muted); }
  label.check { flex-direction: row; align-items: center; gap: 8px; }
  label.check input { width: auto; }
  .row { display: flex; gap: 10px; }
  .row label { flex: 1; }
  fieldset { border: 1px solid var(--border); border-radius: var(--radius); padding: 6px 10px 10px; margin: 0; }
  legend { color: var(--muted); padding: 0 4px; font-size: 12px; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chip { border-radius: 14px; padding: 2px 10px; font: 12px var(--mono); }
  .chip.on { background: var(--accent-soft); border-color: var(--accent); color: var(--accent); }
  .chip b { background: var(--accent); color: var(--accent-fg); border-radius: 50%; width: 15px; height: 15px; display: inline-grid; place-items: center; font-size: 10px; margin-right: 5px; }
  .seg { display: flex; border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; align-self: flex-start; }
  .seg button { border: none; border-radius: 0; padding: 4px 14px; background: transparent; }
  .seg button.on { background: var(--accent); color: var(--accent-fg); font-weight: 600; }
  .sub { margin: 0; color: var(--muted); }
  .note { margin: 0; padding: 8px 10px; border-radius: var(--radius); background: var(--surface-2); }
  .note.danger { background: color-mix(in srgb, var(--danger) 12%, var(--surface)); color: var(--danger); }
  .warn { margin: 6px 0 0; color: var(--warn); font-size: 12.5px; }
  .hint { margin: 6px 0 0; color: var(--muted); font-size: 12.5px; }
  .err { margin: 0; color: var(--danger); white-space: pre-wrap; word-break: break-word; }
  .mono { font-family: var(--mono); font-size: 12px; }
  .sql { margin: 8px 0 0; padding: 10px 12px; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); font: 12px/1.55 var(--mono); white-space: pre-wrap; word-break: break-word; max-height: 40vh; overflow: auto; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
  button.dng, button.dng:hover:not(:disabled) { background: var(--danger); border-color: var(--danger); color: #fff; }
  button.dng:hover:not(:disabled) { filter: brightness(1.1); }
  .grid { display: grid; grid-template-columns: 1.3fr 1.3fr 44px 1fr 44px 54px 30px; gap: 6px 8px; align-items: center; }
  .gh { font-size: 11px; color: var(--muted); font-weight: 600; }
  .gh.c { text-align: center; }
  .grid input[type='checkbox'] { justify-self: center; width: auto; }
  .add { align-self: flex-start; }
</style>
