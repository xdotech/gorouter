// ═══════════════════════════════════════════════════════════
// GoRouter Dashboard — app.js
// Full CRUD for all API endpoints
// ═══════════════════════════════════════════════════════════

// ── Utils ──────────────────────────────────────────────────
function showToast(message, type = '') {
  const c = document.getElementById('toast-container');
  const t = document.createElement('div');
  t.className = `toast ${type}`;
  t.textContent = message;
  c.appendChild(t);
  setTimeout(() => t.remove(), 3500);
}

let _autoLoginAttempted = false;

async function api(endpoint, opts = {}) {
  opts.credentials = 'include';
  try {
    const res = await fetch(`/api${endpoint}`, opts);
    if (res.status === 401) {
      // Auto-login once, then retry
      if (!_autoLoginAttempted) {
        _autoLoginAttempted = true;
        const loginRes = await fetch('/api/auth/login', {
          method: 'POST', credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ password: '123456' })
        });
        if (loginRes.ok) {
          return api(endpoint, opts); // retry original call
        }
      }
      // Auto-login failed — show manual login
      document.getElementById('loginModal').classList.add('active');
      throw new Error('Unauthorized');
    }
    _autoLoginAttempted = false; // reset on success
    if (!res.ok) {
      const body = await res.text();
      let msg = 'API Error';
      try { msg = JSON.parse(body).error || body; } catch { msg = body; }
      throw new Error(msg);
    }
    return res.status === 204 ? null : await res.json();
  } catch (e) {
    if (e.message !== 'Unauthorized') showToast(e.message, 'error');
    throw e;
  }
}

function postJSON(endpoint, data) {
  return api(endpoint, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
}
function putJSON(endpoint, data) {
  return api(endpoint, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(data) });
}
function del(endpoint) {
  return api(endpoint, { method: 'DELETE' });
}

function openModal(id) { document.getElementById(id).classList.add('active'); }
function closeModal(id) { document.getElementById(id).classList.remove('active'); }

function fmtNum(n) { return n == null ? '0' : Number(n).toLocaleString(); }
function fmtDate(s) {
  if (!s) return '—';
  const d = new Date(s);
  return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

// ── Navigation ─────────────────────────────────────────────
const pages = {
  dashboard: { load: loadDashboard },
  providers: { load: loadProviders },
  combos: { load: loadCombos },
  aliases: { load: loadAliases },
  keys: { load: loadKeys },
  usage: { load: loadUsage },
  pricing: { load: loadPricing },
  settings: { load: loadSettings },
};

function navigateTo(page) {
  document.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));
  document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
  const btn = document.querySelector(`.nav-item[data-page="${page}"]`);
  if (btn) btn.classList.add('active');
  document.getElementById(`page-${page}`)?.classList.add('active');
  location.hash = page;
  pages[page]?.load();
}

document.querySelectorAll('.nav-item[data-page]').forEach(btn => {
  btn.addEventListener('click', () => navigateTo(btn.dataset.page));
});

// ── Auth ───────────────────────────────────────────────────
document.getElementById('loginForm').addEventListener('submit', async e => {
  e.preventDefault();
  const pw = document.getElementById('loginPassword').value;
  try {
    await postJSON('/auth/login', { password: pw });
    closeModal('loginModal');
    document.getElementById('loginPassword').value = '';
    showToast('Login successful', 'success');
    loadDashboard();
  } catch { }
});

