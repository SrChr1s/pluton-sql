<script lang="ts">
  // Plutón estilizado: disco con el "corazón" (Tombaugh Regio), bruma azul de su atmósfera y a Caronte en órbita.
  // El mismo dibujo, sobre fondo oscuro, está en build/appicon.svg (de ahí sale el icono de la app).
  let { size = 24, orbit = true, animate = false }: { size?: number; orbit?: boolean; animate?: boolean } = $props()
  const uid = Math.random().toString(36).slice(2, 8)
</script>

{#snippet path()}
  <ellipse cx="50" cy="50" rx="46" ry="15" fill="none" stroke="#a99bff" stroke-opacity=".4" stroke-width="1" stroke-dasharray="2 3" />
{/snippet}

<!-- Caronte. Animado, recorre toda la órbita y cada mitad lo recorta para mostrarlo solo mientras pasa por ella. -->
{#snippet moon()}
  {#if animate}
    <circle r="5" fill="#cfcbd8" stroke="#8a869c" stroke-width=".8">
      <animateMotion dur="24s" repeatCount="indefinite" path="M96 50 A46 15 0 1 1 4 50 A46 15 0 1 1 96 50" />
    </circle>
  {:else}
    <circle cx="86" cy="39" r="5" fill="#cfcbd8" stroke="#8a869c" stroke-width=".8" />
  {/if}
{/snippet}

<svg width={size} height={size} viewBox="0 0 100 100" role="img" aria-label="Plutón SQL" class:animate>
  <defs>
    <radialGradient id="body-{uid}" cx="36%" cy="30%" r="80%">
      <stop offset="0" stop-color="#f3dcc2" />
      <stop offset=".55" stop-color="#cf9f78" />
      <stop offset="1" stop-color="#7d5142" />
    </radialGradient>
    <radialGradient id="shade-{uid}" cx="30%" cy="26%" r="85%">
      <stop offset=".55" stop-color="#000" stop-opacity="0" />
      <stop offset="1" stop-color="#14091f" stop-opacity=".62" />
    </radialGradient>
    <clipPath id="disc-{uid}"><circle cx="50" cy="50" r="27" /></clipPath>
    <!-- Mitades de la órbita (en sus propias coordenadas, antes de inclinarla): la de arriba queda detrás del planeta y la de abajo delante. -->
    <clipPath id="back-{uid}"><rect x="-20" y="-20" width="140" height="70" /></clipPath>
    <clipPath id="front-{uid}"><rect x="-20" y="50" width="140" height="70" /></clipPath>
  </defs>

  {#if orbit}
    <!-- Mitad trasera de la órbita: el planeta la tapa. Caronte pasa por aquí al ir por detrás. -->
    <g transform="rotate(-24 50 50)">
      <g clip-path="url(#back-{uid})">
        {@render path()}
        {@render moon()}
      </g>
    </g>
  {/if}

  <circle cx="50" cy="50" r="30" fill="#8fb3ff" fill-opacity=".14" />
  <circle cx="50" cy="50" r="27" fill="url(#body-{uid})" />
  <g clip-path="url(#disc-{uid})">
    <!-- Tombaugh Regio (el corazón) -->
    <path d="M48 70 C26 55 28 36 41 35 C47 35 50 39 52 43 C55 37 63 35 68 40 C74 47 67 60 48 70Z" fill="#fdf0de" fill-opacity=".92" />
    <!-- Cthulhu Macula: la mancha oscura -->
    <ellipse cx="32" cy="58" rx="9" ry="6" fill="#5a3427" fill-opacity=".7" transform="rotate(-18 32 58)" />
    <ellipse cx="66" cy="62" rx="5" ry="3.2" fill="#6b4331" fill-opacity=".45" />
    <circle cx="50" cy="50" r="27" fill="url(#shade-{uid})" />
  </g>

  {#if orbit}
    <!-- Mitad delantera: la línea cruza el planeta y Caronte pasa por delante. -->
    <g transform="rotate(-24 50 50)">
      <g clip-path="url(#front-{uid})">
        {@render path()}
        {#if animate}{@render moon()}{/if}
      </g>
    </g>
  {/if}
</svg>

<style>
  svg { display: block; flex: none; }
  @media (prefers-reduced-motion: reduce) { svg :global(animateMotion) { display: none; } }
</style>
