import './app.css';

import { Pair, GetStatus } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

document.querySelector('#app').innerHTML = `
  <div class="wrap">
    <h1>PedeBot Print Agent</h1>

    <div class="status">
      <span id="dot" class="dot offline"></span>
      <span id="status-text">Não pareado</span>
    </div>

    <div id="pairing-box">
      <label for="code">Código de pareamento</label>
      <input id="code" type="text" placeholder="PDB-XXXX-XXXX" autocomplete="off" />
      <button id="pair-btn">Conectar</button>
      <div id="pair-error" class="error"></div>
    </div>

    <h2>Eventos</h2>
    <ul id="log"></ul>
  </div>
`;

const dot = document.getElementById('dot');
const statusText = document.getElementById('status-text');
const pairingBox = document.getElementById('pairing-box');
const codeInput = document.getElementById('code');
const pairBtn = document.getElementById('pair-btn');
const pairError = document.getElementById('pair-error');
const log = document.getElementById('log');

function appendLog(message) {
  const li = document.createElement('li');
  const time = new Date().toLocaleTimeString();
  li.textContent = `[${time}] ${message}`;
  log.prepend(li);
}

function renderStatus(status) {
  if (!status.paired) {
    dot.className = 'dot offline';
    statusText.textContent = 'Não pareado';
    pairingBox.style.display = '';

    return;
  }

  pairingBox.style.display = 'none';

  if (status.connected) {
    dot.className = 'dot online';
    statusText.textContent = 'Conectado';
  } else {
    dot.className = 'dot offline';
    statusText.textContent = status.last_error
      ? `Desconectado (${status.last_error})`
      : 'Desconectado';
  }
}

pairBtn.addEventListener('click', async () => {
  const code = codeInput.value.trim();
  if (!code) return;

  pairError.textContent = '';
  pairBtn.disabled = true;

  try {
    const error = await Pair(code);
    if (error) {
      pairError.textContent = error;
      appendLog(`Falha no pareamento: ${error}`);
    } else {
      appendLog('Pareado com sucesso.');
    }
  } catch (err) {
    pairError.textContent = String(err);
  } finally {
    pairBtn.disabled = false;
  }
});

EventsOn('status', (status) => {
  renderStatus(status);
  appendLog(`Status: paired=${status.paired} connected=${status.connected}`);
});

GetStatus().then(renderStatus);