// ═══════════════════════════════════════════════════════════
// 1. Dashboard
// ═══════════════════════════════════════════════════════════
async function loadDashboard() {
  const container = document.getElementById('dashboardKpis');
  try {
    const [provData, comboData, usageData, settingsData] = await Promise.all([
      api('/providers').catch(() => []),
      api('/combos').catch(() => []),
      api('/usage/providers').catch(() => ({})),
      api('/settings').catch(() => ({})),
    ]);

    const providers = Array.isArray(provData) ? provData : [];
    const combos = Array.isArray(comboData) ? comboData : [];
    const active = providers.filter(p => p.isActive).length;

    let totalReqs = 0, totalTokens = 0;
    if (usageData && typeof usageData === 'object') {
      Object.values(usageData).forEach(s => {
        totalReqs += (s.requests || s.totalRequests || 0);
        totalTokens += (s.tokens_in || s.promptTokens || 0) + (s.tokens_out || s.completionTokens || 0);
      });
    }

    container.innerHTML = `
      <div class="kpi-card">
        <div class="kpi-top">
          <div class="kpi-icon blue"><span class="material-symbols-outlined">cloud</span></div>
          <div class="kpi-label">Providers</div>
        </div>
        <div class="kpi-value">${active}</div>
        <div class="kpi-sub">${providers.length} total, ${active} active</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-top">
          <div class="kpi-icon green"><span class="material-symbols-outlined">layers</span></div>
          <div class="kpi-label">Combos</div>
        </div>
        <div class="kpi-value">${combos.length}</div>
        <div class="kpi-sub">Fallback chains configured</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-top">
          <div class="kpi-icon purple"><span class="material-symbols-outlined">query_stats</span></div>
          <div class="kpi-label">Requests</div>
        </div>
        <div class="kpi-value">${fmtNum(totalReqs)}</div>
        <div class="kpi-sub">Total routed requests</div>
      </div>
      <div class="kpi-card">
        <div class="kpi-top">
          <div class="kpi-icon orange"><span class="material-symbols-outlined">token</span></div>
          <div class="kpi-label">Tokens</div>
        </div>
        <div class="kpi-value">${fmtNum(totalTokens)}</div>
        <div class="kpi-sub">Total tokens processed</div>
      </div>
    `;
  } catch { container.innerHTML = '<p class="text-muted">Failed to load dashboard data.</p>'; }
}

// ═══════════════════════════════════════════════════════════
// 2. Providers
// ═══════════════════════════════════════════════════════════
async function loadProviders() {
  const grid = document.getElementById('providerGrid');
  try {
    const data = await api('/providers');
    const providers = Array.isArray(data) ? data : [];
    if (providers.length === 0) {
      grid.innerHTML = '<p class="text-muted">No providers. Click "Add Provider" or use OAuth to connect.</p>';
      return;
    }
    grid.innerHTML = providers.map(p => {
      const status = p.isActive ? 'active' : (p.testStatus === 'unavailable' ? 'error' : 'unknown');
      const statusLabel = p.isActive ? 'Active' : (p.testStatus || 'Inactive');
      const authBadge = p.authType === 'oauth' ? '<span class="badge badge-oauth">OAuth</span>' : '<span class="badge badge-apikey">API Key</span>';
      const name = p.name || p.provider;
      return `<div class="provider-card">
        <div class="provider-card-top">
          <div>
            <div class="provider-card-name">${esc(name)}</div>
            <div class="provider-card-id">${esc(p.provider)}</div>
          </div>
          <span class="badge badge-${status}">${statusLabel}</span>
        </div>
        <div class="provider-card-meta">
          ${authBadge}
          ${p.apiKey ? `<span class="badge badge-blue">${esc(p.apiKey)}</span>` : ''}
          ${p.lastError ? `<span class="badge badge-error" title="${esc(p.lastError)}">Error</span>` : ''}
        </div>
        <div class="provider-card-actions">
          <button class="btn btn-sm btn-secondary" onclick="toggleProvider('${p.id}', ${!p.isActive})">
            ${p.isActive ? 'Disable' : 'Enable'}
          </button>
          <button class="btn btn-sm btn-danger" onclick="deleteProvider('${p.id}')">Delete</button>
        </div>
      </div>`;
    }).join('');
  } catch { grid.innerHTML = '<p class="text-muted">Failed to load providers.</p>'; }
}

document.getElementById('btnAddProvider').addEventListener('click', () => openModal('providerModal'));

