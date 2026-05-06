const servicesEl = document.querySelector("#services");
const activityEl = document.querySelector("#activity");
const refreshButton = document.querySelector("#refresh");

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "accept": "application/json" },
    ...options
  });
  if (!response.ok) {
    throw new Error(`${response.status} ${response.statusText}`);
  }
  return response.json();
}

function renderServices(services) {
  servicesEl.innerHTML = "";
  for (const service of services) {
    const row = document.createElement("article");
    row.className = "service-row";
    row.innerHTML = `
      <div>
        <strong>${service.name}</strong>
        <span>${service.language}</span>
      </div>
      <div>${service.git?.branch ?? "unknown"}</div>
      <div>${service.runtime?.container ?? "unknown"}</div>
      <div>${service.health ?? "unknown"}</div>
      <div class="actions">
        <button data-action="deploy" data-service="${service.name}">Deploy</button>
        <button data-action="debug" data-service="${service.name}">Debug</button>
        <button data-action="logs" data-service="${service.name}">Logs</button>
        <button data-action="config" data-service="${service.name}">Config</button>
      </div>
    `;
    servicesEl.appendChild(row);
  }
}

function renderConfig(config) {
  const envRows = (config.env || []).map((entry) => `
    <tr>
      <td>${escapeHTML(entry.name)}</td>
      <td>${escapeHTML(entry.value)}</td>
      <td>${escapeHTML(entry.source)}</td>
      <td>${entry.secret ? "yes" : "no"}</td>
    </tr>
  `).join("");
  const fileRows = (config.files || []).map((file) => `
    <tr>
      <td>${escapeHTML(file.target)}</td>
      <td>${escapeHTML(file.source)}</td>
      <td>${escapeHTML(file.mode)}</td>
      <td>${file.secret ? "yes" : "no"}</td>
    </tr>
  `).join("");
  const volumeRows = (config.volumes || []).map((volume) => `
    <tr>
      <td>${escapeHTML(volume.target)}</td>
      <td>${escapeHTML(volume.source)}</td>
      <td>${escapeHTML(volume.type)}</td>
    </tr>
  `).join("");
  activityEl.innerHTML = `
    <div class="config-view">
      <h3>${escapeHTML(config.service)} config</h3>
      <h4>Env</h4>
      <table><thead><tr><th>Name</th><th>Value</th><th>Source</th><th>Secret</th></tr></thead><tbody>${envRows}</tbody></table>
      <h4>Files</h4>
      <table><thead><tr><th>Target</th><th>Source</th><th>Mode</th><th>Secret</th></tr></thead><tbody>${fileRows}</tbody></table>
      <h4>Volumes</h4>
      <table><thead><tr><th>Target</th><th>Source</th><th>Type</th></tr></thead><tbody>${volumeRows}</tbody></table>
      <h4>Depends On</h4>
      <pre>${escapeHTML((config.dependsOn || []).join("\n"))}</pre>
    </div>
  `;
}

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

async function refresh() {
  const services = await api("/api/services");
  renderServices(services);
}

async function runAction(action, service) {
  if (action === "config") {
    const config = await api(`/api/services/${service}/config`);
    renderConfig(config);
    return;
  }
  if (action === "logs") {
    const logs = await api(`/api/services/${service}/logs?tail=200`);
    activityEl.textContent = logs.lines.join("\n");
    return;
  }
  const result = await api(`/api/services/${service}/${action}`, { method: "POST" });
  activityEl.textContent = JSON.stringify(result, null, 2);
}

servicesEl.addEventListener("click", (event) => {
  const button = event.target.closest("button[data-action]");
  if (!button) return;
  runAction(button.dataset.action, button.dataset.service).catch((error) => {
    activityEl.textContent = error.message;
  });
});

refreshButton.addEventListener("click", () => refresh().catch((error) => {
  activityEl.textContent = error.message;
}));

refresh().catch((error) => {
  activityEl.textContent = error.message;
});
