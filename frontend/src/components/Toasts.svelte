<script lang="ts">
  import { ui } from '../lib/ui.svelte'
</script>

<div class="toasts" aria-live="polite">
  {#each ui.toasts as t (t.id)}
    <button class="toast" class:err={t.kind === 'err'} role={t.kind === 'err' ? 'alert' : 'status'} title="Cerrar" onclick={() => ui.dismissToast(t.id)}>{t.msg}</button>
  {/each}
</div>

<style>
  /* Abajo a la izquierda: a la derecha taparían el pie del panel de historial y los botones de la barra de herramientas. */
  .toasts { position: fixed; left: 14px; bottom: 38px; display: flex; flex-direction: column; align-items: flex-start; gap: 8px; z-index: 40; max-width: min(380px, 90vw); pointer-events: none; }
  .toast { pointer-events: auto; text-align: left; display: block; background: var(--surface-2); border: 1px solid var(--border); border-left: 3px solid var(--ok); border-radius: var(--radius); padding: 8px 12px; box-shadow: var(--shadow); word-break: break-word; font: inherit; color: var(--text); }
  .toast.err { border-left-color: var(--danger); color: var(--danger); }
</style>
