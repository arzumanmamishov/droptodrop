package main

const adminPanelHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>DropToDrop Admin</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; background: #f5f6fa; color: #1e293b; }

/* Login */
.login-wrap { display: flex; justify-content: center; align-items: center; height: 100vh; background: #f5f6fa; }
.login-box { background: #fff; padding: 48px 40px; border-radius: 20px; box-shadow: 0 8px 32px rgba(0,0,0,0.08); width: 380px; text-align: center; }
.login-box img { width: 56px; height: 56px; margin-bottom: 16px; }
.login-box h1 { font-size: 24px; margin-bottom: 4px; }
.login-box p { color: #94a3b8; font-size: 14px; margin-bottom: 28px; }
.login-box input { width: 100%; padding: 12px 16px; border: 1.5px solid #e2e8f0; border-radius: 12px; font-size: 14px; margin-bottom: 16px; transition: border 0.2s; }
.login-box input:focus { outline: none; border-color: #1e40af; box-shadow: 0 0 0 3px rgba(30,64,175,0.1); }
.login-box button { width: 100%; padding: 12px; background: #111; color: #fff; border: none; border-radius: 12px; font-size: 14px; font-weight: 600; cursor: pointer; transition: background 0.2s; }
.login-box button:hover { background: #333; }
.login-error { color: #dc2626; font-size: 13px; margin-bottom: 12px; }

/* Layout */
.app { display: none; min-height: 100vh; }
.layout { display: flex; min-height: 100vh; }

/* Sidebar */
.sidebar { width: 240px; background: #fff; border-right: 1px solid #f0f0f0; padding: 20px 0; display: flex; flex-direction: column; position: fixed; height: 100vh; z-index: 10; }
.sidebar-logo { padding: 0 24px 20px; display: flex; align-items: center; gap: 10px; font-size: 18px; font-weight: 700; color: #111; }
.sidebar-logo img { width: 32px; height: 32px; }
.nav-item { padding: 10px 24px; font-size: 14px; color: #64748b; cursor: pointer; display: flex; align-items: center; gap: 10px; transition: all 0.15s; border-left: 3px solid transparent; }
.nav-item:hover { background: #f8fafc; color: #1e293b; }
.nav-item.active { background: #f0f4ff; color: #1e40af; font-weight: 600; border-left-color: #1e40af; }
.nav-icon { width: 18px; text-align: center; }

/* Main */
.main { flex: 1; margin-left: 240px; padding: 0; }
.topbar { padding: 24px 32px 16px; display: flex; justify-content: space-between; align-items: center; }
.greeting h2 { font-size: 22px; font-weight: 700; color: #111; }
.greeting p { font-size: 13px; color: #94a3b8; margin-top: 2px; }
.topbar-right { display: flex; align-items: center; gap: 12px; }
.topbar-date { font-size: 13px; color: #94a3b8; background: #f8fafc; padding: 6px 14px; border-radius: 8px; }
.btn-logout { padding: 6px 16px; background: #f1f5f9; color: #64748b; border: none; border-radius: 8px; font-size: 13px; cursor: pointer; }
.btn-logout:hover { background: #e2e8f0; }

.content { padding: 0 32px 32px; }

/* Stats */
.stats-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 24px; }
.stat-card { background: #fff; border-radius: 16px; padding: 20px 24px; display: flex; align-items: center; gap: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.04); }
.stat-icon { width: 44px; height: 44px; border-radius: 12px; display: flex; align-items: center; justify-content: center; font-size: 20px; }
.stat-icon.blue { background: #eff6ff; }
.stat-icon.green { background: #f0fdf4; }
.stat-icon.amber { background: #fffbeb; }
.stat-icon.red { background: #fef2f2; }
.stat-icon.purple { background: #faf5ff; }
.stat-info .stat-val { font-size: 24px; font-weight: 700; color: #111; }
.stat-info .stat-lbl { font-size: 12px; color: #94a3b8; margin-top: 2px; }
.stat-info .stat-change { font-size: 11px; font-weight: 600; margin-top: 2px; }
.stat-info .stat-change.up { color: #16a34a; }
.stat-info .stat-change.down { color: #dc2626; }

/* Cards */
.card { background: #fff; border-radius: 16px; box-shadow: 0 1px 3px rgba(0,0,0,0.04); margin-bottom: 20px; overflow: hidden; }
.card-head { padding: 16px 24px; font-weight: 600; font-size: 15px; display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid #f8fafc; }
.card-head .count { font-size: 12px; color: #94a3b8; font-weight: 400; background: #f1f5f9; padding: 2px 10px; border-radius: 10px; }
table { width: 100%; border-collapse: collapse; font-size: 13px; }
th { text-align: left; padding: 10px 20px; color: #94a3b8; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; font-weight: 500; }
td { padding: 12px 20px; border-top: 1px solid #f8fafc; }
tr:hover td { background: #fafbfc; }
.badge { padding: 3px 10px; border-radius: 20px; font-size: 11px; font-weight: 600; display: inline-block; }
.badge-green { background: #dcfce7; color: #166534; }
.badge-blue { background: #dbeafe; color: #1e40af; }
.badge-amber { background: #fef3c7; color: #92400e; }
.badge-red { background: #fee2e2; color: #991b1b; }
.badge-gray { background: #f1f5f9; color: #475569; }
.badge-purple { background: #f3e8ff; color: #7c3aed; }
.btn-sm { padding: 4px 12px; font-size: 11px; border-radius: 8px; cursor: pointer; font-weight: 600; border: none; transition: all 0.15s; }
.btn-danger { background: #fee2e2; color: #dc2626; border: 1px solid #fca5a5; }
.btn-danger:hover { background: #dc2626; color: #fff; }
.btn-success { background: #dcfce7; color: #166534; border: 1px solid #86efac; }
.btn-success:hover { background: #166534; color: #fff; }
.loading { text-align: center; padding: 60px; color: #94a3b8; font-size: 14px; }
</style>
</head>
<body>

<div id="login" class="login-wrap">
  <div class="login-box">
    <img src="/pngdrop.png" alt="DropToDrop">
    <h1>Welcome Back</h1>
    <p>Sign in to your admin dashboard</p>
    <div id="login-error" class="login-error" style="display:none"></div>
    <input type="password" id="pwd" placeholder="Enter password" onkeydown="if(event.key==='Enter')doLogin()">
    <button onclick="doLogin()">Sign In</button>
  </div>
</div>

<div id="app" class="app">
  <div class="layout">
    <div class="sidebar">
      <div class="sidebar-logo"><img src="/pngdrop.png" alt="">DropToDrop</div>
      <div id="nav"></div>
    </div>
    <div class="main">
      <div class="topbar">
        <div class="greeting"><h2 id="greeting-text">Good Morning!</h2><p>Here's what's happening with your platform</p></div>
        <div class="topbar-right">
          <span class="topbar-date" id="topbar-date"></span>
          <button class="btn-logout" onclick="logout()">Logout</button>
        </div>
      </div>
      <div class="content" id="content"></div>
    </div>
  </div>
</div>

<script>
let token = localStorage.getItem('admin_token');
const API = '/admin-panel/api';
const NAV = [
  {id:'Overview', icon:'📊', label:'Overview'},
  {id:'Shops', icon:'🏪', label:'Shops'},
  {id:'Orders', icon:'📦', label:'Orders'},
  {id:'Payouts', icon:'💳', label:'Payouts'},
  {id:'Revenue', icon:'💰', label:'Revenue'},
  {id:'Disputes', icon:'⚠️', label:'Disputes'},
  {id:'Subscriptions', icon:'📋', label:'Subscriptions'},
  {id:'Activity', icon:'🔄', label:'Activity'},
];
let currentTab = 'Overview';

function doLogin() {
  const pwd = document.getElementById('pwd').value;
  fetch('/admin-panel/login', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify({password: pwd}) })
    .then(r => r.json()).then(d => {
      if (d.token) { token = d.token; localStorage.setItem('admin_token', token); showApp(); }
      else { document.getElementById('login-error').textContent = 'Wrong password'; document.getElementById('login-error').style.display = 'block'; }
    });
}
function logout() { token = null; localStorage.removeItem('admin_token'); document.getElementById('app').style.display='none'; document.getElementById('login').style.display='flex'; }
function api(path) { return fetch(API + path, { headers: { 'X-Admin-Token': token } }).then(r => { if (r.status === 401) { logout(); throw new Error('unauthorized'); } return r.json(); }); }

function showApp() {
  document.getElementById('login').style.display = 'none';
  document.getElementById('app').style.display = 'block';
  var h = new Date().getHours();
  document.getElementById('greeting-text').textContent = h < 12 ? 'Good Morning! ☀️' : h < 18 ? 'Good Afternoon! 👋' : 'Good Evening! 🌙';
  document.getElementById('topbar-date').textContent = new Date().toLocaleDateString('en-US', { weekday: 'short', month: 'short', day: 'numeric', year: 'numeric' });
  renderNav(); loadTab(currentTab);
}

function renderNav() {
  document.getElementById('nav').innerHTML = NAV.map(n =>
    '<div class="nav-item '+(n.id===currentTab?'active':'')+'" onclick="loadTab(\''+n.id+'\')"><span class="nav-icon">'+n.icon+'</span>'+n.label+'</div>'
  ).join('');
}

function loadTab(tab) {
  currentTab = tab; renderNav();
  document.getElementById('content').innerHTML = '<div class="loading">Loading...</div>';
  if (tab === 'Overview') loadOverview();
  else if (tab === 'Shops') loadShops();
  else if (tab === 'Orders') loadOrders();
  else if (tab === 'Payouts') loadPayouts();
  else if (tab === 'Revenue') loadRevenue();
  else if (tab === 'Disputes') loadDisputes();
  else if (tab === 'Subscriptions') loadSubscriptions();
  else if (tab === 'Activity') loadActivity();
}

function badge(status) {
  var m = { pending:'amber', accepted:'blue', processing:'purple', fulfilled:'green', rejected:'red', cancelled:'red', paid:'green', payment_sent:'blue', disputed:'red', open:'amber', resolved:'green', closed:'gray', no_payout:'gray', success:'green', failure:'red', active:'green', suspended:'red', supplier:'blue', reseller:'purple', unset:'gray' };
  return '<span class="badge badge-'+(m[status]||'gray')+'">'+status+'</span>';
}

function statCard(icon, color, value, label) {
  return '<div class="stat-card"><div class="stat-icon '+color+'">'+icon+'</div><div class="stat-info"><div class="stat-val">'+value+'</div><div class="stat-lbl">'+label+'</div></div></div>';
}

function loadOverview() {
  api('/stats').then(d => {
    document.getElementById('content').innerHTML =
      '<div class="stats-row">' +
        statCard('🏪','blue', d.total_shops, 'Total Shops') +
        statCard('📦','amber', d.total_orders, 'Total Orders') +
        statCard('✅','green', d.fulfilled_orders, 'Fulfilled') +
        statCard('⏳','amber', d.pending_orders, 'Pending') +
      '</div>' +
      '<div class="stats-row">' +
        statCard('💰','green', '$'+(d.total_revenue||0).toFixed(2), 'Total Revenue') +
        statCard('💳','blue', '$'+(d.total_paid||0).toFixed(2), 'Paid Out') +
        statCard('📊','purple', d.active_listings, 'Active Listings') +
        statCard('⚠️','red', d.open_disputes, 'Open Disputes') +
      '</div>' +
      '<div class="stats-row">' +
        statCard('🏭','blue', d.suppliers, 'Suppliers') +
        statCard('🛒','purple', d.resellers, 'Resellers') +
        statCard('📥','green', d.total_imports, 'Total Imports') +
        statCard('❌','red', d.rejected_orders, 'Rejected') +
      '</div>';
  });
}

function suspendShop(id, s) {
  var ns = s === 'active' ? 'suspended' : 'active';
  if (!confirm((ns==='suspended'?'SUSPEND':'ACTIVATE')+' this shop?')) return;
  fetch(API+'/shops/'+id+'/status', { method:'PUT', headers:{'Content-Type':'application/json','X-Admin-Token':token}, body:JSON.stringify({status:ns}) }).then(()=>loadShops());
}

function loadShops() {
  api('/shops').then(d => {
    var shops = d.shops||[];
    var html = '<div class="card"><div class="card-head">All Shops <span class="count">'+shops.length+'</span></div><table><tr><th>Shop</th><th>Role</th><th>Status</th><th>PayPal</th><th>Products</th><th>Orders</th><th>Joined</th><th>Action</th></tr>';
    shops.forEach(s => {
      var btn = s.status==='active'
        ? '<button class="btn-sm btn-danger" onclick="suspendShop(\''+s.id+'\',\'active\')">Suspend</button>'
        : '<button class="btn-sm btn-success" onclick="suspendShop(\''+s.id+'\',\'suspended\')">Activate</button>';
      html += '<tr><td><strong>'+s.domain+'</strong><br><span style="font-size:11px;color:#94a3b8">'+s.name+'</span></td><td>'+badge(s.role)+'</td><td>'+badge(s.status)+'</td><td style="font-size:12px;color:#64748b">'+(s.paypal||'—')+'</td><td>'+s.listing_count+' / '+s.import_count+'</td><td>'+s.order_count+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(s.created_at).toLocaleDateString()+'</td><td>'+btn+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

var orderSort = 'newest';
function loadOrders() {
  api('/orders').then(d => {
    var orders = (d.orders||[]).slice();
    orders.sort(function(a,b) { return orderSort==='newest' ? new Date(b.created_at)-new Date(a.created_at) : new Date(a.created_at)-new Date(b.created_at); });
    var today = new Date().toDateString();
    var todayOrders = orders.filter(function(o){return new Date(o.created_at).toDateString()===today;});
    var restOrders = orders.filter(function(o){return new Date(o.created_at).toDateString()!==today;});

    var html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px"><span style="font-size:16px;font-weight:700">Orders</span><button onclick="orderSort=orderSort===\'newest\'?\'oldest\':\'newest\';loadOrders()" class="btn-sm" style="background:#f1f5f9;color:#475569;border:1px solid #e2e8f0;padding:6px 14px;font-size:12px">'+(orderSort==='newest'?'↓ Newest':'↑ Oldest')+'</button></div>';

    if (todayOrders.length > 0) {
      html += '<div style="font-size:14px;font-weight:700;color:#1e293b;margin-bottom:10px">📅 Today\'s Orders <span style="font-size:11px;background:#dbeafe;color:#1e40af;padding:2px 8px;border-radius:10px">'+todayOrders.length+'</span></div>';
      html += '<div class="card"><table><tr><th>Order</th><th>Status</th><th>Amount</th><th>Customer</th><th>Flow</th><th>Payment</th><th>Fee</th><th>Time</th></tr>';
      todayOrders.forEach(function(o) { html += orderRow(o); });
      html += '</table></div>';
    }

    if (restOrders.length > 0) {
      html += '<div style="font-size:14px;font-weight:700;color:#64748b;margin:16px 0 10px">Previous Orders <span style="font-size:11px;background:#f1f5f9;color:#94a3b8;padding:2px 8px;border-radius:10px">'+restOrders.length+'</span></div>';
      html += '<div class="card"><table><tr><th>Order</th><th>Status</th><th>Amount</th><th>Customer</th><th>Flow</th><th>Payment</th><th>Fee</th><th>Date</th></tr>';
      restOrders.forEach(function(o) { html += orderRow(o); });
      html += '</table></div>';
    }

    if (orders.length === 0) html += '<div class="card" style="padding:40px;text-align:center;color:#94a3b8">No orders yet</div>';
    document.getElementById('content').innerHTML = html;
  });
}
function orderRow(o) {
  return '<tr><td><strong>#'+o.order_number+'</strong></td><td>'+badge(o.status)+'</td><td style="font-weight:600">$'+o.amount.toFixed(2)+' <span style="color:#94a3b8;font-weight:400">'+o.currency+'</span></td><td>'+o.customer+'</td><td style="font-size:12px">'+o.reseller+' → '+o.supplier+'</td><td>'+badge(o.pay_status)+'</td><td style="color:#1e40af;font-weight:600">$'+o.platform_fee.toFixed(2)+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(o.created_at).toLocaleDateString()+' '+new Date(o.created_at).toLocaleTimeString([],{hour:"2-digit",minute:"2-digit"})+'</td></tr>';
}

function loadPayouts() {
  api('/payouts').then(d => {
    var payouts = d.payouts||[];
    var html = '<div class="card"><div class="card-head">Payouts <span class="count">'+payouts.length+'</span></div><table><tr><th>Order</th><th>Status</th><th>Wholesale</th><th>Your Fee</th><th>Supplier Gets</th><th>Reseller → Supplier</th><th>Date</th></tr>';
    payouts.forEach(p => {
      html += '<tr><td><strong>#'+p.order_number+'</strong></td><td>'+badge(p.status)+'</td><td>$'+p.wholesale.toFixed(2)+'</td><td style="color:#1e40af;font-weight:600">$'+p.platform_fee.toFixed(2)+'</td><td style="color:#166534;font-weight:600">$'+p.supplier_payout.toFixed(2)+'</td><td style="font-size:12px">'+p.reseller+' → '+p.supplier+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(p.created_at).toLocaleDateString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

function loadRevenue() {
  api('/revenue').then(d => {
    var html = '<div class="stats-row">' +
      statCard('💰','blue', '$'+(d.total_revenue||0).toFixed(2), 'Total Volume') +
      statCard('🏦','green', '$'+(d.total_fees||0).toFixed(2), 'Your Platform Fees') +
      statCard('✅','green', '$'+(d.paid_fees||0).toFixed(2), 'Fees Collected') +
    '</div><div class="stats-row">' +
      statCard('⏳','amber', '$'+(d.pending_fees||0).toFixed(2), 'Fees Pending') +
      statCard('⚠️','red', '$'+(d.disputed_fees||0).toFixed(2), 'Fees Disputed') +
      statCard('💵','red', '$'+((d.total_fees||0)-(d.paid_fees||0)).toFixed(2), 'Still Owed to You') +
    '</div>';
    var shops = d.shop_breakdown||[];
    if (shops.length > 0) {
      html += '<div class="card"><div class="card-head">Revenue by Shop <span class="count">'+shops.length+'</span></div><table><tr><th>Shop</th><th>Role</th><th>Volume</th><th>Fees Generated</th><th>Collected</th><th>Pending</th><th>Owes You</th></tr>';
      shops.forEach(s => {
        html += '<tr><td><strong>'+s.domain+'</strong></td><td>'+badge(s.role)+'</td><td>$'+s.total_volume.toFixed(2)+'</td><td style="color:#1e40af;font-weight:600">$'+s.total_fees.toFixed(2)+'</td><td style="color:#166534">$'+s.paid_fees.toFixed(2)+'</td><td style="color:#92400e">$'+s.pending_fees.toFixed(2)+'</td><td style="font-weight:700;color:'+(s.owed>0?'#dc2626':'#166534')+'">$'+s.owed.toFixed(2)+'</td></tr>';
      });
      html += '</table></div>';
    }
    document.getElementById('content').innerHTML = html;
  });
}

var disputeTab = 'orders';
function loadDisputes() {
  api('/disputes').then(d => {
    var disputes = d.disputes||[];
    var appTypes = ['app_bug','payment_problem','account_issue','feature_request','policy_violation','app_other'];
    var appComplaints = disputes.filter(function(x){ return x.description.indexOf('[APP COMPLAINT]')===0 || appTypes.indexOf(x.type)>=0; });
    var orderDisputes = disputes.filter(function(x){ return appComplaints.indexOf(x)<0; });

    var html = '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">' +
      '<div style="display:flex;gap:4px">' +
        '<div class="tab '+(disputeTab==='orders'?'active':'')+'" onclick="disputeTab=\'orders\';loadDisputes()" style="padding:8px 20px;border-radius:8px;font-size:13px;font-weight:500;cursor:pointer;'+(disputeTab==='orders'?'background:#1e40af;color:#fff':'color:#64748b')+'">Order Disputes <span style="font-size:11px">('+ orderDisputes.length+')</span></div>' +
        '<div class="tab '+(disputeTab==='app'?'active':'')+'" onclick="disputeTab=\'app\';loadDisputes()" style="padding:8px 20px;border-radius:8px;font-size:13px;font-weight:500;cursor:pointer;'+(disputeTab==='app'?'background:#1e40af;color:#fff':'color:#64748b')+'">App Complaints <span style="font-size:11px">('+appComplaints.length+')</span></div>' +
      '</div></div>';

    var list = disputeTab === 'orders' ? orderDisputes : appComplaints;

    if (list.length > 0) {
      html += '<div class="card"><div class="card-head">'+(disputeTab==='orders'?'Order Disputes':'App Complaints')+' <span class="count">'+list.length+'</span></div><table><tr><th>Order</th><th>Type</th><th>Status</th><th>Reporter</th><th>Shop</th><th>Description</th><th>Resolution</th><th>Date</th></tr>';
      list.forEach(function(x) {
        var desc = x.description.replace('[APP COMPLAINT] ','');
        html += '<tr><td><strong>#'+x.order_number+'</strong></td><td>'+badge(x.type)+'</td><td>'+badge(x.status)+'</td><td>'+badge(x.reporter_role)+'</td><td style="font-size:12px">'+x.reporter_shop+'</td><td style="font-size:12px;max-width:250px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+desc+'</td><td style="font-size:12px">'+(x.resolution||'—')+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(x.created_at).toLocaleDateString()+' '+new Date(x.created_at).toLocaleTimeString([],{hour:"2-digit",minute:"2-digit"})+'</td></tr>';
      });
      html += '</table></div>';
    } else {
      html += '<div class="card" style="padding:40px;text-align:center;color:#94a3b8">No '+(disputeTab==='orders'?'order disputes':'app complaints')+' yet</div>';
    }

    document.getElementById('content').innerHTML = html;
  });
}

function loadSubscriptions() {
  api('/subscriptions').then(d => {
    var subs = d.subscriptions||[];
    var html = '<div class="card"><div class="card-head">Subscriptions <span class="count">'+subs.length+'</span></div><table><tr><th>Shop</th><th>Plan</th><th>Price</th><th>Status</th><th>Started</th><th>Period Ends</th></tr>';
    subs.forEach(function(s) {
      html += '<tr><td><strong>'+s.domain+'</strong></td><td>'+badge(s.plan_name)+'</td><td style="font-weight:600">$'+s.price.toFixed(2)+'/mo</td><td>'+badge(s.status)+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(s.created_at).toLocaleDateString()+'</td><td style="font-size:12px;color:#94a3b8">'+(s.period_end ? new Date(s.period_end).toLocaleDateString() : '—')+'</td></tr>';
    });
    if (subs.length === 0) html += '<tr><td colspan="6" style="text-align:center;color:#94a3b8;padding:24px">No subscriptions yet</td></tr>';
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

function loadActivity() {
  api('/activity').then(d => {
    var activity = d.activity||[];
    var html = '<div class="card"><div class="card-head">Recent Activity <span class="count">'+activity.length+'</span></div><table><tr><th>Action</th><th>Resource</th><th>Shop</th><th>Outcome</th><th>Time</th></tr>';
    activity.forEach(a => {
      html += '<tr><td>'+badge(a.action)+'</td><td style="color:#64748b">'+a.resource_type+'</td><td style="font-size:12px">'+a.shop+'</td><td>'+badge(a.outcome)+'</td><td style="font-size:12px;color:#94a3b8">'+new Date(a.created_at).toLocaleString()+'</td></tr>';
    });
    html += '</table></div>';
    document.getElementById('content').innerHTML = html;
  });
}

if (token) { showApp(); }
</script>
</body>
</html>`