document.getElementById('btnSaveProvider').addEventListener('click', async () => {
  const provider = document.getElementById('providerType').value;
  const name = document.getElementById('providerName').value || provider;
  const apiKey = document.getElementById('providerKey').value;
  if (!apiKey) { showToast('API Key is required', 'error'); return; }
  try {
    await postJSON('/providers', { provider, name, apiKey, isActive: true });
    closeModal('providerModal');
    document.getElementById('providerName').value = '';
    document.getElementById('providerKey').value = '';
    showToast('Provider added', 'success');
    loadProviders();
  } catch { }
});

window.toggleProvider = async (id, isActive) => {
  try {
    await putJSON(`/providers/${id}`, { isActive });
    showToast(`Provider ${isActive ? 'enabled' : 'disabled'}`, 'success');
    loadProviders();
  } catch { }
};

window.deleteProvider = async (id) => {
  if (!confirm('Delete this provider?')) return;
  try { await del(`/providers/${id}`); showToast('Deleted', 'success'); loadProviders(); } catch { }
};

// Qwen device code
document.getElementById('btnQwenDevice').addEventListener('click', async () => {
  try {
    const data = await postJSON('/oauth/qw/device-code', {});
    if (data.verification_uri) {
      window.open(data.verification_uri, '_blank');
      showToast('Check the opened page for the code. Polling...', 'success');
      pollQwen(data.device_code);
    }
  } catch { }
});

async function pollQwen(code) {
  for (let i = 0; i < 60; i++) {
    await new Promise(r => setTimeout(r, 5000));
    try {
      const res = await postJSON('/oauth/qw/poll', { device_code: code });
      if (res.status === 'connected') {
        showToast('Qwen connected!', 'success');
        loadProviders();
        return;
      }
    } catch { return; }
  }
  showToast('Qwen polling timed out', 'error');
}

// ═══════════════════════════════════════════════════════════
// 3. Combos
// ═══════════════════════════════════════════════════════════
async function loadCombos() {
  const tbody = document.getElementById('combosList');
  try {
    const data = await api('/combos');
    const combos = Array.isArray(data) ? data : [];
    if (combos.length === 0) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="3">No combos configured.</td></tr>';
      return;
    }
    tbody.innerHTML = combos.map(c => {
      const chain = (c.models || []).map(m => `<span class="model-tag">${esc(m)}</span>`).join('<span class="arrow">→</span>');
      return `<tr>
        <td><strong>${esc(c.name)}</strong></td>
        <td><div class="combo-chain">${chain}</div></td>
        <td><button class="btn btn-sm btn-danger" onclick="deleteCombo('${c.id}')">Delete</button></td>
      </tr>`;
    }).join('');
  } catch { }
}

document.getElementById('btnAddCombo').addEventListener('click', () => openModal('comboModal'));

document.getElementById('btnSaveCombo').addEventListener('click', async () => {
  const name = document.getElementById('comboName').value.trim();
  const text = document.getElementById('comboModels').value.trim();
  if (!name || !text) { showToast('Name and models required', 'error'); return; }
  const models = text.split('\n').map(s => s.trim()).filter(Boolean);
  try {
    await postJSON('/combos', { name, models });
    closeModal('comboModal');
    document.getElementById('comboName').value = '';
    document.getElementById('comboModels').value = '';
    showToast('Combo added', 'success');
    loadCombos();
  } catch { }
});

window.deleteCombo = async (id) => {
  if (!confirm('Delete this combo?')) return;
  try { await del(`/combos/${id}`); showToast('Deleted', 'success'); loadCombos(); } catch { }
};

// ═══════════════════════════════════════════════════════════
// 4. Aliases
// ═══════════════════════════════════════════════════════════
async function loadAliases() {
  const tbody = document.getElementById('aliasesList');
  try {
    const data = await api('/models/alias');
    const entries = Object.entries(data || {});
    if (entries.length === 0) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="3">No aliases configured.</td></tr>';
      return;
    }
    tbody.innerHTML = entries.map(([alias, target]) =>
      `<tr>
        <td><strong class="text-mono">${esc(alias)}</strong></td>
        <td class="text-mono">${esc(target)}</td>
        <td><button class="btn btn-sm btn-danger" onclick="deleteAlias('${esc(alias)}')">Delete</button></td>
      </tr>`
    ).join('');
  } catch { }
}

