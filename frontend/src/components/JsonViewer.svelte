<script lang="ts">
  import Icon from './Icon.svelte'
  import JsonNode from './JsonNode.svelte'
  import { parseJson } from '../lib/json'
  import { copyText } from '../lib/clipboard'
  import { ui, type JsonView } from '../lib/ui.svelte'

  let { view, onclose }: { view: JsonView; onclose: () => void } = $props()

  const value = $derived(parseJson(view.text))
  const pretty = $derived(value === undefined ? view.text : JSON.stringify(value, null, 2))
  const compact = $derived(value === undefined ? view.text : JSON.stringify(value))
  let mode = $state<'tree' | 'text'>('tree')
  let filter = $state('')
  let cmd = $state({ n: 0, open: true })
  const bytes = $derived(new Blob([view.text]).size)
  const size = (n: number) => (n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)

  async function copy(text: string, what: string) { await copyText(text); ui.toast(`${what} copiado`) }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="overlay" role="presentation" onclick={onclose}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="jv-title" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <h3 id="jv-title">{view.title}</h3>
    {#if value === undefined}
      <p class="err" role="alert">El contenido no es un JSON válido.</p>
      <pre class="dump">{view.text}</pre>
    {:else}
      <div class="bar">
        <div class="seg" role="tablist" aria-label="Modo de vista">
          <button role="tab" class:on={mode === 'tree'} aria-selected={mode === 'tree'} onclick={() => (mode = 'tree')}>Árbol</button>
          <button role="tab" class:on={mode === 'text'} aria-selected={mode === 'text'} onclick={() => (mode = 'text')}>Texto</button>
        </div>
        {#if mode === 'tree'}
          <input class="q" placeholder="Buscar clave o valor…" aria-label="Buscar en el JSON" bind:value={filter} />
          <button class="ghost icon" title="Expandir todo" aria-label="Expandir todo" onclick={() => (cmd = { n: cmd.n + 1, open: true })}><Icon name="layers" size={14} /></button>
          <button class="ghost icon" title="Contraer todo" aria-label="Contraer todo" onclick={() => (cmd = { n: cmd.n + 1, open: false })}><Icon name="up" size={14} /></button>
        {/if}
        <span class="grow"></span>
        <span class="note">{size(bytes)}</span>
      </div>
      {#if mode === 'tree'}
        <div class="tree"><JsonNode k={null} v={value} path="$" {cmd} {filter} /></div>
        <p class="tip">Doble clic en una clave copia su ruta.</p>
      {:else}
        <pre class="dump">{pretty}</pre>
      {/if}
    {/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Cerrar</button>
      {#if value !== undefined}
        <button type="button" onclick={() => copy(compact, 'JSON minificado')}><Icon name="copy" size={14} /> Copiar minificado</button>
        <button type="button" class="primary" onclick={() => copy(pretty, 'JSON')}><Icon name="copy" size={14} /> Copiar JSON</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 20; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow); padding: 18px 20px; width: 760px; max-width: 94vw; max-height: 92vh; display: flex; flex-direction: column; min-height: 0; }
  h3 { margin: 0 0 8px; font-size: 15px; }
  .err { margin: 0 0 8px; color: var(--danger); }
  .bar { display: flex; align-items: center; gap: 6px; margin-bottom: 8px; }
  .seg { display: flex; border: 1px solid var(--border); border-radius: var(--radius); overflow: hidden; }
  .seg button { border: none; border-radius: 0; background: transparent; padding: 3px 12px; color: var(--muted); }
  .seg button.on { background: var(--accent-soft); color: var(--text); }
  .q { flex: 0 1 240px; }
  .grow { flex: 1; }
  .note { color: var(--muted); font-size: 12px; }
  .tree, .dump { flex: 1; min-height: 120px; max-height: 62vh; overflow: auto; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); padding: 8px 10px; }
  .dump { margin: 0; font: 12px/1.55 var(--mono); white-space: pre-wrap; word-break: break-word; }
  .tip { margin: 6px 0 0; font-size: 11.5px; color: var(--muted); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
</style>
