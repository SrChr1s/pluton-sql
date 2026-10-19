<script lang="ts">
  import Icon from './Icon.svelte'
  import Logo from './Logo.svelte'

  let { hasConnections, onnew }: { hasConnections: boolean; onnew: () => void } = $props()

  // Campo de estrellas determinista (misma disposición en cada arranque).
  let seed = 7
  const rnd = () => ((seed = (seed * 16807) % 2147483647) / 2147483647)
  const stars = Array.from({ length: 70 }, () => ({
    x: rnd() * 100, y: rnd() * 100, r: 0.4 + rnd() * 1.1, o: 0.25 + rnd() * 0.6, d: rnd() * 6,
  }))
</script>

<div class="welcome">
  <svg class="sky" viewBox="0 0 100 100" preserveAspectRatio="none" aria-hidden="true">
    {#each stars as s}
      <circle cx={s.x} cy={s.y} r={s.r * 0.18} fill="#fff" opacity={s.o} style:animation-delay="{s.d}s" class="star" />
    {/each}
  </svg>
  <div class="content">
    <Logo size={200} animate />
    <h1>Pluton SQL</h1>
    <p>{hasConnections ? 'Elige una conexión en el panel lateral y explora tus bases de datos.' : 'Crea tu primera conexión a PostgreSQL, MySQL, SQLite o SQL Server.'}</p>
    <button class="primary" onclick={onnew}><Icon name="plus" size={14} /> Nueva conexión</button>
  </div>
</div>

<style>
  .welcome { position: relative; flex: 1; display: grid; place-items: center; overflow: hidden;
    background: radial-gradient(ellipse 60% 55% at 50% 42%, color-mix(in srgb, var(--accent) 16%, transparent), transparent 70%), var(--bg); }
  .sky { position: absolute; inset: 0; width: 100%; height: 100%; }
  .star { animation: twinkle 5s ease-in-out infinite; }
  .content { position: relative; display: flex; flex-direction: column; align-items: center; gap: 10px; text-align: center; padding: 24px; color: var(--muted); }
  h1 { margin: 6px 0 0; font-size: 26px; letter-spacing: .02em; color: var(--text); }
  p { margin: 0 0 8px; max-width: 380px; }
  @keyframes twinkle { 50% { opacity: .15; } }
  @media (prefers-reduced-motion: reduce) { .star { animation: none; } }
  :global(:root[data-theme="light"]) .sky { display: none; }
</style>
