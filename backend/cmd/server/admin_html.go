package main

const adminPanelHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>DropToDrop Admin</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f0f4f8; color: #1e293b; }
.login-wrap { display: flex; justify-content: center; align-items: center; height: 100vh; }
.login-box { background: #fff; padding: 40px; border-radius: 16px; box-shadow: 0 4px 24px rgba(0,0,0,0.08); width: 360px; }
.login-box h1 { font-size: 22px; margin-bottom: 8px; }
.login-box p { color: #64748b; font-size: 14px; margin-bottom: 24px; }
.login-box input { width: 100%; padding: 10px 14px; border: 1px solid #e2e8f0; border-radius: 8px; font-size: 14px; margin-bottom: 16px; }
.login-box input:focus { outline: none; border-color: #1e40af; }
.login-box button { width: 100%; padding: 10px; background: #1e40af; color: #fff; border: none; border-radius: 8px; font-size: 14px; font-weight: 600; cursor: pointer; }
.login-box button:hover { background: #1e3a8a; }
.login-error { color: #dc2626; font-size: 13px; margin-bottom: 12px; }

.app { display: none; }
.header { background: linear-gradient(135deg, #0f172a, #1e3a8a); color: #fff; padding: 16px 32px; display: flex; justify-content: space-between; align-items: center; }
.header h1 { font-size: 20px; font-weight: 700; }
.header button { background: rgba(255,255,255,0.15); color: #fff; border: none; padding: 6px 16px; border-radius: 6px; cursor: pointer; font-size: 13px; }
.tabs { display: flex; gap: 4px; padding: 16px 32px; background: #fff; border-bottom: 1px solid #e2e8f0; }
.tab { padding: 8px 20px; border-radius: 8px; font-size: 13px; font-weight: 500; cursor: pointer; color: #64748b; border: 1px solid transparent; }
.tab:hover { background: #f1f5f9; }
.tab.active { background: #1e40af; color: #fff; }
.content { padding: 24px 32px; max-width: 1200px; margin: 0 auto; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 12px; margin-bottom: 24px; }
.stat { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; padding: 16px 20px; }
.stat-label { font-size: 12px; color: #64748b; margin-bottom: 4px; }
.stat-value { font-size: 28px; font-weight: 700; }
.stat-value.green { color: #166534; }
.stat-value.blue { color: #1e40af; }
.stat-value.red { color: #dc2626; }
.stat-value.amber { color: #92400e; }
.card { background: #fff; border: 1px solid #e2e8f0; border-radius: 12px; overflow: hidden; margin-bottom: 20px; }
.card-header { padding: 14px 20px; font-weight: 600; font-size: 15px; border-bottom: 1px solid #f1f5f9; }
table { width: 100%; border-collapse: collapse; font-size: 13px; }
th { text-align: left; padding: 10px 16px; color: #64748b; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; border-bottom: 1px solid #f1f5f9; }
td { padding: 10px 16px; border-bottom: 1px solid #f8fafc; }
tr:hover { background: #f8fafc; }
.badge { padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; display: inline-block; }
.badge-green { background: #dcfce7; color: #166534; }
.badge-blue { background: #dbeafe; color: #1e40af; }
.badge-amber { background: #fef3c7; color: #92400e; }
.badge-red { background: #fee2e2; color: #991b1b; }
.badge-gray { background: #f1f5f9; color: #475569; }
.loading { text-align: center; padding: 40px; color: #94a3b8; }
</style>
</head>
<body>

<div id="login" class="login-wrap">
  <div class="login-box">
    <h1>DropToDrop Admin</h1>
    <p>Enter admin password to access the dashboard.</p>
    <div id="login-error" class="login-error" style="display:none"></div>
    <input type="password" id="pwd" placeholder="Password" onkeydown="if(event.key==='Enter')doLogin()">
    <button onclick="doLogin()">Sign In</button>
  </div>
</div>

<div id="app" class="app">
  <div class="header">
    <h1>DropToDrop Admin</h1>
    <button onclick="logout()">Logout</button>
  </div>
  <div class="tabs" id="tabs"></div>
  <div class="content" id="content"></div>
</div>

<script>
let token = localStorage.getItem('admin_token');
const API = '/admin-panel/api';
const TABS = ['Overview','Shops','Orders','Payouts','Disputes','Activity'];
let currentTab = 'Overview';

function doLogin() {
  const pwd = document.getElementById('pwd').value;
  fetch('/admin-panel/login', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({password: pwd}) })
    .then(r => r.json())
    .then(d => {
      if (d.token) { token = d.token; localStorage.setItem('admin_token', token); showApp(); }
      else { document.getElementById('login-error').textContent = 'Wrong password'; document.getElementById('login-error').style.display = 'block'; }
    });
}
function logout() { token = null; localStorage.removeItem('admin_token'); document.getElementById('app').style.display='none'; document.getElementById('login').style.display='flex'; }
function api(path) { return fetch(API + path, { headers: { 'X-Admin-Token': token } }).then(r => { if (r.status === 401) { logout(); throw new Error('unauthorized'); } return r.json(); }); }

function showApp() {
  document.getElementById('login').style.display = 'none';
  document.getElementById('app').style.display = 'block';
  renderTabs();
  loadTab(currentTab);
}

function renderTabs() {
  document.getElementById('tabs').innerHTML = TABS.map(t => '<div class="tab '+(t===currentTab?'active':'')+'" onclick="loadTab(\''+t+'\')">'+t+'</div>').join('');
}

function loadTab(tab) {
  currentTab = tab;
  renderTabs();
  document.getElementById('content').innerHTML = '<div class="loading">Loading...</div>';
  if (tab === 'Overview') loadOverview();
  else if (tab === 'Shops') loadShops();
  else if (tab === 'Orders') loadOrders();
  else if (tab === 'Payouts') loadPayouts();
  else if (tab === 'Disputes') loadDisputes();
  else if (tab === 'Activity') loadActivity();
}

function badge(status) {
  const m = { pending:'amber', accepted:'blue', processing:'blue', fulfilled:'green', rejected:'red', cancelled:'red', paid:'green', payment_sent:'blue', disputed:'red', open:'amber', resolved:'green', closed:'gray', no_payout:'gray', success:'green', failure:'red' };
  return '<span class="badge badge-'+(m[status]||'gray')+'">'+status+'</span>';
}

function loadOverview() {
  api('/stats').then(d => {
    document.getElementById('content').innerHTML = '<div class="stats-grid">' +
      stat('Total Shops', d.total_shops) + stat('Suppliers', d.suppliers, 'blue') + stat('Resellers', d.resellers, 'blue') + stat('Active Listings', d.active_listings) +
      stat('Total Orders', d.total_orders) + stat('Pending', d.pending_orders, 'amber') + stat('Fulfilled', d.fulfilled_orders, 'green') + stat('Rejected', d.rejected_orders, 'red') +
      stat('Total Revenue', '$'+d.total_revenue.toFixed(2)) + stat('Paid Out', '$'+d.total_paid.toFixed(2), 'green') + stat('Pending Payouts', '$'+d.total_pending.toFixed(2), 'amber') +
      stat('Total Imports', d.total_imports) + stat('Total Disputes', d.total_disputes) + stat('Open Disputes', d.open_disputes, d.open_disputes > 0 ? 'red' : '') +
    '</div>';
  });
}
function stat(label, value, color) {
  return '<div class="stat"><div class="stat-label">'+label+'</div><div class="stat-value '+(color||'')+'">'+value+'</div></div>';
}

function loadShops() {
  api('/shops').then(d => {
    let html = '<div class="card"><div class="card-header">All Shops ('+((d.shops||[]).length)+')</div><table><tr><th>Domain</th><th>Name</th><th>Role</th><th>Status</th><th>PayPal</th><th>Listings</th><th>Imports</th><th>Orders</th><th>Joined</th></tr>';
    (d.shops||[]).forEach(s => {
      html += '<tr><td><strong>'+s.domain+'</strong></td><td>'+s.name+'</td><td>'+badge(s.role)+'</td><td>'+badge(s.status)+'</td><td style="font-size:12px;color:#64748b">'+s.paypal+'</td><td>'+s.listing_count+'</td><td>'+s.import_count+'</td><td>'+s.order_count+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(s.created_at).toLocaleDateString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

function loadOrders() {
  api('/orders').then(d => {
    let html = '<div class="card"><div class="card-header">Orders ('+((d.orders||[]).length)+')</div><table><tr><th>Order</th><th>Status</th><th>Amount</th><th>Customer</th><th>Reseller</th><th>Supplier</th><th>Payment</th><th>Fee</th><th>Payout</th><th>Date</th></tr>';
    (d.orders||[]).forEach(o => {
      html += '<tr><td><strong>#'+o.order_number+'</strong></td><td>'+badge(o.status)+'</td><td><strong>$'+o.amount.toFixed(2)+'</strong> '+o.currency+'</td><td>'+o.customer+'</td><td style="font-size:12px">'+o.reseller+'</td><td style="font-size:12px">'+o.supplier+'</td><td>'+badge(o.pay_status)+'</td><td style="color:#64748b">$'+o.platform_fee.toFixed(2)+'</td><td style="color:#166534">$'+o.supplier_payout.toFixed(2)+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(o.created_at).toLocaleDateString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

function loadPayouts() {
  api('/payouts').then(d => {
    let html = '<div class="card"><div class="card-header">Payouts ('+((d.payouts||[]).length)+')</div><table><tr><th>Order</th><th>Status</th><th>Wholesale</th><th>Fee</th><th>Supplier Payout</th><th>Reseller</th><th>Supplier</th><th>Date</th></tr>';
    (d.payouts||[]).forEach(p => {
      html += '<tr><td><strong>#'+p.order_number+'</strong></td><td>'+badge(p.status)+'</td><td>$'+p.wholesale.toFixed(2)+'</td><td style="color:#64748b">$'+p.platform_fee.toFixed(2)+'</td><td style="color:#166534;font-weight:600">$'+p.supplier_payout.toFixed(2)+'</td><td style="font-size:12px">'+p.reseller+'</td><td style="font-size:12px">'+p.supplier+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(p.created_at).toLocaleDateString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

function loadDisputes() {
  api('/disputes').then(d => {
    let html = '<div class="card"><div class="card-header">Disputes ('+((d.disputes||[]).length)+')</div><table><tr><th>Order</th><th>Type</th><th>Status</th><th>Reporter</th><th>Shop</th><th>Description</th><th>Resolution</th><th>Date</th></tr>';
    (d.disputes||[]).forEach(x => {
      html += '<tr><td><strong>#'+x.order_number+'</strong></td><td>'+badge(x.type)+'</td><td>'+badge(x.status)+'</td><td>'+badge(x.reporter_role)+'</td><td style="font-size:12px">'+x.reporter_shop+'</td><td style="font-size:12px;max-width:200px;overflow:hidden;text-overflow:ellipsis">'+x.description+'</td><td style="font-size:12px">'+x.resolution+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(x.created_at).toLocaleDateString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

function loadActivity() {
  api('/activity').then(d => {
    let html = '<div class="card"><div class="card-header">Recent Activity ('+((d.activity||[]).length)+')</div><table><tr><th>Action</th><th>Resource</th><th>Shop</th><th>Outcome</th><th>Time</th></tr>';
    (d.activity||[]).forEach(a => {
      html += '<tr><td>'+badge(a.action)+'</td><td>'+a.resource_type+'</td><td style="font-size:12px">'+a.shop+'</td><td>'+badge(a.outcome)+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(a.created_at).toLocaleString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

// Auto-login if token exists
if (token) { showApp(); }
</script>
</body>
</html>`
