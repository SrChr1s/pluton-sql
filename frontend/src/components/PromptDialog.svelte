<script lang="ts">
  import { ui } from '../lib/ui.svelte'
  let value = $state('')
  let input = $state<HTMLInputElement>()
  // Cada vez que se abre, parte del valor sugerido y lo deja seleccionado.
  $effect(() => {
    if (ui.prompting) { value = ui.prompting.value ?? ''; queueMicrotask(() => { input?.focus(); input?.select() }) }
  })
  const submit = () => { const v = value.trim(); if (v) ui.answerPrompt(v) }
</script>

{#if ui.prompting}
  <div class="overlay" role="presentation" onclick={() => ui.answerPrompt(null)}>
    <!-- svelte-ignore a11y_click_events_have_key_events -->
    <div class="modal" role="dialog" aria-modal="true" aria-labelledby="pd-title" tabindex="-1" onclick={(e) => e.stopPropagation()}
         onkeydown={(e) => e.key === 'Escape' && ui.answerPrompt(null)}>
      <h3 id="pd-title">{ui.prompting.title}</h3>
      <form onsubmit={(e) => { e.preventDefault(); submit() }}>
        <label>{ui.prompting.label}<input bind:this={input} bind:value /></label>
        <div class="actions">
          <button type="button" onclick={() => ui.answerPrompt(null)}>Cancelar</button>
          <button type="submit" class="primary" disabled={!value.trim()}>{ui.prompting.confirmLabel ?? 'Aceptar'}</button>
        </div>
      </form>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 30; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; padding: 18px 20px; width: 400px; box-shadow: var(--shadow); }
  form { display: flex; flex-direction: column; gap: 10px; }
  h3 { margin: 0 0 10px; font-size: 15px; }
  label { display: flex; flex-direction: column; gap: 4px; color: var(--muted); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px; }
</style>
