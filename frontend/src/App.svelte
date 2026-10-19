<script lang="ts">
  import { untrack } from 'svelte'
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
  import Welcome from './components/Welcome.svelte'
  import TitleBar from './components/TitleBar.svelte'
  import Rail, { type Panel } from './components/Rail.svelte'
  import ExportDialog from './components/ExportDialog.svelte'
  import ImportDialog from './components/ImportDialog.svelte'
  import PromptDialog from './components/PromptDialog.svelte'
  import Toasts from './components/Toasts.svelte'
  import { ui } from './lib/ui.svelte'
  import { ws } from './lib/workspace.svelte'
  import type { Profile } from './lib/api'
  import { Environment } from '../wailsjs/runtime/runtime'

  const LAYOUT = 'pluton.layout'
  const saved = (() => { try { return JSON.parse(localStorage.getItem(LAYOUT) ?? '{}') } catch { return {} } })()

  let sidebarW = $state<number>(saved.sidebarW ?? 280)
  let sidebarOpen = $state<boolean>(saved.sidebarOpen ?? true)
  let editorPct = $state<number>(saved.editorPct ?? 42)
  let panel = $state<Panel>(saved.panel ?? 'connections')
  // Sin conexión activa solo tiene sentido la lista de conexiones.
  const shownPanel = $derived<Panel>(ws.active ? panel : 'connections')

  ws.loadProfiles()
  ws.loadWorkspaces()

  // Al cambiar de conexión activa (también desde el menú de la barra inferior) se muestran sus entidades.
  const activeId = $derived(ws.active?.id)
  $effect(() => { if (activeId) untrack(() => { panel = 'entities'; ui.connForm = null }) })

  // La barra de título propia solo hace falta en Linux (la ventana va sin marco).
  let customBar = $state(false)
  Environment().then((e) => (customBar = e.platform === 'linux')).catch(() => { /* fuera de Wails */ })

  // Carga el índice de autocompletado de la base de datos de la pestaña de consulta activa.
  $effect(() => { if (ws.queryTab) void ws.ensureIndex(ws.queryTab.database) })

  // Guarda borradores de las pestañas y la disposición con un pequeño retardo.
  $effect(() => {
    void ws.signature
    const h = setTimeout(() => ws.persist(), 400)
    return () => clearTimeout(h)
  })
  $effect(() => {
    const layout = JSON.stringify({ sidebarW, sidebarOpen, editorPct, panel })
    const h = setTimeout(() => { try { localStorage.setItem(LAYOUT, layout) } catch { /* no crítico */ } }, 300)
    return () => clearTimeout(h)
  })

  function onkeydown(e: KeyboardEvent) {
    const mod = e.ctrlKey || e.metaKey
    const k = e.key.toLowerCase()
    if (ui.connForm) return
    if (mod && k === 't') { e.preventDefault(); ws.newTab() }
    else if (mod && k === 'w') { e.preventDefault(); if (ws.tab) void ws.close(ws.tab.id) }
    else if (mod && k === 'b') { e.preventDefault(); sidebarOpen = !sidebarOpen }
    else if (mod && (e.key === 'PageDown' || (e.key === 'Tab' && !e.shiftKey))) { e.preventDefault(); ws.step(1) }
    else if (mod && (e.key === 'PageUp' || (e.key === 'Tab' && e.shiftKey))) { e.preventDefault(); ws.step(-1) }
    else if (e.altKey && /^[1-9]$/.test(e.key)) { e.preventDefault(); ws.selectNth(Number(e.key) - 1) }
    else if (e.key === 'F5') { e.preventDefault(); if (ws.dataTab) ws.dataTab.state.reload(); else ws.run() }
    else if (mod && k === 's') { e.preventDefault(); if (ws.dataTab) void ws.dataTab.state.apply(); else void ws.saveFile(ws.queryTab, e.shiftKey) }
    else if (mod && k === 'o') { e.preventDefault(); if (ws.active) void ws.openFile() }
    else if (mod && k === 'h') { e.preventDefault(); if (ws.active) { sidebarOpen = true; panel = shownPanel === 'history' ? 'entities' : 'history' } }
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

<div class="app" style:grid-template-columns="48px {sidebarOpen ? sidebarW : 0}px 1fr" style:grid-template-rows="{customBar ? 32 : 0}px 1fr 28px">
  {#if customBar}<div class="titlebar"><TitleBar /></div>{/if}
  <div class="rail"><Rail panel={shownPanel} onselect={(p) => { panel = p; sidebarOpen = true }} /></div>
  {#if sidebarOpen}
    <div class="side">
      <Sidebar panel={shownPanel} onnew={() => (ui.connForm = { profile: null })} onedit={(p) => (ui.connForm = { profile: p })}
        onconnect={() => { panel = 'entities'; ui.connForm = null }} />
      <div class="vsplit" role="separator" aria-orientation="vertical" aria-label="Redimensionar panel lateral" onpointerdown={(e) => drag(e, 'x')}></div>
    </div>
  {/if}

  <main>
    {#if ui.connForm}
      <ConnectionForm profile={ui.connForm.profile} oncancel={() => (ui.connForm = null)}
        onsaved={async (p, connect) => {
          ui.connForm = null
          await ws.loadProfiles()
          if (connect) { ws.connect(p); panel = 'entities' }
          else if (ws.active?.id === p.id) ws.active = ws.profiles.find((x) => x.id === p.id) ?? null // datos nuevos; se reconecta al usarla
        }} />
    {:else if ws.active && ws.tab}
      <TabBar ontoggle={() => (sidebarOpen = !sidebarOpen)} />
      {#if ws.queryTab}
        {@const q = ws.queryTab}
        <div class="work">
          <div class="editor" style:height="{editorPct}%">
            {#key q.id}
              <QueryEditor bind:value={q.sql} driver={ws.active.driver} index={ws.indexFor(q.database)} onrun={(text) => ws.run(text)} />
            {/key}
            <div class="fab">
              <button class="ghost" title="Abrir un script .sql (Ctrl+O)" onclick={() => ws.openFile()}><Icon name="folder" size={14} /> Abrir</button>
              <button title={q.filePath ? `Guardar en ${q.filePath} (Ctrl+S)` : 'Guardar como archivo .sql (Ctrl+S)'} onclick={() => ws.saveFile(q)}>Guardar</button>
              {#if q.filePath}<button class="ghost" title="Guardar como… (Ctrl+Shift+S)" onclick={() => ws.saveFile(q, true)}>Guardar como…</button>{/if}
              {#if q.running}
                <button onclick={() => ws.cancel()}><Icon name="stop" size={12} /> Cancelar</button>
              {:else}
                <button class="run" title="Ejecuta la selección o todo (Ctrl+Enter)" onclick={() => ws.run()} disabled={!q.sql.trim()}>Ejecutar</button>
              {/if}
            </div>
          </div>
          <div class="hsplit" role="separator" aria-orientation="horizontal" aria-label="Redimensionar editor" onpointerdown={(e) => drag(e, 'y')}></div>
          <div class="results"><ResultGrid result={q.result} error={q.error} running={q.running} /></div>
        </div>
      {:else if ws.dataTab}
        {#key ws.dataTab.id}<TableView tab={ws.dataTab} />{/key}
      {/if}
    {:else}
      <Welcome hasConnections={ws.profiles.length > 0} onnew={() => (ui.connForm = { profile: null })} />
    {/if}
  </main>

  <StatusBar />
</div>

{#if ui.schema}
  {#key ui.schema}
    <SchemaChangeDialog loc={ui.schema.loc} kind={ui.schema.kind} target={ui.schema.target} structure={ui.schema.structure}
      oncancel={() => (ui.schema = null)}
      ondone={(loc, op) => { ui.schema = null; void ws.schemaChanged(loc, op) }} />
  {/key}
{/if}
{#if ui.exporting}{#key ui.exporting}<ExportDialog source={ui.exporting} oncancel={() => (ui.exporting = null)} />{/key}{/if}
{#if ui.importing}{#key ui.importing}<ImportDialog target={ui.importing} oncancel={() => (ui.importing = null)} />{/key}{/if}
<ConfirmDialog />
<PromptDialog />
<Toasts />

<style>
  .app { display: grid; height: 100%; background: var(--bg); }
  .titlebar { grid-column: 1 / -1; grid-row: 1; }
  .rail { grid-column: 1; grid-row: 2; min-height: 0; }
  .side { grid-column: 2; grid-row: 2; min-height: 0; min-width: 0; position: relative; }
  .vsplit { position: absolute; top: 0; right: 0; bottom: 0; width: 4px; cursor: col-resize; z-index: 5; }
  .vsplit:hover, .vsplit:active { background: var(--accent); opacity: .5; }
  main { grid-column: 3; grid-row: 2; display: flex; flex-direction: column; min-width: 0; min-height: 0; background: var(--bg); }
  .app > :global(footer .left) { grid-column: 1 / 3; grid-row: 3; }
  .app > :global(footer .right) { grid-column: 3; grid-row: 3; }
  .work { flex: 1; display: flex; flex-direction: column; min-height: 0; min-width: 0; }
  .editor { min-height: 60px; position: relative; background: var(--surface); }
  .fab { position: absolute; right: 18px; bottom: 10px; display: flex; gap: 8px; z-index: 3; }
  .fab button { background: var(--surface-2); }
  .fab button.ghost { background: transparent; }
  .fab button.run { background: var(--accent); color: var(--accent-fg); border-color: var(--accent); font-weight: 600; }
  .fab button.run:hover:not(:disabled) { background: var(--accent); filter: brightness(1.1); }
  .hsplit { height: 5px; margin: -2px 0; cursor: row-resize; position: relative; z-index: 5; background: transparent; }
  .hsplit:hover, .hsplit:active { background: var(--accent); opacity: .5; }
  .results { flex: 1; min-height: 0; }
</style>
