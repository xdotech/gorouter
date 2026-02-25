// Utils
function showToast(message) {
    const container = document.getElementById('toast-container');
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(() => { toast.remove(); }, 3000);
}

// Global fetch wrapper for /api
async function apiCall(endpoint, options = {}) {
    try {
        options.credentials = 'include'; // Send cookies (JWT)
        const res = await fetch(`/api${endpoint}`, options);
        if (res.status === 401) {
            document.getElementById('loginModal').classList.add('active');
            throw new Error('Unauthorized');
        }
        if (!res.ok) {
            const err = await res.text();
            throw new Error(err || 'API Error');
        }
        return res.status !== 204 ? await res.json() : null;
    } catch (e) {
        if (e.message !== 'Unauthorized') showToast(`Error: ${e.message}`);
        throw e;
    }
}

// Navigation
document.querySelectorAll('nav a').forEach(link => {
    link.addEventListener('click', (e) => {
        e.preventDefault();
        document.querySelectorAll('nav a').forEach(l => l.classList.remove('active'));
        document.querySelectorAll('.page').forEach(p => p.classList.remove('active'));
        
        e.target.classList.add('active');
        const pageId = `page-${e.target.dataset.page}`;
        document.getElementById(pageId).classList.add('active');
        
        // Router hook
        if(e.target.dataset.page === 'providers') loadProviders();
        if(e.target.dataset.page === 'combos') loadCombos();
        if(e.target.dataset.page === 'usage') loadUsage();
    });
});

// Auth
document.getElementById('loginForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const password = document.getElementById('loginPassword').value;
    try {
        await apiCall('/auth/login', {
            method: 'POST',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({ password })
        });
        document.getElementById('loginModal').classList.remove('active');
        document.getElementById('loginPassword').value = '';
        showToast('Login successful');
        loadProviders(); // Default page load
    } catch (err) {}
});

document.getElementById('btnLogout').addEventListener('click', async (e) => {
    e.preventDefault();
    await apiCall('/auth/logout', { method: 'POST' });
    document.getElementById('loginModal').classList.add('active');
});

// Loaders
async function loadProviders() {
    const tbody = document.getElementById('providersList');
    try {
        const data = await apiCall('/providers');
        tbody.innerHTML = '';
        if(!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" style="text-align: center;">No providers added.</td></tr>';
            return;
        }
        
        data.forEach(p => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td><strong>${p.id}</strong></td>
                <td><span class="badge ${p.type}">${p.type}</span></td>
                <td>${p.provider}</td>
                <td>${p.enabled ? '🟢 Active' : '🔴 Disabled'}</td>
                <td>
                    <button class="btn btn-secondary btn-sm" onclick="editProvider('${p.id}')">Edit</button>
                    ${p.type === 'oauth' && p.status === 'device_code_pending' ? 
                        `<a href="${p.verification_uri}" target="_blank" class="btn btn-primary btn-sm">Login</a>` : ''}
                </td>
            `;
            tbody.appendChild(tr);
        });
    } catch(e) {}
}

async function loadCombos() {
    const tbody = document.getElementById('combosList');
    try {
        const data = await apiCall('/combos');
        tbody.innerHTML = '';
        if(!data || Object.keys(data).length === 0) {
            tbody.innerHTML = '<tr><td colspan="3" style="text-align: center;">No combos configured.</td></tr>';
            return;
        }
        
        for (const [name, combo] of Object.entries(data)) {
            const tr = document.createElement('tr');
            const modelsHtml = combo.models.map(m => `<span class="badge" style="margin-right: 4px;">${m}</span>`).join(' → ');
            tr.innerHTML = `
                <td><strong>${name}</strong></td>
                <td>${modelsHtml}</td>
                <td><button class="btn btn-danger btn-sm" onclick="deleteCombo('${name}')">Delete</button></td>
            `;
            tbody.appendChild(tr);
        }
    } catch(e) {}
}

async function loadUsage() {
    // simplified load dummy or actual data if backend has it
    const tbody = document.getElementById('usageList');
    try {
        const data = await apiCall('/usage/providers');
        tbody.innerHTML = '';
        if(!data || Object.keys(data).length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" style="text-align: center;">No usage data yet.</td></tr>';
            return;
        }
        
        for (const [name, stat] of Object.entries(data)) {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td><strong>${name}</strong></td>
                <td>${stat.requests}</td>
                <td>${stat.tokens_in}</td>
                <td>${stat.tokens_out}</td>
                <td>$${stat.cost.toFixed(4)}</td>
            `;
            tbody.appendChild(tr);
        }
    } catch(e) {}
}

// Initial bootstrap
window.onload = () => {
    // Try hitting settings to verify auth state
    apiCall('/settings').then(data => {
        document.getElementById('settingRequireKey').checked = data.require_api_key;
        loadProviders();
    }).catch(e => {}); // Modal will trigger on 401
};
