let csrfToken = '';

function showLogin(message) {
  document.getElementById('loginView').classList.remove('hidden');
  document.getElementById('appView').classList.add('hidden');
  const msg = document.getElementById('loginMsg');
  if (message) {
    msg.textContent = message;
    msg.classList.remove('hidden');
  } else {
    msg.textContent = '';
    msg.classList.add('hidden');
  }
  document.getElementById('loginToken').value = '';
  csrfToken = '';
}

function showApp() {
  document.getElementById('loginView').classList.add('hidden');
  document.getElementById('appView').classList.remove('hidden');
}

async function fetchJSON(url, options = {}) {
  const res = await fetch(url, {
    credentials: 'same-origin',
    ...options,
  });
  const text = await res.text();
  if (!res.ok) {
    throw new Error(text || res.statusText);
  }
  return text ? JSON.parse(text) : null;
}

async function login() {
  const token = document.getElementById('loginToken').value.trim();
  if (!token) {
    showLogin('请输入 ADMIN_TOKEN');
    return;
  }

  try {
    const data = await fetchJSON('/api/admin/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    });
    csrfToken = (data && data.csrf_token) || '';
    if (!csrfToken) throw new Error('missing csrf token');
    showApp();
    await refreshAll();
  } catch (err) {
    showLogin('登录失败，请检查密钥');
  }
}

async function logout() {
  try {
    await fetch('/api/admin/logout', {
      method: 'POST',
      credentials: 'same-origin',
      headers: csrfToken ? { 'X-CSRF-Token': csrfToken } : {},
    });
  } catch (_) {}
  showLogin('');
}

function writeHeaders() {
  return {
    'Content-Type': 'application/json',
    'X-CSRF-Token': csrfToken,
  };
}

function appendTextCell(row, value) {
  const cell = document.createElement('td');
  cell.textContent = value == null ? '' : String(value);
  row.appendChild(cell);
  return cell;
}

function appendCodeCell(row, value) {
  const cell = document.createElement('td');
  const code = document.createElement('code');
  code.textContent = value == null ? '' : String(value);
  cell.appendChild(code);
  row.appendChild(cell);
  return cell;
}

function createSubscriptionLink(format, token, label) {
  const link = document.createElement('a');
  link.href = '/api/' + format + '/' + encodeURIComponent(token || '');
  link.target = '_blank';
  link.rel = 'noopener noreferrer';
  link.textContent = label;
  return link;
}

function appendSubscriptionCell(row, token) {
  const cell = document.createElement('td');
  [
    ['subscription', 'Subscription'],
    ['v2ray', 'V2Ray'],
    ['singbox', 'Sing-box'],
  ].forEach(([format, label]) => {
    const wrapper = document.createElement('div');
    wrapper.appendChild(createSubscriptionLink(format, token, label));
    cell.appendChild(wrapper);
  });
  row.appendChild(cell);
}

function appendDeleteCell(row, onDelete) {
  const cell = document.createElement('td');
  const button = document.createElement('button');
  button.type = 'button';
  button.textContent = '删除';
  button.addEventListener('click', onDelete);
  cell.appendChild(button);
  row.appendChild(cell);
}

async function loadUsers() {
  const users = await fetchJSON('/api/admin/users');
  const userList = Array.isArray(users) ? users : [];
  const rows = document.getElementById('userRows');
  const fragment = document.createDocumentFragment();
  userList.forEach(u => {
    const row = document.createElement('tr');
    const token = u.token || '';
    appendTextCell(row, u.name);
    appendTextCell(row, u.email || '');
    appendCodeCell(row, token);
    appendTextCell(row, u.uuid);
    appendTextCell(row, u.expire || '');
    appendSubscriptionCell(row, token);
    appendDeleteCell(row, () => delUser(token));
    fragment.appendChild(row);
  });
  rows.replaceChildren(fragment);
}

async function loadNodes() {
  const nodes = await fetchJSON('/api/admin/nodes');
  const nodeList = Array.isArray(nodes) ? nodes : [];
  const rows = document.getElementById('nodeRows');
  const fragment = document.createDocumentFragment();
  nodeList.forEach(n => {
    const row = document.createElement('tr');
    const name = n.name || '';
    appendTextCell(row, name);
    appendTextCell(row, n.server);
    appendTextCell(row, n.port);
    appendDeleteCell(row, () => delNode(name));
    fragment.appendChild(row);
  });
  rows.replaceChildren(fragment);
}

async function loadStats() {
  const s = await fetchJSON('/api/admin/stats');
  document.getElementById('stats').textContent = JSON.stringify(s, null, 2);
}

async function addUser() {
  const body = {
    name: document.getElementById('uName').value,
    email: document.getElementById('uEmail').value,
    uuid: document.getElementById('uUUID').value,
    expire: Number(document.getElementById('uExpire').value || 0),
  };
  const res = await fetch('/api/admin/users', {
    method: 'POST',
    credentials: 'same-origin',
    headers: writeHeaders(),
    body: JSON.stringify(body),
  });
  if (!res.ok) return alert(await res.text());
  document.getElementById('uName').value = '';
  document.getElementById('uEmail').value = '';
  document.getElementById('uUUID').value = '';
  document.getElementById('uExpire').value = '';
  await refreshAll();
}

async function delUser(token) {
  const res = await fetch('/api/admin/users?token=' + encodeURIComponent(token), {
    method: 'DELETE',
    credentials: 'same-origin',
    headers: { 'X-CSRF-Token': csrfToken },
  });
  if (!res.ok) return alert(await res.text());
  await refreshAll();
}

async function addNode() {
  const body = {
    name: document.getElementById('nName').value,
    server: document.getElementById('nServer').value,
    port: Number(document.getElementById('nPort').value),
  };
  const res = await fetch('/api/admin/nodes', {
    method: 'POST',
    credentials: 'same-origin',
    headers: writeHeaders(),
    body: JSON.stringify(body),
  });
  if (!res.ok) return alert(await res.text());
  document.getElementById('nName').value = '';
  document.getElementById('nServer').value = '';
  document.getElementById('nPort').value = '';
  await refreshAll();
}

async function delNode(name) {
  const res = await fetch('/api/admin/nodes?name=' + encodeURIComponent(name), {
    method: 'DELETE',
    credentials: 'same-origin',
    headers: { 'X-CSRF-Token': csrfToken },
  });
  if (!res.ok) return alert(await res.text());
  await refreshAll();
}

async function refreshAll() {
  try {
    await Promise.all([loadUsers(), loadNodes(), loadStats()]);
  } catch (err) {
    const msg = err && err.message ? err.message : String(err);
    if (msg.includes('unauthorized') || msg.includes('forbidden')) {
      showLogin('登录已失效，请重新输入 ADMIN_TOKEN');
      return;
    }
    alert('加载失败: ' + msg);
  }
}

document.getElementById('loginButton').addEventListener('click', login);
document.getElementById('logoutButton').addEventListener('click', logout);
document.getElementById('addUserButton').addEventListener('click', addUser);
document.getElementById('addNodeButton').addEventListener('click', addNode);
document.getElementById('loginToken').addEventListener('keydown', event => {
  if (event.key === 'Enter') {
    login();
  }
});

showLogin('');
fetch('/api/admin/logout', {
  method: 'POST',
  credentials: 'same-origin',
}).catch(() => {});

setInterval(() => {
  if (!document.getElementById('appView').classList.contains('hidden')) {
    loadStats().catch(() => {});
  }
}, 5000);