document.getElementById('btnAddAlias').addEventListener('click', () => openModal('aliasModal'));

document.getElementById('btnSaveAlias').addEventListener('click', async () => {
  const alias = document.getElementById('aliasName').value.trim();
  const target = document.getElementById('aliasTarget').value.trim();
  if (!alias || !target) { showToast('Alias and target required', 'error'); return; }
  try {
    await postJSON('/models/alias', { alias, target });
    closeModal('aliasModal');
    document.getElementById('aliasName').value = '';
    document.getElementById('aliasTarget').value = '';
    showToast('Alias added', 'success');
    loadAliases();
  } catch { }
});

window.deleteAlias = async (alias) => {
  if (!confirm(`Delete alias "${alias}"?`)) return;
  try { await del(`/models/alias/${alias}`); showToast('Deleted', 'success'); loadAliases(); } catch { }
};

// ═══════════════════════════════════════════════════════════
// 5. API Keys
// ═══════════════════════════════════════════════════════════
async function loadKeys() {
  const tbody = document.getElementById('keysList');
  try {
    const data = await api('/keys');
    const keys = Array.isArray(data) ? data : [];
    if (keys.length === 0) {
      tbody.innerHTML = '<tr class="empty-row"><td colspan="4">No API keys.</td></tr>';
      return;
    }
    tbody.innerHTML = keys.map(k =>
      `<tr>
        <td><strong>${esc(k.name)}</strong></td>
        <td class="text-mono">${esc(k.key)}</td>
        <td class="text-sm text-muted">${fmtDate(k.createdAt)}</td>
        <td><button class="btn btn-sm btn-danger" onclick="deleteKey('${k.id}')">Delete</button></td>
      </tr>`
    ).join('');
  } catch { }
}

document.getElementById('btnGenerateKey').addEventListener('click', async () => {
  const name = prompt('Key name:', 'API Key');
  if (!name) return;
  try {
    const key = await postJSON('/keys', { name });
    showToast('Key generated! Full key: ' + key.key, 'success');
    loadKeys();
  } catch { }
});

window.deleteKey = async (id) => {
  if (!confirm('Delete this key?')) return;
  try { await del(`/keys/${id}`); showToast('Deleted', 'success'); loadKeys(); } catch { }
};

// ═══════════════════════════════════════════════════════════
// 6. Usage
// ═══════════════════════════════════════════════════════════
let usagePage = 1;

