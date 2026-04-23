const API = window.location.origin;
let token = localStorage.getItem('token');
let currentStep = 1;

// --- Init ---
if (token) {
    showDashboard();
} else {
    document.getElementById('auth-screen').classList.remove('hidden');
}

// --- Auth ---
function showTab(tab) {
    document.querySelectorAll('.tab').forEach(t => t.classList.remove('active'));
    event.target.classList.add('active');
    document.getElementById('login-form').classList.toggle('hidden', tab !== 'login');
    document.getElementById('register-form').classList.toggle('hidden', tab !== 'register');
    document.getElementById('auth-error').classList.add('hidden');
}

async function handleLogin(e) {
    e.preventDefault();
    const email = document.getElementById('login-email').value;
    const password = document.getElementById('login-password').value;
    try {
        const res = await fetch(API + '/api/auth/login', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Login failed');
        token = data.token;
        localStorage.setItem('token', token);
        localStorage.setItem('email', email);
        showDashboard();
    } catch (err) {
        showError(err.message);
    }
}

async function handleRegister(e) {
    e.preventDefault();
    const email = document.getElementById('reg-email').value;
    const password = document.getElementById('reg-password').value;
    const displayName = document.getElementById('reg-name').value;
    const role = document.getElementById('reg-role').value;
    try {
        const res = await fetch(API + '/api/auth/register', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password })
        });
        const data = await res.json();
        if (!res.ok) throw new Error(data.error || 'Registration failed');
        token = data.token;
        localStorage.setItem('token', token);
        localStorage.setItem('email', email);

        // Create profile
        await fetch(API + '/api/users/profile', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
            body: JSON.stringify({ display_name: displayName, role: role })
        });
        showDashboard();
    } catch (err) {
        showError(err.message);
    }
}

function logout() {
    token = null;
    localStorage.removeItem('token');
    localStorage.removeItem('email');
    document.getElementById('auth-screen').classList.remove('hidden');
    document.getElementById('dashboard-screen').classList.add('hidden');
}

function showError(msg) {
    const el = document.getElementById('auth-error');
    el.textContent = msg;
    el.classList.remove('hidden');
}

// --- Dashboard ---
async function showDashboard() {
    document.getElementById('auth-screen').classList.add('hidden');
    document.getElementById('dashboard-screen').classList.remove('hidden');
    document.getElementById('user-email').textContent = localStorage.getItem('email') || '';
    checkHealth();
    loadProfile();
    loadAssets();
    setInterval(checkHealth, 15000);
}

async function checkHealth() {
    const services = ['auth', 'users', 'inventory'];
    const names = ['auth', 'user', 'inventory'];
    for (let i = 0; i < services.length; i++) {
        try {
            const res = await fetch(API + '/api/' + services[i] + '/health', { signal: AbortSignal.timeout(3000) });
            const dot = document.querySelector('#health-' + names[i] + ' .dot');
            dot.className = 'dot ' + (res.ok ? 'up' : 'down');
        } catch {
            const dot = document.querySelector('#health-' + names[i] + ' .dot');
            dot.className = 'dot down';
        }
    }
}

async function loadProfile() {
    try {
        const res = await fetch(API + '/api/users/profile/me', {
            headers: { 'Authorization': 'Bearer ' + token }
        });
        if (!res.ok) return;
        const profile = await res.json();
        document.getElementById('profile-name').textContent = profile.display_name;
        document.getElementById('profile-role').textContent = profile.role;
        currentStep = profile.onboarding_step || 1;
        updateStepper(currentStep, profile.onboarding_done);
    } catch {}
}

function updateStepper(step, done) {
    document.querySelectorAll('.step').forEach(el => {
        const s = parseInt(el.dataset.step);
        el.classList.remove('active', 'done');
        if (s < step || done) el.classList.add('done');
        else if (s === step && !done) el.classList.add('active');
    });
    const btn = document.getElementById('next-step-btn');
    const status = document.getElementById('onboarding-status');
    if (done) {
        btn.classList.add('hidden');
        status.textContent = 'Onboarding complete! Starter pack assigned.';
    } else {
        btn.classList.remove('hidden');
        status.textContent = 'Step ' + step + ' of 5';
    }
}

async function advanceStep() {
    const nextStep = currentStep + 1;
    try {
        const res = await fetch(API + '/api/users/onboarding', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
            body: JSON.stringify({ onboarding_step: nextStep })
        });
        const data = await res.json();
        if (!res.ok) return;
        const profile = data.profile;
        currentStep = profile.onboarding_step;
        updateStepper(currentStep, profile.onboarding_done);
        if (profile.onboarding_done) {
            setTimeout(loadAssets, 2000);
        }
    } catch {}
}

// --- Assets ---
async function loadAssets() {
    const filter = document.getElementById('asset-filter').value;
    const url = API + '/api/inventory/assets' + (filter ? '?category=' + filter : '');
    try {
        const res = await fetch(url);
        const assets = await res.json();
        const tbody = document.getElementById('assets-body');
        const noAssets = document.getElementById('no-assets');
        if (!assets || assets.length === 0) {
            tbody.innerHTML = '';
            noAssets.classList.remove('hidden');
            return;
        }
        noAssets.classList.add('hidden');
        tbody.innerHTML = assets.map(a =>
            '<tr>' +
            '<td>' + esc(a.name) + '</td>' +
            '<td>' + esc(a.category) + '</td>' +
            '<td><span class="badge ' + a.status + '">' + a.status + '</span></td>' +
            '<td>' + (a.assigned_to || '-') + '</td>' +
            '</tr>'
        ).join('');
    } catch {
        document.getElementById('assets-body').innerHTML = '';
    }
}

function showAddAsset() { document.getElementById('add-asset-form').classList.remove('hidden'); }
function hideAddAsset() { document.getElementById('add-asset-form').classList.add('hidden'); }

async function createAsset() {
    const name = document.getElementById('asset-name').value;
    const category = document.getElementById('asset-category').value;
    if (!name) return;
    try {
        await fetch(API + '/api/inventory/assets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'Authorization': 'Bearer ' + token },
            body: JSON.stringify({ name, category, metadata: {} })
        });
        document.getElementById('asset-name').value = '';
        hideAddAsset();
        loadAssets();
    } catch {}
}

function esc(s) { const d = document.createElement('div'); d.textContent = s; return d.innerHTML; }
