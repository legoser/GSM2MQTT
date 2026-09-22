package api

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GSM2MQTT Control Center</title>
<style>
:root {
  --primary: #1976d2; --primary-hover: #1565c0;
  --bg: #f4f6f9; --card-bg: #ffffff;
  --text: #212529; --text-muted: #6c757d;
  --success: #2e7d32; --warning: #ed6c02; --danger: #d32f2f;
  --border: #e0e0e0;
}
* { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
body { background: var(--bg); color: var(--text); padding: 1.5rem; }
.header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; flex-wrap: wrap; gap: 1rem; }
.header h1 { font-size: 1.75rem; color: var(--primary); display: flex; align-items: center; gap: 0.5rem; }
.gateway-badge { background: #e8f5e9; color: var(--success); padding: 0.35rem 0.75rem; border-radius: 999px; font-weight: 600; font-size: 0.875rem; border: 1px solid #c8e6c9; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 1.5rem; margin-bottom: 1.5rem; }
.card { background: var(--card-bg); border-radius: 10px; padding: 1.25rem; box-shadow: 0 1px 3px rgba(0,0,0,0.08); border: 1px solid var(--border); }
.card h2 { font-size: 1.15rem; margin-bottom: 1rem; color: var(--primary); display: flex; align-items: center; gap: 0.5rem; }
table { width: 100%; border-collapse: collapse; text-align: left; }
th, td { padding: 0.75rem; border-bottom: 1px solid var(--border); font-size: 0.9rem; }
th { background: #fafafa; color: var(--text-muted); font-weight: 600; }
.badge { display: inline-block; padding: 0.25rem 0.6rem; border-radius: 4px; font-size: 0.75rem; font-weight: bold; text-transform: uppercase; }
.badge-ready { background: #e8f5e9; color: var(--success); }
.badge-error { background: #ffebee; color: var(--danger); }
.badge-warning { background: #fff3e0; color: var(--warning); }
.sig-bar-container { background: #e0e0e0; border-radius: 4px; height: 8px; width: 80px; display: inline-block; vertical-align: middle; margin-right: 6px; overflow: hidden; }
.sig-bar { height: 100%; border-radius: 4px; transition: width 0.3s; }
.form-group { margin-bottom: 0.85rem; }
label { display: block; font-size: 0.85rem; font-weight: 600; margin-bottom: 0.3rem; color: var(--text-muted); }
input, select, textarea { width: 100%; padding: 0.55rem 0.75rem; border: 1px solid var(--border); border-radius: 6px; font-size: 0.9rem; }
input:focus, select:focus, textarea:focus { outline: none; border-color: var(--primary); }
.btn-group { display: flex; gap: 0.5rem; }
button { flex: 1; padding: 0.65rem 1rem; border: none; border-radius: 6px; font-weight: 600; cursor: pointer; transition: background 0.2s; font-size: 0.9rem; }
.btn-primary { background: var(--primary); color: #fff; }
.btn-primary:hover { background: var(--primary-hover); }
.btn-danger { background: var(--danger); color: #fff; }
.btn-danger:hover { background: #b71c1c; }
.btn-secondary { background: #e0e0e0; color: #333; }
.btn-secondary:hover { background: #d5d5d5; }
.quick-chips { display: flex; gap: 0.4rem; margin-top: 0.3rem; margin-bottom: 0.6rem; flex-wrap: wrap; }
.chip { padding: 0.2rem 0.5rem; background: #e3f2fd; color: #1565c0; border-radius: 4px; font-size: 0.8rem; cursor: pointer; user-select: none; }
.chip:hover { background: #bbdefb; }
.res-box { background: #f8f9fa; border: 1px solid var(--border); border-radius: 6px; padding: 0.75rem; font-family: monospace; font-size: 0.85rem; margin-top: 0.75rem; min-height: 48px; max-height: 120px; overflow-y: auto; white-space: pre-wrap; word-break: break-all; }
.inbox-table { max-height: 250px; overflow-y: auto; }
</style>
</head>
<body>
<div class="header">
  <h1><span>📟</span> GSM2MQTT Control Center</h1>
  <div>
    <span class="gateway-badge">● Gateway Online</span>
    <button onclick="refreshAll()" style="margin-left:8px; padding:0.35rem 0.75rem;" class="btn-secondary">↻ Refresh</button>
  </div>
</div>

<div class="card" style="margin-bottom: 1.5rem;">
  <h2><span>📡</span> Modems Status & Telemetry</h2>
  <div style="overflow-x: auto;">
    <table id="modemsTable">
      <thead>
        <tr>
          <th>Modem ID</th><th>Type</th><th>Status</th><th>Signal Strength</th><th>Operator</th><th>SIM Status</th><th>Balance</th>
        </tr>
      </thead>
      <tbody id="modemsBody">
        <tr><td colspan="7" style="text-align:center; color:#888;">Loading telemetry...</td></tr>
      </tbody>
    </table>
  </div>
</div>

<div class="grid">
  <!-- USSD Commands -->
  <div class="card">
    <h2><span>⚡</span> USSD Command & Balance</h2>
    <div class="form-group">
      <label>Modem:</label><select id="ussdModem"></select>
    </div>
    <div class="form-group">
      <label>USSD Code:</label>
      <input type="text" id="ussdCode" value="*100#" placeholder="*100#">
      <div class="quick-chips">
        <span class="chip" onclick="setUSSD('*100#')">*100# (МТС/Tele2)</span>
        <span class="chip" onclick="setUSSD('*105#')">*105# (Мегафон)</span>
        <span class="chip" onclick="setUSSD('*102#')">*102# (Билайн)</span>
      </div>
    </div>
    <button class="btn-primary" onclick="sendUSSD()">Execute USSD Query</button>
    <div id="ussdResult" class="res-box">USSD response will appear here...</div>
  </div>

  <!-- Voice Calls -->
  <div class="card">
    <h2><span>📞</span> Voice Call Testing</h2>
    <div class="form-group">
      <label>Modem:</label><select id="callModem"></select>
    </div>
    <div class="form-group">
      <label>Phone Number:</label>
      <input type="text" id="callNumber" placeholder="+79001234567">
    </div>
    <div class="btn-group">
      <button class="btn-primary" onclick="dialCall()">Dial Call</button>
      <button class="btn-danger" onclick="hangupCall()">Hangup</button>
    </div>
    <div id="callResult" class="res-box">Call status: Idle</div>
  </div>

  <!-- Send SMS -->
  <div class="card">
    <h2><span>✉️</span> Send SMS</h2>
    <div class="form-group">
      <label>Modem:</label><select id="smsModem"></select>
    </div>
    <div class="form-group">
      <label>Phone Number:</label>
      <input type="text" id="smsTo" placeholder="+79001234567">
    </div>
    <div class="form-group">
      <label>Message Text:</label>
      <textarea id="smsText" rows="2" placeholder="Test SMS from GSM2MQTT Dashboard"></textarea>
    </div>
    <button class="btn-primary" onclick="sendSMS()">Send Message</button>
    <div id="smsResult" class="res-box">SMS dispatch status...</div>
  </div>
</div>

<div class="card">
  <h2><span>📥</span> Received SMS Inbox</h2>
  <div class="inbox-table">
    <table>
      <thead>
        <tr><th>Time</th><th>From</th><th>Modem</th><th>Message</th></tr>
      </thead>
      <tbody id="inboxBody">
        <tr><td colspan="4" style="text-align:center; color:#888;">No messages received yet</td></tr>
      </tbody>
    </table>
  </div>
</div>

<script>
function setUSSD(c) { document.getElementById('ussdCode').value = c; }

function getSignalHTML(rssi, dbm) {
  let pct = Math.min(100, Math.round((rssi / 31) * 100));
  let color = pct > 50 ? '#2e7d32' : pct > 25 ? '#ed6c02' : '#d32f2f';
  return '<div class="sig-bar-container"><div class="sig-bar" style="width:'+pct+'%; background:'+color+'"></div></div>' +
         '<span>' + rssi + ' / 31 (' + dbm + ' dBm)</span>';
}

function getBadge(status) {
  let s = (status || 'unknown').toLowerCase();
  let cls = s === 'ready' ? 'badge-ready' : s.includes('error') ? 'badge-error' : 'badge-warning';
  return '<span class="badge ' + cls + '">' + (status || 'OFFLINE') + '</span>';
}

function updateModemSelects(modems) {
  ['ussdModem', 'callModem', 'smsModem'].forEach(id => {
    let sel = document.getElementById(id);
    let cur = sel.value;
    sel.innerHTML = modems.map(m => '<option value="' + m.id + '">' + m.id + ' (' + m.type + ')</option>').join('');
    if (cur) sel.value = cur;
  });
}

function fetchModems() {
  fetch('/api/modems')
    .then(r => r.json())
    .then(modems => {
      updateModemSelects(modems);
      let body = document.getElementById('modemsBody');
      if (!modems || modems.length === 0) {
        body.innerHTML = '<tr><td colspan="7" style="text-align:center;">No modems registered</td></tr>';
        return;
      }
      body.innerHTML = modems.map(m => {
        let h = m.health || {};
        return '<tr>' +
          '<td><strong>' + m.id + '</strong></td>' +
          '<td>' + m.type + '</td>' +
          '<td>' + getBadge(h.status) + '</td>' +
          '<td>' + getSignalHTML(h.signal || 0, h.signal_dbm || 0) + '</td>' +
          '<td>' + (h.operator || 'Unknown') + '</td>' +
          '<td>' + (h.sim || 'N/A') + '</td>' +
          '<td><strong>' + (m.balance !== undefined ? m.balance.toFixed(2) : '0.00') + ' ' + (m.currency || 'RUB') + '</strong></td>' +
        '</tr>';
      }).join('');
    })
    .catch(e => console.error('failed fetching modems', e));
}

function fetchInbox() {
  fetch('/api/sms/inbox')
    .then(r => r.json())
    .then(list => {
      let body = document.getElementById('inboxBody');
      if (!list || list.length === 0) {
        body.innerHTML = '<tr><td colspan="4" style="text-align:center; color:#888;">No messages received yet</td></tr>';
        return;
      }
      body.innerHTML = list.map(m => '<tr>' +
        '<td>' + m.timestamp + '</td>' +
        '<td><strong>' + m.sender + '</strong></td>' +
        '<td>' + m.modem_id + '</td>' +
        '<td>' + m.text + '</td>' +
      '</tr>').join('');
    })
    .catch(e => console.error('failed fetching inbox', e));
}

function sendUSSD() {
  let m = document.getElementById('ussdModem').value;
  let code = document.getElementById('ussdCode').value;
  let out = document.getElementById('ussdResult');
  out.innerText = 'Executing ' + code + '...';
  fetch('/api/ussd/send', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({modem_id: m, code: code})
  })
  .then(r => r.json())
  .then(d => { out.innerText = d.message || JSON.stringify(d); fetchModems(); })
  .catch(e => { out.innerText = 'Error: ' + e; });
}

function dialCall() {
  let m = document.getElementById('callModem').value;
  let num = document.getElementById('callNumber').value;
  let out = document.getElementById('callResult');
  out.innerText = 'Dialing ' + num + '...';
  fetch('/api/call/dial', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({modem_id: m, number: num})
  })
  .then(r => r.json())
  .then(d => { out.innerText = d.success ? 'Calling ' + num + ' (Active)' : d.error; })
  .catch(e => { out.innerText = 'Dial error: ' + e; });
}

function hangupCall() {
  let m = document.getElementById('callModem').value;
  let out = document.getElementById('callResult');
  fetch('/api/call/hangup', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({modem_id: m})
  })
  .then(r => r.json())
  .then(d => { out.innerText = 'Call terminated (Idle)'; })
  .catch(e => { out.innerText = 'Hangup error: ' + e; });
}

function sendSMS() {
  let m = document.getElementById('smsModem').value;
  let to = document.getElementById('smsTo').value;
  let txt = document.getElementById('smsText').value;
  let out = document.getElementById('smsResult');
  out.innerText = 'Sending SMS...';
  fetch('/api/sms/send', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({modem_id: m, to: to, text: txt})
  })
  .then(r => r.json())
  .then(d => { out.innerText = d.success ? 'SMS Sent! Ref IDs: ' + JSON.stringify(d.refs) : d.error; })
  .catch(e => { out.innerText = 'Send error: ' + e; });
}

function refreshAll() {
  fetchModems();
  fetchInbox();
}

setInterval(refreshAll, 3000);
refreshAll();
</script>
</body>
</html>`