async function loadUsage() {
  const kpiContainer = document.getElementById('usageKpis');
  const logBody = document.getElementById('requestLogList');

  try {
    const stats = await api('/usage/providers').catch(() => ({}));
    let html = '';
    if (stats && typeof stats === 'object') {
      const entries = Object.entries(stats);
      entries.forEach(([name, s]) => {
        const reqs = s.requests || s.totalRequests || 0;
        const tIn = s.tokens_in || s.promptTokens || 0;
        const tOut = s.tokens_out || s.completionTokens || 0;
        const cost = s.cost || s.estimatedCost || 0;
        html += `<div class="kpi-card">
          <div class="kpi-top">
            <div class="kpi-icon blue"><span class="material-symbols-outlined">cloud</span></div>
            <div class="kpi-label">${esc(name)}</div>
          </div>
          <div class="kpi-value">${fmtNum(reqs)}</div>
          <div class="kpi-sub">${fmtNum(tIn)} in / ${fmtNum(tOut)} out · $${Number(cost).toFixed(4)}</div>
        </div>`;
      });
    }
    kpiContainer.innerHTML = html || '<p class="text-muted">No usage data yet.</p>';
  } catch { }

  try {
    const logs = await api(`/usage/request-logs?page=${usagePage}&limit=20`).catch(() => ({ entries: [], total: 0 }));
    const entries = logs.entries || [];
    if (entries.length === 0) {
      logBody.innerHTML = '<tr class="empty-row"><td colspan="6">No request logs.</td></tr>';
    } else {
      logBody.innerHTML = entries.map(e => {
        const statusClass = (e.statusCode || 200) < 400 ? 'badge-active' : 'badge-error';
        return `<tr>
          <td class="text-sm">${fmtDate(e.timestamp || e.createdAt)}</td>
          <td>${esc(e.provider)}</td>
          <td class="text-mono text-sm">${esc(e.model)}</td>
          <td class="text-sm">${fmtNum(e.promptTokens)}/${fmtNum(e.completionTokens)}</td>
          <td><span class="badge ${statusClass}">${e.statusCode || 200}</span></td>
          <td class="text-sm">${e.durationMs || 0}ms</td>
        </tr>`;
      }).join('');
    }

    // Pagination
    const total = logs.total || 0;
    const totalPages = Math.ceil(total / 20) || 1;
    const pagDiv = document.getElementById('logPagination');
    let pagHtml = `<button ${usagePage <= 1 ? 'disabled' : ''} onclick="goUsagePage(${usagePage - 1})">‹</button>`;
    for (let i = 1; i <= Math.min(totalPages, 7); i++) {
      pagHtml += `<button class="${i === usagePage ? 'active' : ''}" onclick="goUsagePage(${i})">${i}</button>`;
    }
    pagHtml += `<button ${usagePage >= totalPages ? 'disabled' : ''} onclick="goUsagePage(${usagePage + 1})">›</button>`;
    pagDiv.innerHTML = pagHtml;
  } catch { }
}

window.goUsagePage = (p) => { usagePage = p; loadUsage(); };

// ═══════════════════════════════════════════════════════════
// 7. Pricing
// ═══════════════════════════════════════════════════════════
let pricingData = {};

async function loadPricing() {
  const container = document.getElementById('pricingEditor');
  try {
    pricingData = await api('/pricing') || {};
    renderPricing(container);
  } catch { }
}

function renderPricing(container) {
  const entries = Object.entries(pricingData);
  if (entries.length === 0) {
    container.innerHTML = `<p class="text-muted">No pricing data. Add model pricing below.</p>
      <div class="form-group mt-4">
        <label>Model Key</label>
        <input type="text" id="newPricingKey" class="form-control" placeholder="cc/claude-opus-4-6">
      </div>
      <div class="form-group">
        <label>Price per 1M tokens ($)</label>
        <input type="number" id="newPricingValue" class="form-control" step="0.01" placeholder="15.00">
      </div>
      <button class="btn btn-secondary btn-sm" onclick="addPricingEntry()"><span class="material-symbols-outlined">add</span> Add</button>`;
    return;
  }
  let html = entries.map(([k, v]) =>
    `<div class="toggle-row">
      <div>
        <div class="toggle-label text-mono">${esc(k)}</div>
      </div>
      <div class="flex items-center gap-8">
        <input type="number" class="form-control" style="width:100px" step="0.01" value="${v}" onchange="pricingData['${esc(k)}']=parseFloat(this.value)">
        <button class="btn-icon" onclick="delete pricingData['${esc(k)}']; renderPricing(document.getElementById('pricingEditor'))">
          <span class="material-symbols-outlined" style="font-size:16px">delete</span>
        </button>
      </div>
    </div>`
  ).join('');
  html += `<div class="mt-4 flex gap-8">
    <input type="text" id="newPricingKey" class="form-control" style="flex:1" placeholder="model key">
    <input type="number" id="newPricingValue" class="form-control" style="width:100px" step="0.01" placeholder="0.00">
    <button class="btn btn-sm btn-secondary" onclick="addPricingEntry()"><span class="material-symbols-outlined">add</span> Add</button>
  </div>`;
  container.innerHTML = html;
}

window.addPricingEntry = () => {
  const k = document.getElementById('newPricingKey').value.trim();
  const v = parseFloat(document.getElementById('newPricingValue').value);
  if (!k || isNaN(v)) { showToast('Key and price required', 'error'); return; }
  pricingData[k] = v;
  renderPricing(document.getElementById('pricingEditor'));
};

