<script lang="ts">
  import Icon from './Icon.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { theme } from '../lib/theme.svelte'
  import { ui } from '../lib/ui.svelte'
  import { errMsg, type Workspace as WorkspaceInfo } from '../lib/api'
  import Menu, { type MenuItem } from './Menu.svelte'

  export type Panel = 'connections' | 'entities' | 'saved' | 'history'
  let { panel, onselect }: { panel: Panel; onselect: (p: Panel) => void } = $props()

  const items: { id: Panel; icon: string; label: string; needsConn: boolean }[] = [
    { id: 'connections', icon: 'server', label: 'Conexiones', needsConn: false },
    { id: 'entities', icon: 'database', label: 'Entidades', needsConn: true },
    { id: 'saved', icon: 'bookmark', label: 'Consultas guardadas', needsConn: true },
    { id: 'history', icon: 'clock', label: 'Historial (Ctrl+H)', needsConn: true },
  ]
  // Color estable por workspace, derivado de su id, para distinguirlos de un vistazo.
  const hue = (id: string) => [...id].reduce((h, c) => (h * 31 + c.charCodeAt(0)) % 360, 7)
  const initial = (name: string) => ([...name.trim()][0] ?? '?').toUpperCase()

  // Soltar una conexión sobre un workspace (o sobre "Todas": sin workspace) la mueve ahí.
  let over = $state<string | null>(null)
  const dropProps = (target: string) => ({
    ondragover: (e: DragEvent) => { if (ui.dragConn) { e.preventDefault(); if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'; over = target } },
    ondragleave: () => { if (over === target) over = null },
    ondrop: (e: DragEvent) => { e.preventDefault(); void drop(target) },
  })
  async function drop(target: string) {
    const id = ui.dragConn
    over = null; ui.dragConn = ''
    if (!id) return
    const p = ws.profiles.find((x) => x.id === id)
    if (!p || (p.workspace || '') === target) return
    try {
      await ws.moveProfile(id, target)
      ui.toast(`"${p.name}" ahora está ${target ? `en "${ws.workspaces.find((w) => w.id === target)?.name}"` : 'sin workspace'}`)
    } catch (e) { ui.toast(errMsg(e), 'err') }
  }

  let ctx = $state<{ x: number; y: number; w: WorkspaceInfo } | null>(null)
  // El menú se cierra antes de ejecutar la acción (y `ctx` pasa a null): se captura el workspace ahora.
  const ctxItems = $derived<MenuItem[]>(ctx ? ((w) => [
    { label: 'Renombrar…', icon: 'pencil', run: () => void rename(w) },
    { label: 'Eliminar workspace', icon: 'trash', run: () => void remove(w) },
  ])(ctx.w) : [])

  async function create() {
    const name = await ui.prompt({ title: 'Nuevo workspace', label: 'Nombre', confirmLabel: 'Crear' })
    if (!name?.trim()) return
    try { ws.selectWorkspace((await ws.createWorkspace(name)).id) } catch (e) { ui.toast(errMsg(e), 'err') }
  }

  async function rename(w: WorkspaceInfo) {
    const name = await ui.prompt({ title: 'Renombrar workspace', label: 'Nombre', value: w.name, confirmLabel: 'Renombrar' })
    if (!name?.trim() || name === w.name) return
    try { await ws.renameWorkspace(w, name) } catch (e) { ui.toast(errMsg(e), 'err') }
  }

  async function remove(w: WorkspaceInfo) {
    const ok = await ui.confirm({
      title: `Eliminar el workspace "${w.name}"`,
      message: 'Sus conexiones no se borran: pasan a "Sin workspace" y siguen visibles en "Todas las conexiones".',
      confirmLabel: 'Eliminar', danger: true,
    })
    if (!ok) return
    try { await ws.deleteWorkspace(w) } catch (e) { ui.toast(errMsg(e), 'err') }
  }

  const themeLabel = $derived(theme.mode === 'dark' ? 'Cambiar a tema claro' : 'Cambiar a tema oscuro')
</script>

<nav class="rail" aria-label="Paneles">
  {#each items as it (it.id)}
    <!-- Conectado: se ocultan las conexiones y los workspaces; para cambiar de conexión está el menú de la barra inferior. -->
    {#if it.needsConn ? ws.active : !ws.active}
      <button class="ghost icon" class:on={panel === it.id} title={it.label} aria-label={it.label} aria-pressed={panel === it.id} onclick={() => onselect(it.id)}>
        <Icon name={it.icon} size={19} />
      </button>
    {/if}
  {/each}
  {#if !ws.active}
  <div class="wss" role="group" aria-label="Workspaces">
    <button class="ghost icon all" class:on={!ws.workspaceId} title="Todas las conexiones" aria-label="Todas las conexiones" aria-pressed={!ws.workspaceId} class:drop={over === ''} onclick={() => ws.selectWorkspace('')} {...dropProps('')}>
      <Icon name="grid" size={17} />
    </button>
    {#each ws.workspaces as w (w.id)}
      <button class="ghost icon w" class:on={ws.workspaceId === w.id} class:drop={over === w.id} {...dropProps(w.id)} style:--h={hue(w.id)} title={w.name} aria-label="Workspace {w.name}" aria-pressed={ws.workspaceId === w.id}
              onclick={() => ws.selectWorkspace(w.id)} oncontextmenu={(e) => { e.preventDefault(); ctx = { x: e.clientX, y: e.clientY, w } }}>
        {initial(w.name)}
      </button>
    {/each}
    <button class="ghost icon add" title="Nuevo workspace" aria-label="Nuevo workspace" onclick={create}><Icon name="plus" size={16} /></button>
  </div>
  {/if}
  <span class="grow"></span>
  {#if ctx}<Menu at={{ x: ctx.x, y: ctx.y }} items={ctxItems} onclose={() => (ctx = null)} />{/if}
  <button class="ghost icon" title={themeLabel} aria-label={themeLabel} onclick={() => theme.toggle()}>
    <Icon name={theme.mode === 'dark' ? 'sun' : 'moon'} size={18} />
  </button>
</nav>

<style>
  .rail { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 10px 0; height: 100%; box-sizing: border-box; background: var(--surface); }
  .rail button { width: 36px; height: 36px; padding: 0; display: grid; place-items: center; color: var(--muted); border-radius: 8px; }
  .rail button:hover { color: var(--text); }
  .rail button { position: relative; }
  .rail button.on { background: var(--accent-soft); color: var(--accent); }
  .rail > button.on::before { content: ''; position: absolute; left: -6px; top: 8px; bottom: 8px; width: 3px; border-radius: 0 3px 3px 0; background: var(--accent); }
  .grow { flex: 1; }
  .wss { display: flex; flex-direction: column; align-items: center; gap: 6px; padding: 8px 0; border-top: 1px solid var(--border); width: 100%; max-height: 50%; overflow-y: auto; scrollbar-width: none; }
  .rail .wss button { width: 32px; height: 32px; }
  .rail .wss .w { font: 700 13px var(--sans); color: hsl(var(--h) 70% 72%); background: hsl(var(--h) 45% 24% / .55); border-radius: 9px; }
  :global(:root[data-theme="light"]) .rail .wss .w { color: hsl(var(--h) 60% 30%); background: hsl(var(--h) 70% 85%); }
  .rail .wss .w.on { box-shadow: 0 0 0 2px var(--bg), 0 0 0 4px hsl(var(--h) 70% 60%); }
  .rail .wss .all.on { background: var(--accent-soft); color: var(--accent); }
  .rail .wss button.drop { outline: 2px dashed var(--accent); outline-offset: 2px; background: var(--accent-soft); }
  .rail .wss .add { border: 1px dashed var(--border); }
</style>
