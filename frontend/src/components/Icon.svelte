<script lang="ts">
  // Iconos de trazo (estilo Lucide). Cada valor son uno o varios paths del viewBox 24x24.
  const icons: Record<string, string[]> = {
    power: ['M12 2v10', 'M18.4 6.6a9 9 0 1 1-12.8 0'],
    collapse: ['M7 20l5-5 5 5', 'M7 4l5 5 5-5'],
    chevrons: ['M7 15l5 5 5-5', 'M7 9l5-5 5 5'],
    link: ['M10 13a5 5 0 0 0 7.5.5l3-3a5 5 0 0 0-7-7l-1.7 1.7', 'M14 11a5 5 0 0 0-7.5-.5l-3 3a5 5 0 0 0 7 7l1.7-1.7'],
    switch: ['M17 3l4 4-4 4', 'M3 7h18', 'M7 21l-4-4 4-4', 'M21 17H3'],
    grid: ['M3 3h8v8H3z', 'M13 3h8v8h-8z', 'M3 13h8v8H3z', 'M13 13h8v8h-8z'],
    orbit: ['M12 9.5a2.5 2.5 0 1 0 0 5 2.5 2.5 0 0 0 0-5z', 'M21.5 12c0 2.4-4.3 4.5-9.5 4.5S2.5 14.4 2.5 12 6.8 7.5 12 7.5s9.5 2.1 9.5 4.5z', 'M19 7.2l.01 0'],
    minus: ['M5 12h14'],
    maximize: ['M5 5h14v14H5z'],
    plus: ['M12 5v14', 'M5 12h14'],
    x: ['M18 6 6 18', 'M6 6l12 12'],
    play: ['M7 4l13 8-13 8z'],
    stop: ['M6 6h12v12H6z'],
    database: ['M3 5c0-1.1 4-2 9-2s9 .9 9 2-4 2-9 2-9-.9-9-2z', 'M3 5v14c0 1.1 4 2 9 2s9-.9 9-2V5', 'M3 12c0 1.1 4 2 9 2s9-.9 9-2'],
    table: ['M3 3h18v18H3z', 'M3 9h18', 'M3 15h18', 'M9 3v18'],
    eyeoff: ['M9.9 5.1A10 10 0 0 1 12 5c6.5 0 10 7 10 7a17 17 0 0 1-3.2 4', 'M6.6 6.6A17 17 0 0 0 2 12s3.5 7 10 7a9.7 9.7 0 0 0 5-1.4', 'M9.9 9.9a3 3 0 0 0 4.2 4.2', 'M3 3l18 18'],
    view: ['M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z', 'M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z'],
    layers: ['M12 2 2 7l10 5 10-5-10-5z', 'M2 17l10 5 10-5', 'M2 12l10 5 10-5'],
    right: ['M9 6l6 6-6 6'],
    down: ['M6 9l6 6 6-6'],
    up: ['M18 15l-6-6-6 6'],
    left: ['M15 6l-6 6 6 6'],
    undo: ['M3 7v6h6', 'M3 13a9 9 0 1 0 3-7.7L3 8'],
    check: ['M20 6 9 17l-5-5'],
    sun: ['M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z', 'M12 2v2', 'M12 20v2', 'M4.9 4.9l1.4 1.4', 'M17.7 17.7l1.4 1.4', 'M2 12h2', 'M20 12h2', 'M4.9 19.1l1.4-1.4', 'M17.7 6.3l1.4-1.4'],
    moon: ['M21 12.8A9 9 0 1 1 11.2 3a7 7 0 0 0 9.8 9.8z'],
    monitor: ['M3 4h18v12H3z', 'M8 20h8', 'M12 16v4'],
    pencil: ['M12 20h9', 'M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4z'],
    trash: ['M3 6h18', 'M8 6V4h8v2', 'M19 6l-1 14H6L5 6'],
    refresh: ['M21 12a9 9 0 1 1-3-6.7L21 8', 'M21 3v5h-5'],
    search: ['M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14z', 'M21 21l-4.3-4.3'],
    server: ['M4 4h16v6H4z', 'M4 14h16v6H4z', 'M8 7h.01', 'M8 17h.01'],
    key: ['M15 7a4 4 0 1 1-1.2 2.8L3 20v-3h3v-3h3l1.8-1.8A4 4 0 0 1 15 7z'],
    sidebar: ['M3 4h18v16H3z', 'M9 4v16'],
    sql: ['M8 6 3 12l5 6', 'M16 6l5 6-5 6'],
    folder: ['M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z'],
    save: ['M5 3h11l3 3v13a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z', 'M7 3v5h8V3', 'M7 21v-7h10v7'],
    download: ['M12 3v12', 'M7 11l5 5 5-5', 'M5 21h14'],
    upload: ['M12 21V9', 'M7 13l5-5 5 5', 'M5 3h14'],
    clock: ['M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z', 'M12 7v5l3 2'],
    bookmark: ['M6 3h12v18l-6-4-6 4z'],
    copy: ['M9 9h11v11H9z', 'M5 15V5a1 1 0 0 1 1-1h10'],
    file: ['M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8z', 'M14 3v5h5'],
  }
  let { name, size = 16 }: { name: string; size?: number } = $props()
  const fill = $derived(name === 'play' || name === 'stop')
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill={fill ? 'currentColor' : 'none'} stroke="currentColor"
     stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
  {#each icons[name] ?? [] as d}<path {d} />{/each}
</svg>

<style>
  svg { flex: none; display: block; }
</style>
