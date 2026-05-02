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
      </div>
    `;
    servicesEl.appendChild(row);
  }
}

async function refresh() {
  const services = await api("/api/services");
  renderServices(services);
}

async function runAction(action, service) {
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
