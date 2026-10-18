<script lang="ts">
  import type { QueryResult } from '../lib/api'
  let { result, error, running }: { result: QueryResult | null; error: string; running: boolean } = $props()

  const show = (v: unknown) => (v === null || v === undefined ? null : String(v))
</script>

<div class="wrap">
  {#if running}
    <p class="msg">Ejecutando…</p>
  {:else if error}
    <pre class="msg err">{error}</pre>
  {:else if !result}
    <p class="msg">Escribe una consulta y pulsa Ctrl+Enter.</p>
  {:else if !result.isQuery}
    <p class="msg ok">OK — {result.rowsAffected} fila(s) afectada(s) · {result.durationMs} ms</p>
  {:else}
    <div class="scroll">
      <table>
        <thead><tr><th class="n">#</th>{#each result.columns as c}<th>{c}</th>{/each}</tr></thead>
        <tbody>
          {#each result.rows as row, i}
            <tr><td class="n">{i + 1}</td>
              {#each row as v}
                {@const s = show(v)}
                <td class:null={s === null} title={s ?? 'NULL'}>{s ?? 'NULL'}</td>
              {/each}
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
    <div class="foot">
      {result.rows.length} fila(s){result.truncated ? ' (truncado, límite 1000)' : ''} · {result.durationMs} ms
    </div>
  {/if}
</div>

<style>
  .wrap { display: flex; flex-direction: column; height: 100%; min-height: 0; }
  .scroll { flex: 1; overflow: auto; min-height: 0; }
  table { border-collapse: collapse; font-family: var(--mono); font-size: 12px; }
  th { position: sticky; top: 0; background: var(--panel2); text-align: left; z-index: 1; }
  th, td { padding: 3px 10px; border: 1px solid var(--border); white-space: nowrap; max-width: 420px; overflow: hidden; text-overflow: ellipsis; }
  tr:hover td { background: #ffffff08; }
  .n { color: var(--muted); text-align: right; background: var(--panel); position: sticky; left: 0; }
  .null { color: var(--muted); font-style: italic; }
  .foot { padding: 4px 10px; background: var(--panel); border-top: 1px solid var(--border); color: var(--muted); }
  .msg { padding: 12px 14px; margin: 0; color: var(--muted); }
  .msg.err { color: var(--danger); white-space: pre-wrap; font-family: var(--mono); }
  .msg.ok { color: var(--ok); }
</style>
