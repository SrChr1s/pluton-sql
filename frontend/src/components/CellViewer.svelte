<script lang="ts">
  import Icon from './Icon.svelte'
  import { api, errMsg, type CellPreview } from '../lib/api'
  import { ui, type CellView } from '../lib/ui.svelte'

  let { view, onclose }: { view: CellView; onclose: () => void } = $props()

  let data = $state<CellPreview | null>(null)
  let error = $state('')
  let saving = $state(false)

  const req = () => ({ table: view.loc, column: view.column, key: view.key }) as Parameters<typeof api.cellPreview>[0]
  const size = (n: number) => (n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : `${(n / (1 << 20)).toFixed(1)} MB`)

  $effect(() => {
    api.cellPreview(req()).then((d) => (data = d), (e) => (error = errMsg(e)))
  })

  async function save() {
    saving = true
    try {
      const path = await api.cellSave(req(), `${view.loc.table}_${view.column}`)
      if (path) ui.toast(`Guardado en ${path}`)
    } catch (e) {
      error = errMsg(e)
    }
    saving = false
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="overlay" role="presentation" onclick={onclose}>
  <!-- svelte-ignore a11y_click_events_have_key_events -->
  <div class="modal" role="dialog" aria-modal="true" aria-labelledby="cv-title" tabindex="-1" onclick={(e) => e.stopPropagation()}>
    <h3 id="cv-title">{view.title}</h3>
    {#if error}
      <p class="err" role="alert">{error}</p>
    {:else if !data}
      <p class="note"><span class="orbit"></span> Leyendo el contenido…</p>
    {:else}
      <p class="note">
        {data.kind === 'image' ? 'Imagen' : data.kind === 'text' ? 'Texto' : 'Binario'} · {data.mime} · {size(data.size)}
        {#if data.offset > 0}· cabecera de {data.offset} B omitida al guardar{/if}
      </p>
      {#if data.kind === 'image'}
        {#if data.image}
          <div class="img"><img src={data.image} alt={view.title} /></div>
        {:else}
          <p class="note">La imagen es demasiado grande para mostrarla; se puede guardar como archivo.</p>
        {/if}
      {:else if data.kind === 'text'}
        <pre class="dump">{data.text}{data.size > data.text.length ? '\n…' : ''}</pre>
      {:else}
        <pre class="dump">{data.hex}{data.size > 512 ? '…' : ''}</pre>
      {/if}
    {/if}
    <div class="actions">
      <button type="button" onclick={onclose}>Cerrar</button>
      <button type="button" class="primary" onclick={save} disabled={!data || saving}><Icon name="download" size={14} /> Guardar como…</button>
    </div>
  </div>
</div>

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 20; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; box-shadow: var(--shadow); padding: 18px 20px; width: 640px; max-width: 94vw; max-height: 92vh; overflow: auto; }
  h3 { margin: 0 0 6px; font-size: 15px; }
  .note { margin: 0 0 10px; color: var(--muted); display: flex; align-items: center; gap: 8px; }
  .err { margin: 0; color: var(--danger); white-space: pre-wrap; word-break: break-word; }
  .img { display: grid; place-items: center; padding: 10px; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); max-height: 60vh; overflow: auto; }
  .img img { max-width: 100%; max-height: 56vh; object-fit: contain; }
  .dump { margin: 0; padding: 8px 10px; background: var(--bg); border: 1px solid var(--border); border-radius: var(--radius); font: 11.5px/1.5 var(--mono); white-space: pre-wrap; word-break: break-all; max-height: 50vh; overflow: auto; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; }
</style>
