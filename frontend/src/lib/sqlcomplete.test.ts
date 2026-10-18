import { describe, expect, it } from 'vitest'
import { complete, lexState, referencedTables, type SchemaIndex } from './sqlcomplete'
import { needsQuote, quoteIdent, quoteIfNeeded, unquoteIdent } from './sql'

const t = (schema: string, name: string, cols: [string, string][], isView = false) => ({
  schema, name, isView, columns: cols.map(([name, type]) => ({ name, type })),
})

// PostgreSQL: esquema por defecto "public" y otro esquema "ventas"
const pg: SchemaIndex = {
  defaultSchema: 'public',
  tables: [
    t('public', 'clientes', [['id', 'integer'], ['nombre', 'text'], ['Fecha Alta', 'date'], ['order', 'int']]),
    t('public', 'pedidos', [['id', 'integer'], ['cliente_id', 'integer'], ['total', 'numeric']]),
    t('public', 'v_activos', [['id', 'integer']], true),
    t('ventas', 'pedidos', [['id', 'integer'], ['monto', 'numeric']]), // mismo nombre que public.pedidos
    t('ventas', 'facturas', [['numero', 'text']]),
  ],
}
// MySQL/SQLite: sin esquemas
const my: SchemaIndex = { defaultSchema: '', tables: [t('', 'clientes', [['id', 'int'], ['Fecha Alta', 'date']]), t('', 'pedidos', [['id', 'int']])] }

/** Marca el cursor con "|" y devuelve el texto y la posición. */
function at(sql: string): [string, number] {
  const i = sql.indexOf('|')
  return [sql.slice(0, i) + sql.slice(i + 1), i]
}
function labels(sql: string, idx = pg, driver = 'postgres', explicit = false): string[] | null {
  const [text, pos] = at(sql)
  return complete(text, pos, idx, driver, explicit)?.options.map((o) => o.label) ?? null
}
function run(sql: string, idx = pg, driver = 'postgres', explicit = false) {
  const [text, pos] = at(sql)
  return complete(text, pos, idx, driver, explicit)
}

describe('lexState', () => {
  it('detecta cadenas y comentarios abiertos', () => {
    expect(lexState("SELECT 'abc")).toBe('string')
    expect(lexState("SELECT 'it''s open")).toBe('string')
    expect(lexState("SELECT 'cerrada' ")).toBe('code')
    expect(lexState('SELECT 1 -- nota')).toBe('comment')
    expect(lexState('SELECT 1 -- nota\nSELECT ')).toBe('code')
    expect(lexState('SELECT /* abierto')).toBe('comment')
    expect(lexState('SELECT /* cerrado */ 1')).toBe('code')
    expect(lexState('SELECT "Col -- no es comentario')).toBe('code')
  })
})

describe('referencedTables', () => {
  const refs = (s: string, idx = pg) => referencedTables(s, idx).map((r) => `${r.alias}:${r.table.schema}.${r.table.name}`)
  it('con y sin AS', () => {
    expect(refs('SELECT * FROM clientes c')).toEqual([':public.clientes'.replace(':', 'c:')])
    expect(refs('SELECT * FROM clientes AS c JOIN pedidos p ON p.cliente_id = c.id')).toEqual(['c:public.clientes', 'p:public.pedidos'])
  })
  it('una palabra clave no es un alias', () => {
    expect(refs('SELECT * FROM clientes WHERE id = 1')).toEqual([':public.clientes'])
    expect(refs('SELECT * FROM clientes LEFT JOIN pedidos ON 1=1')).toEqual([':public.clientes', ':public.pedidos'])
    expect(refs('SELECT * FROM clientes ORDER BY id')).toEqual([':public.clientes'])
  })
  it('lista separada por comas', () => {
    expect(refs('SELECT * FROM clientes c, pedidos p, ventas.facturas f WHERE 1=1')).toEqual(['c:public.clientes', 'p:public.pedidos', 'f:ventas.facturas'])
  })
  it('esquema explícito y el esquema por defecto gana en homónimas', () => {
    expect(refs('FROM ventas.pedidos v')).toEqual(['v:ventas.pedidos'])
    expect(refs('FROM pedidos')).toEqual([':public.pedidos'])
  })
  it('INSERT / UPDATE y nombres citados', () => {
    expect(refs('INSERT INTO clientes (id, nombre) VALUES (1, 2)')).toEqual([':public.clientes'])
    expect(refs('UPDATE clientes SET nombre = 1')).toEqual([':public.clientes'])
    expect(refs('SELECT * FROM "clientes" "c"')).toEqual(['c:public.clientes'])
  })
  it('ignora tablas que no existen', () => {
    expect(refs('SELECT * FROM fantasma f')).toEqual([])
  })
})

