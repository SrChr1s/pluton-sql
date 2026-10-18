<script lang="ts">
  import { api, emptyProfile, errMsg, type Profile } from '../lib/api'

  let { profile, onsaved, oncancel }: { profile: Profile | null; onsaved: (p: Profile) => void; oncancel: () => void } = $props()

  let p = $state<Profile>(profile ? { ...emptyProfile(), ...profile } : emptyProfile())
  let password = $state('')
  let drivers = $state<string[]>([])
  let status = $state<{ ok: boolean; msg: string } | null>(null)
  let busy = $state(false)
  const isSqlite = $derived(p.driver === 'sqlite')

  api.drivers().then((d) => (drivers = d))

  async function onDriver() {
    p.port = await api.defaultPort(p.driver)
  }
  // Abre un diálogo nativo; si se cancela ("") conserva la ruta actual.
  async function pick(dialog: () => Promise<string>) {
    try {
      const path = await dialog()
      if (path) {
        p.filePath = path
        if (!p.name.trim()) p.name = path.split(/[\\/]/).pop()!.replace(/\.[^.]+$/, '')
        status = null
      }
    } catch (e) { status = { ok: false, msg: errMsg(e) } }
  }
  async function test() {
    busy = true; status = null
    try { await api.testConnection(p, password); status = { ok: true, msg: 'Conexión exitosa' } }
    catch (e) { status = { ok: false, msg: errMsg(e) } }
    busy = false
  }
  async function save() {
    if (!p.name.trim()) { status = { ok: false, msg: 'El nombre es obligatorio' }; return }
    busy = true
    try { onsaved(await api.saveConnection(p, password)) }
    catch (e) { status = { ok: false, msg: errMsg(e) }; busy = false }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && oncancel()} />

<div class="screen">
  <form class="card" onsubmit={(e) => { e.preventDefault(); save() }}>
    <h3>{profile ? 'Editar conexión' : 'Nueva conexión'}</h3>
    <label>Nombre <input bind:value={p.name} /></label>
    <label>Motor
      <select bind:value={p.driver} onchange={onDriver}>
        {#each drivers as d}<option value={d}>{d}</option>{/each}
      </select>
    </label>
    {#if isSqlite}
      <label>Archivo SQLite
        <div class="file">
          <input bind:value={p.filePath} placeholder="/ruta/base.db" />
          <button type="button" onclick={() => pick(api.pickSQLiteFile)} disabled={busy}>Examinar…</button>
          <button type="button" onclick={() => pick(api.createSQLiteFile)} disabled={busy} title="Crear un archivo de base de datos nuevo">Nueva…</button>
        </div>
      </label>
    {:else}
      <div class="row">
        <label style="flex:3">Host <input bind:value={p.host} /></label>
        <label style="flex:1">Puerto <input type="number" bind:value={p.port} /></label>
      </div>
      <div class="row">
        <label>Usuario <input bind:value={p.user} autocomplete="off" /></label>
        <label>Contraseña <input type="password" bind:value={password} placeholder={profile ? '(sin cambios)' : ''} autocomplete="new-password" /></label>
      </div>
      <label>Base de datos <input bind:value={p.database} placeholder="opcional" /></label>
      {#if p.driver === 'mssql'}
        <div class="box">
        <h4>Opciones de SQL Server</h4>
        <label>Cifrado
          <select bind:value={p.sslMode}>
            <option value="">Requerido — toda la sesión (por defecto)</option>
            <option value="false">Solo el login (el servidor decide el resto)</option>
            <option value="disable">Desactivado — sin cifrar (inseguro)</option>
          </select>
        </label>
        {#if p.sslMode === 'disable'}
          <p class="warn">⚠ Sin cifrado, usuario y contraseña viajan casi en claro. No lo uses por Internet.</p>
        {/if}
        </div>
      {/if}
      <div class="box">
        <label>Versión mínima de TLS
          <select bind:value={p.tlsMinVersion}>
            <option value="">Por defecto (1.2)</option>
            <option value="1.1">TLS 1.1 (legacy)</option>
            <option value="1.0">TLS 1.0 (legacy)</option>
          </select>
        </label>
        {#if p.tlsMinVersion === '1.0' || p.tlsMinVersion === '1.1'}
          <p class="warn">⚠ TLS {p.tlsMinVersion} es inseguro. Úsalo solo con servidores antiguos que no puedas actualizar; no se verifica el certificado del servidor.</p>
        {/if}
      </div>
    {/if}
    {#if status}<p class:ok={status.ok} class="status">{status.msg}</p>{/if}
    <div class="actions">
      <button type="button" onclick={oncancel}>Cancelar</button>
      <span style="flex:1"></span>
      <button type="button" onclick={test} disabled={busy}>Probar</button>
      <button type="submit" class="primary" disabled={busy}>Guardar y conectar</button>
    </div>
  </form>
</div>

<style>
  .screen { flex: 1; overflow: auto; display: flex; justify-content: center; align-items: flex-start; padding: 40px 24px; background: var(--bg); }
  .card { background: var(--surface); border-radius: 10px; padding: 20px; width: 100%; max-width: 560px; display: flex; flex-direction: column; gap: 12px; }
  h3 { margin: 0 0 4px; font-size: 18px; }
  h4 { margin: 0; font-size: 13px; }
  label { display: flex; flex-direction: column; gap: 4px; color: var(--muted); flex: 1; }
  .box { background: var(--bg); border-radius: 8px; padding: 12px; display: flex; flex-direction: column; gap: 10px; }
  .row { display: flex; gap: 10px; }
  .file { display: flex; gap: 6px; }
  .file input { flex: 1; }
  .file button { white-space: nowrap; }
  .actions { display: flex; gap: 8px; margin-top: 8px; padding-top: 14px; border-top: 1px solid var(--border); }
  .status { margin: 0; color: var(--danger); word-break: break-word; }
  .status.ok { color: var(--ok); }
  .warn { margin: 0; color: var(--warn); font-size: 12px; }
</style>
