<script lang="ts">
  import { ui } from '../lib/ui.svelte'
  let confirmBtn = $state<HTMLButtonElement>()
  $effect(() => { if (ui.dialog) confirmBtn?.focus() })
</script>

{#if ui.dialog}
  <div class="overlay" role="presentation" onclick={() => ui.answer(false)}>
    <div class="modal" role="alertdialog" aria-modal="true" aria-labelledby="cd-title" tabindex="-1"
         onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && ui.answer(false)}>
      <h3 id="cd-title">{ui.dialog.title}</h3>
      {#if ui.dialog.message}<p>{ui.dialog.message}</p>{/if}
      <div class="actions">
        <button onclick={() => ui.answer(false)}>Cancelar</button>
        <button bind:this={confirmBtn} class={ui.dialog.danger ? 'primary danger-fill' : 'primary'} onclick={() => ui.answer(true)}>
          {ui.dialog.confirmLabel ?? 'Aceptar'}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay { position: fixed; inset: 0; background: #0008; display: grid; place-items: center; z-index: 30; }
  .modal { background: var(--surface); border: 1px solid var(--border); border-radius: 10px; padding: 18px 20px; width: 380px; box-shadow: var(--shadow); }
  h3 { margin: 0 0 6px; font-size: 15px; }
  p { margin: 0 0 14px; color: var(--muted); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 14px; }
  .danger-fill { background: var(--danger); border-color: var(--danger); color: #fff; }
</style>
