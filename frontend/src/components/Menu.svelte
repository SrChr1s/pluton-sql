<script lang="ts">
  import Icon from './Icon.svelte'

  export type MenuItem = {
    label: string
    icon?: string
    run?: () => void
    /** Submenú: se muestra al pasar el ratón o con clic. */
    sub?: MenuItem[]
    checked?: boolean
    color?: string
    disabled?: boolean
    /** Línea separadora (el resto de campos se ignora). */
    sep?: boolean
  }

  let { items, onclose, up = false, right = false, at }: { items: MenuItem[]; onclose: () => void; up?: boolean; right?: boolean; at?: { x: number; y: number } } = $props()
  let el: HTMLDivElement
  let openSub = $state(-1)

  function pick(it: MenuItem) {
    if (it.disabled) return
    if (it.sub) { openSub = openSub === items.indexOf(it) ? -1 : items.indexOf(it); return }
    onclose()
    it.run?.()
  }
</script>

<!-- El botón que abre el menú debe parar el pointerdown para no cerrarlo y reabrirlo al hacer clic. -->
<svelte:window onpointerdown={(e) => { if (!el?.contains(e.target as Node)) onclose() }} onblur={onclose} onkeydown={(e) => e.key === 'Escape' && onclose()} />

<div class="menu" class:up class:right class:fixed={!!at} role="menu" bind:this={el}
     style:left={at ? `${Math.min(at.x, innerWidth - 240)}px` : undefined} style:top={at ? `${Math.min(at.y, innerHeight - 40 - items.length * 36)}px` : undefined}>
  {#each items as it, i}
    {#if it.sep}<hr />{:else}
    <div class="wrap" onmouseenter={() => it.sub && (openSub = i)} onmouseleave={() => it.sub && openSub === i && (openSub = -1)} role="presentation">
      <button role="menuitem" disabled={it.disabled} aria-haspopup={it.sub ? 'menu' : undefined} aria-expanded={it.sub ? openSub === i : undefined} onclick={() => pick(it)}>
        <span class="ic">{#if it.color}<span class="dot" style:background={it.color}></span>{:else if it.icon}<Icon name={it.icon} size={15} />{/if}</span>
        <span class="lbl">{it.label}</span>
        {#if it.checked}<Icon name="check" size={13} />{/if}
        {#if it.sub}<Icon name="right" size={13} />{/if}
      </button>
      {#if it.sub && openSub === i}
        <div class="menu sub" role="menu">
          {#each it.sub as s}
            <button role="menuitem" disabled={s.disabled} onclick={() => pick(s)}>
              <span class="ic">{#if s.color}<span class="dot" style:background={s.color}></span>{/if}</span>
              <span class="lbl">{s.label}</span>
              {#if s.checked}<Icon name="check" size={13} />{/if}
            </button>
          {/each}
        </div>
      {/if}
    </div>
    {/if}
  {/each}
</div>

<style>
  .menu { position: absolute; z-index: 30; min-width: 220px; padding: 6px; background: var(--surface-2); border: 1px solid var(--border); border-radius: 8px; box-shadow: var(--shadow); left: 0; top: calc(100% + 4px); }
  .menu.fixed { position: fixed; }
  .menu.up { top: auto; bottom: calc(100% + 6px); }
  .menu.right { left: auto; right: 0; }
  .wrap { position: relative; }
  hr { border: none; border-top: 1px solid var(--border); margin: 5px 4px; }
  .menu button { display: flex; align-items: center; gap: 10px; width: 100%; padding: 8px 10px; border: none; background: transparent; border-radius: 5px; text-align: left; color: var(--text); }
  .menu button:hover:not(:disabled) { background: var(--hover); }
  .ic { width: 16px; display: inline-grid; place-items: center; color: var(--muted); }
  .dot { width: 10px; height: 10px; border-radius: 50%; }
  .lbl { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .sub { left: calc(100% - 2px); top: -6px; bottom: auto; min-width: 200px; max-height: 60vh; overflow: auto; }
  .menu.up .sub { top: auto; bottom: -6px; }
</style>
