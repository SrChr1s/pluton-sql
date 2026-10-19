<script lang="ts">
  import Icon from './Icon.svelte'
  import Logo from './Logo.svelte'
  import { WindowMinimise, WindowToggleMaximise, Quit } from '../../wailsjs/runtime/runtime'
</script>

<!-- Solo se muestra donde la ventana no tiene barra propia (Linux). Se arrastra desde cualquier parte vacía. -->
<header ondblclick={() => WindowToggleMaximise()}>
  <span class="title"><Logo size={16} orbit={false} /> Pluton SQL</span>
  <span class="btns">
    <button class="ghost" title="Minimizar" aria-label="Minimizar" ondblclick={(e) => e.stopPropagation()} onclick={() => WindowMinimise()}><Icon name="minus" size={14} /></button>
    <button class="ghost" title="Maximizar o restaurar" aria-label="Maximizar o restaurar" ondblclick={(e) => e.stopPropagation()} onclick={() => WindowToggleMaximise()}><Icon name="maximize" size={12} /></button>
    <button class="ghost close" title="Cerrar" aria-label="Cerrar" ondblclick={(e) => e.stopPropagation()} onclick={() => Quit()}><Icon name="x" size={15} /></button>
  </span>
</header>

<style>
  header { --wails-draggable: drag; display: flex; align-items: center; justify-content: center; height: 100%; background: var(--surface); color: var(--muted); font-size: 12px; user-select: none; position: relative; }
  .title { display: inline-flex; align-items: center; gap: 7px; }
  .btns { --wails-draggable: no-drag; position: absolute; right: 0; top: 0; bottom: 0; display: flex; }
  .btns button { width: 44px; height: 100%; border-radius: 0; padding: 0; display: grid; place-items: center; color: var(--muted); }
  .btns button:hover { color: var(--text); }
  .btns button.close:hover { background: #e5484d; color: #fff; }
</style>
