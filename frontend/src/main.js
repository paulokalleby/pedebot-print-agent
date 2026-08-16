import './app.css';
import pedebotIcon from './assets/images/pedebot-icon.png';

import { Pair, Unpair, GetStatus, GetRecentJobs } from '../wailsjs/go/main/App';
import { EventsOn } from '../wailsjs/runtime/runtime';

document.querySelector('#app').innerHTML = `
  <div class="wrap">
    <header class="brand">
      <img src="${pedebotIcon}" alt="PedeBot" class="brand-icon" />
      <h1>PedeBot Print Agent</h1>
    </header>

    <div class="card status-card">
      <div class="status">
        <span id="dot" class="dot offline"></span>
        <span id="status-text">Não pareado</span>
        <button id="retry-btn" class="btn btn-outline btn-sm" style="display: none;">Parear novamente</button>
      </div>

      <div id="pairing-box">
        <label for="code">Código de pareamento</label>
        <div class="pairing-row">
          <input id="code" type="text" placeholder="PDB-XXXX-XXXX" autocomplete="off" />
          <button id="pair-btn" class="btn btn-primary">Conectar</button>
        </div>
        <div id="pair-error" class="error"></div>
      </div>
    </div>

    <h2>Fila de impressão</h2>
    <div id="queue-empty" class="empty-state">Nenhum trabalho de impressão ainda.</div>
    <ul id="queue" class="queue-list"></ul>

    <details class="events-details">
      <summary>Eventos</summary>
      <ul id="log"></ul>
    </details>
  </div>
`;

const dot = document.getElementById('dot');
const statusText = document.getElementById('status-text');
const retryBtn = document.getElementById('retry-btn');
const pairingBox = document.getElementById('pairing-box');
const codeInput = document.getElementById('code');
const pairBtn = document.getElementById('pair-btn');
const pairError = document.getElementById('pair-error');
const log = document.getElementById('log');
const queueList = document.getElementById('queue');
const queueEmpty = document.getElementById('queue-empty');

function appendLog(message) {
  const li = document.createElement('li');
  const time = new Date().toLocaleTimeString();
  li.textContent = `[${time}] ${message}`;
  log.prepend(li);
}

// "Parear novamente" fica visível sempre que o Agent não está
// conectado - inclusive já pareado, mas com o token inválido (estação
// revogada/excluída no painel) - a reconexão automática do Service
// nunca ia se resolver sozinha nesse caso, então o usuário precisa de
// uma saída manual pra voltar à tela de digitar um novo código.
function renderStatus(status) {
  if (!status.paired) {
    dot.className = 'dot offline';
    statusText.textContent = 'Não pareado';
    retryBtn.style.display = 'none';
    pairingBox.style.display = '';

    return;
  }

  if (status.connected) {
    dot.className = 'dot online';
    statusText.textContent = 'Conectado';
    statusText.title = '';
    retryBtn.style.display = 'none';
    pairingBox.style.display = 'none';
  } else {
    dot.className = 'dot offline';
    const text = status.last_error
      ? `Desconectado (${status.last_error})`
      : 'Desconectado';
    statusText.textContent = text;
    // O texto trunca com "..." na tela (ver CSS) - o erro completo (pode
    // ser bem longo, ex: falha de autenticação do canal Reverb) continua
    // acessível via tooltip nativo do navegador ao passar o mouse.
    statusText.title = text;
    retryBtn.style.display = '';
    pairingBox.style.display = 'none';
  }
}

// Fila de impressão - uma linha por PrintJob, atualizada no lugar
// (upsert por id) conforme os eventos chegam - igual a uma fila de
// impressão de verdade, não um log que só cresce.
const STATUS_LABELS = {
  queued: 'Na fila',
  printing: 'Imprimindo',
  printed: 'Impresso',
  failed: 'Falhou',
};

const jobs = new Map();

// job.label/job.error vêm do cupom do pedido (nome do cliente, endereço,
// etc - dado digitado pelo cliente final no WhatsApp) - nunca confiar
// nesse texto como HTML seguro antes de jogar em innerHTML.
function escapeHtml(value) {
  return String(value)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function renderQueue() {
  const entries = Array.from(jobs.values());
  queueEmpty.style.display = entries.length ? 'none' : '';
  queueList.innerHTML = entries
    .map((job) => {
      const time = new Date(job.updated_at).toLocaleTimeString();
      const statusLabel = STATUS_LABELS[job.status] || job.status;
      let detail = time;
      if (job.status === 'failed') {
        detail = job.will_retry
          ? `${time} - tentativa ${job.attempts}/5, tentando de novo`
          : `${time} - desistiu após ${job.attempts} tentativas`;
      }

      return `
        <li class="queue-item">
          <div class="queue-item-main">
            <span class="badge badge-${job.status}">${escapeHtml(statusLabel)}</span>
            <span class="queue-item-label">${escapeHtml(job.label)}</span>
          </div>
          <div class="queue-item-detail">${escapeHtml(detail)}</div>
          ${job.error ? `<div class="queue-item-error">${escapeHtml(job.error)}</div>` : ''}
        </li>
      `;
    })
    .join('');
}

function upsertJob(job) {
  jobs.set(job.id, job);
  renderQueue();
}

async function doPair(code) {
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
}

pairBtn.addEventListener('click', () => {
  const code = codeInput.value.trim();
  if (code) doPair(code);
});

retryBtn.addEventListener('click', async () => {
  retryBtn.disabled = true;
  try {
    const error = await Unpair();
    if (error) {
      appendLog(`Falha ao parear novamente: ${error}`);
      return;
    }
    appendLog('Pronto para um novo pareamento.');
    codeInput.value = '';
    pairingBox.style.display = '';
    retryBtn.style.display = 'none';
    codeInput.focus();
  } finally {
    retryBtn.disabled = false;
  }
});

EventsOn('status', (status) => {
  renderStatus(status);
  appendLog(`Status: paired=${status.paired} connected=${status.connected}`);
});

EventsOn('print-job', (job) => {
  upsertJob(job);
  appendLog(`Job ${job.label}: ${STATUS_LABELS[job.status] || job.status}`);
});

GetStatus().then(renderStatus);
GetRecentJobs().then((recent) => {
  (recent || []).forEach(upsertJob);
});
