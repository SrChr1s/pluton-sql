<script lang="ts">
  import Icon from './Icon.svelte'
  import { RowWindow, columnWidths } from '../lib/virtual.svelte'
  import { parseJson, rowToJson } from '../lib/json'
  import { ui } from '../lib/ui.svelte'
  import type { QueryResult } from '../lib/api'
  let { result, error, running }: { result: QueryResult | null; error: string; running: boolean } = $props()

  const win = new RowWindow()
  win.rowH = 21
  const rng = $derived(win.range(result?.rows.length ?? 0))
  $effect(() => { result; win.reset() })
  const widths = $derived(result?.isQuery ? columnWidths(result.columns, result.rows) : [])
  // Doble clic: en el número de fila abre la fila completa como JSON; en una celda con JSON, esa celda.
  function onDbl(e: MouseEvent) {
    const td = (e.target as Element).closest('td') as HTMLTableCellElement | null
    const tr = td?.parentElement as HTMLElement | null
    if (!td || !tr || !result) return
    const r = Number(tr.dataset.row)
    if (td.classList.contains('n')) { ui.json = { title: `Fila ${r + 1}`, text: rowToJson(result.columns, result.rows[r]) }; return }
    const v = result.rows[r]?.[td.cellIndex - 1]
    if (typeof v === 'string' && parseJson(v) !== undefined) ui.json = { title: `${result.columns[td.cellIndex - 1]} · fila ${r + 1}`, text: v }
  }
  const show = (v: unknown) => (v === null || v === undefined ? null : String(v))
  const isNum = (v: unknown) => typeof v === 'number'
  const shortcuts: [string, string[][]][] = [
    ['Ejecutar', [['Ctrl', 'Enter'], ['F5']]],
    ['Autocompletar', [['Ctrl', 'Espacio']]],
    ['Nueva pestaña', [['Ctrl', 'T']]],
    ['Cerrar pestaña', [['Ctrl', 'W']]],
    ['Abrir script', [['Ctrl', 'O']]],
    ['Guardar script', [['Ctrl', 'S']]],
    ['Historial', [['Ctrl', 'H']]],
    ['Panel lateral', [['Ctrl', 'B']]],
  ]
</script>

<div class="wrap">
  {#if running}
    <div class="msg"><span class="orbit"></span> Ejecutando…</div>
  {:else if error}
    <div class="msg err" role="alert"><pre>{error}</pre></div>
  {:else if !result}
    <div class="keys">
      {#each shortcuts as [label, combos]}
        <div class="krow"><span class="klabel">{label}</span>
          <span class="kcombos">{#each combos as combo}<span class="kcombo">{#each combo as k}<kbd>{k}</kbd>{/each}</span>{/each}</span>
        </div>
      {/each}
    </div>
  {:else if !result.isQuery}
    <div class="msg ok">Consulta ejecutada — {result.rowsAffected} fila(s) afectada(s) en {result.durationMs} ms</div>
  {:else if result.rows.length === 0}
    <div class="msg hint"><Icon name="orbit" size={20} /><span>Espacio vacío: la consulta no devolvió filas ({result.columns.length} columna(s)).</span></div>
  {:else}
    <div class="scroll" use:win.attach>
      <table style:width="{widths.reduce((a, b) => a + b, 72)}px">
        <colgroup><col style:width="72px" />{#each widths as w}<col style:width="{w}px" />{/each}</colgroup>
        <thead><tr><th class="n">#</th>{#each result.columns as c}<th title={c}>{c}</th>{/each}</tr></thead>
        <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
        <tbody ondblclick={onDbl} title="Doble clic en el número de fila para ver la fila como JSON">
          {#if rng.before > 0}<tr class="pad" aria-hidden="true" style:height="{rng.before}px"><td colspan={result.columns.length + 1}></td></tr>{/if}
          {#each { length: rng.end - rng.start } as _, k (rng.start + k)}
            {@const i = rng.start + k}
            {@const row = result.rows[i]}
            <tr data-row={i}><td class="n">{i + 1}</td>
              {#each row as v}
                {@const s = show(v)}
                <td class:null={s === null} class:num={isNum(v)} title={s ?? 'NULL'}>{s ?? 'NULL'}</td>
              {/each}
            </tr>
          {/each}
          {#if rng.after > 0}<tr class="pad" aria-hidden="true" style:height="{rng.after}px"><td colspan={result.columns.length + 1}></td></tr>{/if}
        </tbody>
      </table>
    </div>
    {#if result.truncated}<div class="foot">Resultado truncado a {result.rows.length} filas.</div>{/if}
  {/if}
</div>

<style>
  .wrap { display: flex; flex-direction: column; height: 100%; min-height: 0; background: var(--bg); }
  .keys { margin: auto; display: flex; flex-direction: column; gap: 7px; padding: 16px; }
  .krow { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; align-items: center; min-width: 340px; }
  .klabel { text-align: right; color: var(--muted); }
  .kcombos { display: flex; flex-direction: column; gap: 4px; }
  .kcombo { display: flex; gap: 4px; }
  .scroll { flex: 1; overflow: auto; min-height: 0; }
  table { border-collapse: separate; border-spacing: 0; font: 12px var(--mono); min-width: 100%; table-layout: fixed; }
  th { position: sticky; top: 0; z-index: 2; background: var(--surface); text-align: left; font: 600 11.5px var(--sans); color: var(--muted); border-bottom-color: var(--muted); }
  th, td { padding: 2px 10px; border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent); border-right: 1px solid color-mix(in srgb, var(--border) 60%, transparent); white-space: nowrap; max-width: 420px; overflow: hidden; text-overflow: ellipsis; }
  tr.pad td { padding: 0; border: none; }
  tbody tr:hover td { background: var(--accent-soft); }
  .n { position: sticky; left: 0; z-index: 1; background: var(--surface); color: var(--muted); text-align: right; width: 1%; font-family: var(--mono); }
  th.n { z-index: 3; }
  .num { text-align: right; }
  .null { color: var(--muted); font-style: italic; opacity: .7; }
  .foot { padding: 4px 12px; background: var(--surface-2); border-top: 1px solid var(--border); color: var(--warn); font-size: 12px; }
  .msg { display: flex; align-items: center; justify-content: center; gap: 10px; height: 100%; padding: 16px; color: var(--muted); text-align: center; }
  .msg.hint { flex-direction: column; gap: 8px; }
  .msg.ok { color: var(--ok); }
  .msg.err { justify-content: flex-start; align-items: flex-start; color: var(--danger); overflow: auto; }
  pre { margin: 0; white-space: pre-wrap; font: 12.5px var(--mono); }
</style>