document.getElementById('btnSavePricing').addEventListener('click', async () => {
  try {
    await putJSON('/pricing', pricingData);
    showToast('Pricing saved', 'success');
  } catch { }
});

// ═══════════════════════════════════════════════════════════
// 8. Settings
// ═══════════════════════════════════════════════════════════
let currentSettings = {};

async function loadSettings() {
  const container = document.getElementById('settingsForm');
  try {
    currentSettings = await api('/settings') || {};
    container.innerHTML = `
      <div class="toggle-row">
        <div>
          <div class="toggle-label">Require Login</div>
          <div class="toggle-desc">Protect management API with password</div>
        </div>
        <label class="switch"><input type="checkbox" id="sRequireLogin" ${currentSettings.requireLogin ? 'checked' : ''}><span class="slider"></span></label>
      </div>
      <div class="toggle-row">
        <div>
          <div class="toggle-label">Require API Key</div>
          <div class="toggle-desc">Enforce Bearer token on /v1/* routes</div>
        </div>
        <label class="switch"><input type="checkbox" id="sRequireKey" ${currentSettings.requireApiKey ? 'checked' : ''}><span class="slider"></span></label>
      </div>
      <div class="toggle-row">
        <div>
          <div class="toggle-label">Cloud Enabled</div>
          <div class="toggle-desc">Enable cloud sync features</div>
        </div>
        <label class="switch"><input type="checkbox" id="sCloudEnabled" ${currentSettings.cloudEnabled ? 'checked' : ''}><span class="slider"></span></label>
      </div>
      <div class="toggle-row">
        <div>
          <div class="toggle-label">Observability</div>
          <div class="toggle-desc">Enable request logging and metrics</div>
        </div>
        <label class="switch"><input type="checkbox" id="sObservability" ${currentSettings.observabilityEnabled ? 'checked' : ''}><span class="slider"></span></label>
      </div>
      <div class="form-group mt-4">
        <label>Fallback Strategy</label>
        <select id="sFallbackStrategy" class="form-control">
          <option value="fill-first" ${currentSettings.fallbackStrategy === 'fill-first' ? 'selected' : ''}>Fill First</option>
          <option value="round-robin" ${currentSettings.fallbackStrategy === 'round-robin' ? 'selected' : ''}>Round Robin</option>
        </select>
      </div>
      <div class="form-group">
        <label>Sticky Round Robin Limit</label>
        <input type="number" id="sStickyLimit" class="form-control" value="${currentSettings.stickyRoundRobinLimit || 3}">
      </div>
      <div class="form-group">
        <label>Observability Max Records</label>
        <input type="number" id="sMaxRecords" class="form-control" value="${currentSettings.observabilityMaxRecords || 1000}">
      </div>
    `;
  } catch { }
}

document.getElementById('btnSaveSettings').addEventListener('click', async () => {
  const updates = {
    requireLogin: document.getElementById('sRequireLogin')?.checked || false,
    requireApiKey: document.getElementById('sRequireKey')?.checked || false,
    cloudEnabled: document.getElementById('sCloudEnabled')?.checked || false,
    observabilityEnabled: document.getElementById('sObservability')?.checked || false,
    fallbackStrategy: document.getElementById('sFallbackStrategy')?.value || 'fill-first',
    stickyRoundRobinLimit: parseInt(document.getElementById('sStickyLimit')?.value) || 3,
    observabilityMaxRecords: parseInt(document.getElementById('sMaxRecords')?.value) || 1000,
  };
  try {
    await putJSON('/settings', updates);
    showToast('Settings saved', 'success');
  } catch { }
});

// ── HTML escape ────────────────────────────────────────────
function esc(s) {
  if (s == null) return '';
  const d = document.createElement('div');
  d.textContent = String(s);
  return d.innerHTML;
}

// ── Bootstrap ──────────────────────────────────────────────
window.onload = () => {
  const hash = location.hash.replace('#', '');
  navigateTo(pages[hash] ? hash : 'dashboard');
};