describe('contexto', () => {
  it('tras FROM sugiere tablas y vistas, no columnas', () => {
    const l = labels('SELECT * FROM cl|')!
    expect(l).toContain('clientes')
    expect(l).toContain('v_activos')
    expect(l).not.toContain('nombre')
  })
  it('las tablas de otro esquema salen cualificadas, y los esquemas como tales', () => {
    const r = run('SELECT * FROM |', pg, 'postgres', true)!
    const f = r.options.find((o) => o.label === 'facturas')!
    expect(f.apply).toBe('ventas.facturas') // se filtra por "facturas" pero se inserta cualificada
    expect(f.detail).toBe('ventas · tabla')
    expect(r.options.map((o) => o.label)).toContain('ventas') // el esquema como tal
    const pedidos = r.options.filter((o) => o.label === 'pedidos')
    expect(pedidos.map((o) => o.apply).sort()).toEqual(['pedidos', 'ventas.pedidos']) // homónimas: la de public sin cualificar
  })
  it('tras JOIN, INTO y UPDATE también', () => {
    expect(labels('SELECT * FROM clientes JOIN pe|')).toContain('pedidos')
    expect(labels('INSERT INTO cl|')).toContain('clientes')
    expect(labels('UPDATE cl|')).toContain('clientes')
  })
  it('en la lista del FROM separada por comas sigue esperando tablas', () => {
    const l = labels('SELECT * FROM clientes c, ped|')!
    expect(l).toContain('pedidos')
    expect(l).not.toContain('cliente_id')
  })
  it('en WHERE / SELECT sugiere columnas de las tablas de la consulta', () => {
    const l = labels('SELECT * FROM clientes c WHERE na|')!
    expect(l).toContain('nombre')
    expect(l).not.toContain('pedidos')
    expect(labels('SELECT no| FROM clientes')).toContain('nombre')
  })
  it('con varias tablas indica de cuál es cada columna', () => {
    const r = run('SELECT i| FROM clientes c JOIN pedidos p ON 1=1')!
    const ids = r.options.filter((o) => o.label === 'id')
    expect(ids.map((o) => o.detail).sort()).toEqual(['integer · clientes', 'integer · pedidos'])
  })
  it('sin tablas aún ofrece tablas (con poca prioridad), no columnas', () => {
    const l = labels('SELECT cl|')!
    expect(l).toContain('clientes')
    expect(l).not.toContain('nombre')
  })
})

describe('alias y calificadores', () => {
  it('alias.columna', () => {
    expect(labels('SELECT c.| FROM clientes c')).toEqual(expect.arrayContaining(['id', 'nombre']))
    expect(labels('SELECT p.| FROM clientes c JOIN pedidos p ON 1=1')).toEqual(expect.arrayContaining(['cliente_id', 'total']))
    expect(labels('SELECT p.| FROM clientes c JOIN pedidos p ON 1=1')).not.toContain('nombre')
  })
  it('con parcial', () => {
    expect(labels('SELECT c.no| FROM clientes AS c')).toContain('nombre')
  })
  it('el alias no distingue mayúsculas', () => {
    expect(labels('SELECT C.| FROM clientes c')).toContain('nombre')
  })
  it('el alias se declara después del cursor (SELECT va antes que FROM)', () => {
    expect(labels('SELECT c.| FROM clientes c WHERE c.id > 1')).toContain('nombre')
  })
  it('tabla sin alias, y tabla que aún no está en el FROM', () => {
    expect(labels('SELECT clientes.| FROM clientes')).toContain('nombre')
    expect(labels('SELECT clientes.|')).toContain('nombre')
  })
  it('esquema. → sus tablas', () => {
    const l = labels('SELECT * FROM ventas.|')!
    expect(l).toEqual(expect.arrayContaining(['pedidos', 'facturas']))
    expect(l).not.toContain('clientes')
  })
  it('esquema.tabla. → columnas', () => {
    expect(labels('SELECT ventas.pedidos.| FROM ventas.pedidos')).toContain('monto')
  })
  it('un calificador desconocido no sugiere nada', () => {
    expect(labels('SELECT x.| FROM clientes c')).toBeNull()
  })
})

