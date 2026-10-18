<script lang="ts">
  import Icon from './Icon.svelte'
  import type { QueryResult } from '../lib/api'
  let { result, error, running }: { result: QueryResult | null; error: string; running: boolean } = $props()

  const show = (v: unknown) => (v === null || v === undefined ? null : String(v))
  const isNum = (v: unknown) => typeof v === 'number'
</script>

<div class="wrap">
  {#if running}
    <div class="msg"><span class="spin"></span> Ejecutando…</div>
  {:else if error}
    <div class="msg err" role="alert"><pre>{error}</pre></div>
  {:else if !result}
    <div class="msg hint"><Icon name="sql" size={22} /><span>Escribe una consulta y pulsa <kbd>Ctrl</kbd> <kbd>Enter</kbd></span></div>
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
  .wrap { display: flex; flex-direction: column; height: 100%; min-height: 0; background: var(--surface); }
  .scroll { flex: 1; overflow: auto; min-height: 0; }
  table { border-collapse: separate; border-spacing: 0; font: 12px var(--mono); min-width: 100%; }
  th { position: sticky; top: 0; z-index: 2; background: var(--surface-2); text-align: left; font: 600 12px var(--sans); color: var(--muted); }
  th, td { padding: 4px 12px; border-bottom: 1px solid var(--border); border-right: 1px solid var(--border); white-space: nowrap; max-width: 420px; overflow: hidden; text-overflow: ellipsis; }
  tbody tr:nth-child(even) td:not(.n) { background: color-mix(in srgb, var(--surface-2) 40%, transparent); }
  tbody tr:hover td { background: var(--accent-soft); }
  .n { position: sticky; left: 0; z-index: 1; background: var(--surface-2); color: var(--muted); text-align: right; width: 1%; font-family: var(--mono); }
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
