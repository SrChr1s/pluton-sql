<script lang="ts">
  import Icon from './Icon.svelte'
  import { childPath } from '../lib/json'
  import { copyText } from '../lib/clipboard'
  import { ui } from '../lib/ui.svelte'
  import JsonNode from './JsonNode.svelte'

  type Cmd = { n: number; open: boolean }
  let { k, v, path, depth = 0, cmd, filter = '' }: { k: string | number | null; v: unknown; path: string; depth?: number; cmd: Cmd; filter?: string } = $props()

  const CHUNK = 100
  const isObj = (x: unknown): x is object => x !== null && typeof x === 'object'
  const entries = $derived(isObj(v) ? (Array.isArray(v) ? v.map((x, i): [string | number, unknown] => [i, x]) : Object.entries(v)) : [])
  const count = $derived(entries.length)

  let open = $state(false)
  $effect.pre(() => { open = depth < 1 }) // la raíz y su primer nivel empiezan abiertos
  let shown = $state(CHUNK)
  let seen = 0
  $effect(() => { if (cmd.n !== seen) { seen = cmd.n; open = cmd.open } })
  // Con un filtro, se abre todo lo que contenga una coincidencia.
  const f = $derived(filter.trim().toLowerCase())
  const has = (x: unknown, key: string | number | null): boolean => {
    if (key !== null && String(key).toLowerCase().includes(f)) return true
    if (!isObj(x)) return String(x).toLowerCase().includes(f)
    return Object.entries(x).some(([kk, vv]) => has(vv, kk))
  }
  const hit = $derived(f === '' || has(v, k))
  const forced = $derived(f !== '' && isObj(v))
  const kind = $derived(v === null ? 'null' : typeof v)
  const label = $derived(k === null ? '' : typeof k === 'number' ? String(k) : JSON.stringify(k))
  const summary = $derived(Array.isArray(v) ? `[${count}]` : `{${count}}`)

  async function copyPath(e: MouseEvent) { e.stopPropagation(); await copyText(path); ui.toast(`Ruta copiada: ${path}`) }
</script>

{#if hit}
  {#if isObj(v)}
    <div class="node" style:--d={depth}>
      <button class="row" onclick={() => (open = !open)} aria-expanded={open || forced} title={path}>
        <span class="chev" class:open={open || forced}><Icon name="down" size={11} /></span>
        {#if k !== null}<span class="key" role="presentation" ondblclick={copyPath}>{label}</span><span class="colon">:</span>{/if}
        <span class="sum">{summary}</span>
      </button>
      {#if open || forced}
        <div class="kids">
          {#each entries.slice(0, shown) as [kk, vv] (kk)}
            <JsonNode k={kk} v={vv} path={childPath(path, kk)} depth={depth + 1} {cmd} {filter} />
          {/each}
          {#if entries.length > shown}
            <button class="more" onclick={() => (shown += CHUNK)}>Mostrar {Math.min(CHUNK, entries.length - shown)} más de {entries.length - shown}…</button>
          {/if}
        </div>
      {/if}
    </div>
  {:else}
    <div class="leaf" style:--d={depth} title={path}>
      <span class="chev"></span>
      {#if k !== null}<span class="key" role="presentation" ondblclick={copyPath}>{label}</span><span class="colon">:</span>{/if}
      <span class="val {kind}">{kind === 'string' ? JSON.stringify(v) : String(v)}</span>
    </div>
  {/if}
{/if}

<style>
  .node, .leaf { font: 12px/1.6 var(--mono); }
  .row { display: flex; align-items: center; gap: 4px; width: 100%; border: none; background: transparent; padding: 0 4px; border-radius: 3px; justify-content: flex-start; font: inherit; color: var(--text); }
  .row:hover, .leaf:hover { background: var(--hover); }
  .leaf { display: flex; align-items: baseline; gap: 4px; padding: 0 4px; border-radius: 3px; }
  .chev { display: inline-flex; width: 12px; flex: none; color: var(--muted); transform: rotate(-90deg); transition: transform .1s; }
  .chev.open { transform: none; }
  .leaf .chev { transform: none; }
  .kids { margin-left: 11px; padding-left: 6px; border-left: 1px solid color-mix(in srgb, var(--border) 70%, transparent); }
  .key { color: var(--accent); cursor: default; }
  .colon { color: var(--muted); }
  .sum { color: var(--muted); font-size: 11px; }
  .val { word-break: break-word; white-space: pre-wrap; }
  .val.string { color: var(--ok); }
  .val.number { color: var(--warn); }
  .val.boolean { color: var(--danger); }
  .val.null { color: var(--muted); font-style: italic; }
  .more { margin: 2px 0 2px 16px; font-size: 11.5px; color: var(--muted); border: none; background: transparent; padding: 0 4px; }
  .more:hover { color: var(--text); }
</style>
