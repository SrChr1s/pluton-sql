<script lang="ts">
  import Icon from './components/Icon.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import TabBar from './components/TabBar.svelte'
  import StatusBar from './components/StatusBar.svelte'
  import QueryEditor from './components/QueryEditor.svelte'
  import ResultGrid from './components/ResultGrid.svelte'
  import TableView from './components/TableView.svelte'
  import ConnectionForm from './components/ConnectionForm.svelte'
  import ConfirmDialog from './components/ConfirmDialog.svelte'
  import SchemaChangeDialog from './components/SchemaChangeDialog.svelte'
  import { ui } from './lib/ui.svelte'
  import { ws } from './lib/workspace.svelte'
  import type { Profile } from './lib/api'

  const LAYOUT = 'pluton.layout'
  const saved = (() => { try { return JSON.parse(localStorage.getItem(LAYOUT) ?? '{}') } catch { return {} } })()

  let form = $state<{ profile: Profile | null } | null>(null)
  let sidebarW = $state<number>(saved.sidebarW ?? 280)
  let sidebarOpen = $state<boolean>(saved.sidebarOpen ?? true)
  let editorPct = $state<number>(saved.editorPct ?? 42)

  ws.loadProfiles()

  // Guarda borradores de las pestañas y la disposición con un pequeño retardo.
  $effect(() => {
    void ws.signature
    const h = setTimeout(() => ws.persist(), 400)
    return () => clearTimeout(h)
  })
  $effect(() => {
    const layout = JSON.stringify({ sidebarW, sidebarOpen, editorPct })
    const h = setTimeout(() => { try { localStorage.setItem(LAYOUT, layout) } catch { /* no crítico */ } }, 300)
    return () => clearTimeout(h)
  })

  function onkeydown(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey
    const k = e.key.toLowerCase()
    if (form) return
    if (mod && k === 't') { e.preventDefault(); ws.newTab() }
    else if (mod && k === 'w') { e.preventDefault(); if (ws.tab) void ws.close(ws.tab.id) }
    else if (mod && k === 'b') { e.preventDefault(); sidebarOpen = !sidebarOpen }
    else if (mod && (e.key === 'PageDown' || (e.key === 'Tab' && !e.shiftKey))) { e.preventDefault(); ws.step(1) }
    else if (mod && (e.key === 'PageUp' || (e.key === 'Tab' && e.shiftKey))) { e.preventDefault(); ws.step(-1) }
    else if (e.altKey && /^[1-9]$/.test(e.key)) { e.preventDefault(); ws.selectNth(Number(e.key) - 1) }
    else if (e.key === 'F5') { e.preventDefault(); if (ws.dataTab) ws.dataTab.state.reload(); else ws.run() }
    else if (mod && k === 's') { e.preventDefault(); ws.dataTab?.state.apply() }
    else if (mod && e.key === '.') { e.preventDefault(); ws.cancel() }
  }

  function drag(e: PointerEvent, axis: 'x' | 'y') {
    e.preventDefault()
    const target = e.currentTarget as HTMLElement
    const area = (axis === 'x' ? target.parentElement : target.parentElement!)!.getBoundingClientRect()
    const move = (ev: PointerEvent) => {
      if (axis === 'x') sidebarW = Math.round(Math.min(520, Math.max(200, ev.clientX - area.left)))
      else editorPct = Math.min(80, Math.max(15, ((ev.clientY - area.top) / area.height) * 100))
    }
    const up = () => { removeEventListener('pointermove', move); removeEventListener('pointerup', up); document.body.style.cursor = '' }
    document.body.style.cursor = axis === 'x' ? 'col-resize' : 'row-resize'
    addEventListener('pointermove', move)
    addEventListener('pointerup', up)
  }
</script>

<svelte:window {onkeydown} />

