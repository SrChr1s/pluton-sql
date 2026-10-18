<script lang="ts">
  import Icon from './Icon.svelte'
  import type { QueryResult } from '../lib/api'
  let { result, error, running }: { result: QueryResult | null; error: string; running: boolean } = $props()

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
    <div class="msg"><span class="spin"></span> Ejecutando…</div>
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
    <div class="msg hint">La consulta no devolvió filas ({result.columns.length} columna(s)).</div>
  {:else}
    <div class="scroll">
      <table>
        <thead><tr><th class="n">#</th>{#each result.columns as c}<th title={c}>{c}</th>{/each}</tr></thead>
        <tbody>
          {#each result.rows as row, i}
            <tr><td class="n">{i + 1}</td>
              {#each row as v}
                {@const s = show(v)}
                <td class:null={s === null} class:num={isNum(v)} title={s ?? 'NULL'}>{s ?? 'NULL'}</td>
              {/each}
            </tr>
          {/each}
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
  table { border-collapse: separate; border-spacing: 0; font: 12px var(--mono); min-width: 100%; }
  th { position: sticky; top: 0; z-index: 2; background: var(--surface); text-align: left; font: 600 11.5px var(--sans); color: var(--muted); border-bottom-color: var(--muted); }
  th, td { padding: 2px 10px; border-bottom: 1px solid color-mix(in srgb, var(--border) 60%, transparent); border-right: 1px solid color-mix(in srgb, var(--border) 60%, transparent); white-space: nowrap; max-width: 420px; overflow: hidden; text-overflow: ellipsis; }
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
  .spin { width: 14px; height: 14px; border: 2px solid var(--border); border-top-color: var(--accent); border-radius: 50%; animation: spin .7s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