describe('límites: sentencias, cadenas, comentarios', () => {
  it('solo cuenta la sentencia del cursor', () => {
    const l = labels('SELECT 1 FROM clientes x; SELECT x.| FROM pedidos p')
    expect(l).toBeNull() // el alias x es de la sentencia anterior
    expect(labels('SELECT * FROM clientes x; SELECT p.| FROM pedidos p')).toContain('cliente_id')
  })
  it('dentro de una cadena o un comentario no sugiere', () => {
    expect(labels("SELECT * FROM clientes WHERE nombre = 'ab|")).toBeNull()
    expect(labels('SELECT * FROM clientes -- cl|')).toBeNull()
    expect(labels('SELECT /* cl| */ 1')).toBeNull()
  })
  it('sin nada escrito solo ofrece si se pide expresamente', () => {
    expect(labels('SELECT * FROM |')).toBeNull()
    expect(labels('SELECT * FROM |', pg, 'postgres', true)).not.toBeNull()
  })
  it('sin índice no hace nada', () => {
    expect(complete('SELECT ', 7, null, 'postgres', true)).toBeNull()
    expect(complete('SELECT ', 7, { tables: [], defaultSchema: '' }, 'postgres', true)).toBeNull()
  })
  it('from apunta al inicio de la palabra parcial', () => {
    expect(run('SELECT * FROM clien|')!.from).toBe('SELECT * FROM '.length)
    expect(run('SELECT c.no| FROM clientes c')!.from).toBe('SELECT c.'.length)
  })
})

describe('prioridad frente a las palabras clave', () => {
  it('lo relevante en cada contexto lleva más prioridad que las palabras clave del lenguaje (máx. 99)', () => {
    const boosts = (sql: string) => run(sql, pg, 'postgres', true)!.options.map((o) => o.boost)
    for (const sql of ['SELECT * FROM |', 'SELECT c.| FROM clientes c', 'SELECT | FROM clientes c WHERE 1=1']) {
      const b = boosts(sql)
      expect(Math.min(...b.filter((x) => x >= 40))).toBeGreaterThanOrEqual(40)
      expect(Math.max(...b)).toBeLessThanOrEqual(99)
    }
    // Las tablas del esquema por defecto antes que las de otros esquemas.
    const o = run('SELECT * FROM |', pg, 'postgres', true)!.options
    expect(o.find((x) => x.apply === 'pedidos')!.boost).toBeGreaterThan(o.find((x) => x.apply === 'ventas.pedidos')!.boost)
  })
})

describe('comillas según el motor', () => {
  const apply = (sql: string, label: string, idx = pg, driver = 'postgres') => run(sql, idx, driver)!.options.find((o) => o.label === label)?.apply
  it('PostgreSQL: cita mayúsculas, espacios y reservadas; lo demás va tal cual', () => {
    expect(apply('SELECT c.| FROM clientes c', 'Fecha Alta')).toBe('"Fecha Alta"')
    expect(apply('SELECT c.| FROM clientes c', 'order')).toBe('"order"')
    expect(apply('SELECT c.| FROM clientes c', 'nombre')).toBe('nombre')
  })
  it('MySQL con acentos graves y SQL Server con corchetes', () => {
    expect(apply('SELECT c.| FROM clientes c', 'Fecha Alta', my, 'mysql')).toBe('`Fecha Alta`')
    expect(apply('SELECT c.| FROM clientes c', 'Fecha Alta', my, 'mssql')).toBe('[Fecha Alta]')
    expect(apply('SELECT c.| FROM clientes c', 'id', my, 'mysql')).toBe('id')
  })
  it('las tablas cualificadas llevan el esquema', () => {
    expect(apply('SELECT * FROM fa|', 'facturas')).toBe('ventas.facturas')
  })
  it('si el usuario ya abrió comillas se completa dentro y se cierran', () => {
    const r = run('SELECT c."Fe| FROM clientes c')!
    expect(r.from).toBe('SELECT c."'.length) // justo después de la comilla
    expect(r.options.find((o) => o.label === 'Fecha Alta')?.apply).toBe('Fecha Alta"')
    const b = run('SELECT c.[Fe| FROM clientes c', my, 'mssql')!
    expect(b.options.find((o) => o.label === 'Fecha Alta')?.apply).toBe('Fecha Alta]')
  })
})

describe('sql.ts', () => {
  it('quoteIdent escapa las comillas', () => {
    expect(quoteIdent('postgres', 'a"b')).toBe('"a""b"')
    expect(quoteIdent('mysql', 'a`b')).toBe('`a``b`')
    expect(quoteIdent('mssql', 'a]b')).toBe('[a]]b]')
  })
  it('unquoteIdent es la inversa', () => {
    for (const [d, n] of [['postgres', 'a"b c'], ['mysql', 'a`b'], ['mssql', 'a]b']] as const) expect(unquoteIdent(quoteIdent(d, n))).toBe(n)
    expect(unquoteIdent('simple')).toBe('simple')
  })
  it('needsQuote', () => {
    expect(needsQuote('postgres', 'Clientes')).toBe(true) // PostgreSQL pliega a minúsculas
    expect(needsQuote('mysql', 'Clientes')).toBe(false)
    expect(needsQuote('postgres', 'select')).toBe(true)
    expect(needsQuote('sqlite', 'mi tabla')).toBe(true)
    expect(quoteIfNeeded('postgres', 'ok_1')).toBe('ok_1')
  })
})
