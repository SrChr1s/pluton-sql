<script lang="ts">
  import Menu, { type MenuItem } from './Menu.svelte'
  import { ws } from '../lib/workspace.svelte'
  import { ui } from '../lib/ui.svelte'

  /** Menú de la conexión activa (desconectar, editar, cambiar); lo abren la barra inferior y la cabecera del panel lateral. */
  let { up = false, onclose }: { up?: boolean; onclose: () => void } = $props()

  const dbName = $derived(ws.currentDb)
  const menuItems = $derived<MenuItem[]>([
    { label: 'Desconectar de la conexión', icon: 'power', run: () => void ws.disconnect() },
    ...(dbName && ws.active && ws.active.driver !== 'sqlite' && ws.isDbOpen(ws.active.id, dbName)
      ? [{ label: `Desconectar de ${dbName}`, icon: 'power', run: () => void ws.disconnectDatabase(dbName) }]
      : []),
    { label: 'Editar conexión', icon: 'pencil', run: () => { if (ws.active) ui.connForm = { profile: ws.active } } },
    {
      label: 'Cambiar de conexión', icon: 'switch',
      sub: ws.profiles.map((p) => ({ label: p.name, color: p.color || undefined, checked: p.id === ws.active?.id, run: () => ws.connect(p) })),
    },
  ])
</script>

<Menu items={menuItems} {up} {onclose} />