<div class="app" style:grid-template-columns={sidebarOpen ? `${sidebarW}px 5px 1fr` : '1fr'}>
  {#if sidebarOpen}
    <Sidebar onnew={() => (form = { profile: null })} onedit={(p) => (form = { profile: p })} />
    <div class="vsplit" role="separator" aria-orientation="vertical" aria-label="Redimensionar panel lateral" onpointerdown={(e) => drag(e, 'x')}></div>
  {/if}

  <main>
    {#if ws.active && ws.tab}
      <TabBar ontoggle={() => (sidebarOpen = !sidebarOpen)} />
      {#if ws.queryTab}
        {@const q = ws.queryTab}
        <div class="toolbar">
          <button class="primary" onclick={() => ws.run()} disabled={q.running || !q.sql.trim()}>
            <Icon name="play" size={12} /> Ejecutar
          </button>
          <button onclick={() => ws.cancel()} disabled={!q.running}><Icon name="stop" size={12} /> Cancelar</button>
          <span class="grow"></span>
          <span class="keys"><kbd>Ctrl</kbd>+<kbd>Enter</kbd> ejecuta la selección o todo</span>
        </div>
        <div class="work">
          <div class="editor" style:height="{editorPct}%">
            {#key q.id}
              <QueryEditor bind:value={q.sql} driver={ws.active.driver} onrun={(text) => ws.run(text)} />
            {/key}
          </div>
          <div class="hsplit" role="separator" aria-orientation="horizontal" aria-label="Redimensionar editor" onpointerdown={(e) => drag(e, 'y')}></div>
          <div class="results"><ResultGrid result={q.result} error={q.error} running={q.running} /></div>
        </div>
      {:else if ws.dataTab}
        {#key ws.dataTab.id}<TableView tab={ws.dataTab} />{/key}
      {/if}
    {:else}
      <div class="welcome">
        <Icon name="database" size={40} />
        <h1>Pluton SQL</h1>
        <p>{ws.profiles.length ? 'Elige una conexión en el panel lateral para empezar.' : 'Crea tu primera conexión a PostgreSQL, MySQL, SQLite o SQL Server.'}</p>
        <button class="primary" onclick={() => (form = { profile: null })}><Icon name="plus" size={14} /> Nueva conexión</button>
      </div>
    {/if}
  </main>

  <div class="status"><StatusBar /></div>
</div>

{#if form}
  <ConnectionForm profile={form.profile} oncancel={() => (form = null)}
    onsaved={async (p) => { form = null; await ws.loadProfiles(); if (ws.active?.id === p.id || !ws.active) ws.connect(p) }} />
{/if}
{#if ui.schema}
  {#key ui.schema}
    <SchemaChangeDialog loc={ui.schema.loc} kind={ui.schema.kind} target={ui.schema.target} structure={ui.schema.structure}
      oncancel={() => (ui.schema = null)}
      ondone={(loc, op) => { ui.schema = null; void ws.schemaChanged(loc, op) }} />
  {/key}
{/if}
<ConfirmDialog />

<style>
  .app { display: grid; grid-template-rows: 1fr 26px; height: 100%; background: var(--surface); }
  .status { grid-column: 1 / -1; }
  main { display: flex; flex-direction: column; min-width: 0; min-height: 0; background: var(--surface); }
  .vsplit { cursor: col-resize; background: transparent; margin-left: -2px; position: relative; z-index: 5; }
  .vsplit:hover, .vsplit:active { background: var(--accent); opacity: .5; }
  .toolbar { display: flex; align-items: center; gap: 8px; padding: 6px 10px; border-bottom: 1px solid var(--border); background: var(--surface); }
  .grow { flex: 1; }
  .keys { color: var(--muted); font-size: 12px; }
  .work { flex: 1; display: flex; flex-direction: column; min-height: 0; }
  .editor { min-height: 60px; }
  .hsplit { height: 5px; margin: -2px 0; cursor: row-resize; position: relative; z-index: 5; background: transparent; border-top: 1px solid var(--border); }
  .hsplit:hover, .hsplit:active { background: var(--accent); opacity: .5; }
  .results { flex: 1; min-height: 0; }
  .welcome { margin: auto; display: flex; flex-direction: column; align-items: center; gap: 10px; color: var(--muted); text-align: center; padding: 24px; }
  .welcome h1 { margin: 4px 0 0; font-size: 22px; color: var(--text); }
  .welcome p { margin: 0 0 8px; max-width: 360px; }
</style>
