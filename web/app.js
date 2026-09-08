"use strict";

// ── tiny helpers ─────────────────────────────────────────────────────────

async function api(path, options) {
  const res = await fetch(path, options);
  const isJson = (res.headers.get("content-type") || "").includes("application/json");
  const body = isJson ? await res.json() : null;
  if (!res.ok) {
    const message = (body && body.message) || `request failed with status ${res.status}`;
    throw new Error(message);
  }
  return body;
}

function toast(message, isError) {
  const el = document.getElementById("toast");
  el.textContent = message;
  el.classList.toggle("error", !!isError);
  el.classList.add("show");
  clearTimeout(toast._t);
  toast._t = setTimeout(() => el.classList.remove("show"), 3200);
}

function el(tag, attrs, children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (k === "text") node.textContent = v;
    else if (k.startsWith("on")) node.addEventListener(k.slice(2), v);
    else node.setAttribute(k, v);
  }
  for (const child of children || []) node.appendChild(child);
  return node;
}

function fmtTime(iso) {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: "short", day: "numeric", hour: "numeric", minute: "2-digit",
    });
  } catch { return iso; }
}

// ── tabs ─────────────────────────────────────────────────────────────────

document.getElementById("tabs").addEventListener("click", (e) => {
  const btn = e.target.closest(".tab");
  if (!btn) return;
  document.querySelectorAll(".tab").forEach((t) => t.classList.toggle("active", t === btn));
  document.querySelectorAll(".panel").forEach((p) =>
    p.classList.toggle("active", p.id === `panel-${btn.dataset.tab}`)
  );
});

// ── health ───────────────────────────────────────────────────────────────

async function refreshHealth() {
  const dot = document.getElementById("statusDot");
  const text = document.getElementById("statusText");
  try {
    const h = await api("/health");
    const dbOk = h.checks && h.checks.database === "up";
    dot.className = "dot " + (dbOk ? "up" : "down");
    text.textContent = `${h.status} · db ${h.checks.database} · redis ${h.checks.redis}`;
  } catch {
    dot.className = "dot down";
    text.textContent = "unreachable";
  }
}

// ── appliances (shared state) ───────────────────────────────────────────

let appliances = [];

function populateApplianceSelects() {
  const selects = [
    document.getElementById("calcAppliance"),
    document.getElementById("multiAppliance"),
    document.getElementById("schedAppliance"),
  ];
  for (const sel of selects) {
    const current = sel.value;
    sel.innerHTML = "";
    for (const a of appliances) {
      sel.appendChild(el("option", { value: a.name, text: a.name }));
    }
    if (appliances.some((a) => a.name === current)) sel.value = current;
  }
}

function renderApplianceTable() {
  const tbody = document.querySelector("#applianceTable tbody");
  tbody.innerHTML = "";
  if (appliances.length === 0) {
    tbody.appendChild(el("tr", { class: "empty-row" }, [el("td", { colspan: "3", text: "No appliances yet." })]));
    return;
  }
  for (const a of appliances) {
    const delBtn = el("button", {
      class: "ghost",
      text: "Delete",
      onclick: () => deleteAppliance(a.id),
    });
    tbody.appendChild(el("tr", {}, [
      el("td", { text: a.name }),
      el("td", { class: "mono", text: `${a.durationHours}h` }),
      el("td", {}, [delBtn]),
    ]));
  }
}

async function loadAppliances() {
  appliances = await api("/appliances");
  populateApplianceSelects();
  renderApplianceTable();
}

async function deleteAppliance(id) {
  try {
    await api(`/appliances/${id}`, { method: "DELETE" });
    toast("Appliance deleted");
    await loadAppliances();
  } catch (err) {
    toast(err.message, true);
  }
}

document.getElementById("applianceForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const name = document.getElementById("newApplianceName").value.trim();
  const durationHours = parseFloat(document.getElementById("newApplianceDuration").value);
  try {
    await api("/appliances", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, durationHours }),
    });
    e.target.reset();
    toast(`Added ${name}`);
    await loadAppliances();
  } catch (err) {
    toast(err.message, true);
  }
});

// ── calculate ────────────────────────────────────────────────────────────

document.getElementById("calcForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const appliance = document.getElementById("calcAppliance").value;
  const zip = document.getElementById("calcZip").value.trim();
  const area = document.getElementById("calcResult");
  area.innerHTML = "";
  try {
    const w = await api(`/calculate?appliance=${encodeURIComponent(appliance)}&zip=${encodeURIComponent(zip)}`);
    area.appendChild(el("div", { class: "result-card" }, [
      el("div", { class: "headline", text: w.recommendation }),
      el("div", { class: "metric-row" }, [
        el("div", { class: "metric" }, [el("span", { class: "label", text: "Start" }), el("span", { class: "value", text: fmtTime(w.startTime) })]),
        el("div", { class: "metric" }, [el("span", { class: "label", text: "End" }), el("span", { class: "value", text: fmtTime(w.endTime) })]),
        el("div", { class: "metric" }, [el("span", { class: "label", text: "Avg gCO₂/kWh" }), el("span", { class: "value accent", text: w.averageCarbonIntensity })]),
        el("div", { class: "metric" }, [el("span", { class: "label", text: "Provider" }), el("span", { class: "value", text: w.provider || "—" })]),
      ]),
    ]));
  } catch (err) {
    area.appendChild(el("div", { class: "error-card", text: err.message }));
  }
});

// ── multi-zone ───────────────────────────────────────────────────────────

document.getElementById("multiForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const appliance = document.getElementById("multiAppliance").value;
  const zips = document.getElementById("multiZips").value.trim();
  const area = document.getElementById("multiResult");
  area.innerHTML = "";
  try {
    const data = await api(`/calculate/multi?appliance=${encodeURIComponent(appliance)}&zips=${encodeURIComponent(zips)}`);
    const grid = el("div", { class: "zone-grid" });
    for (const r of data.results) {
      const isGreenest = data.greenest && r.window && r.window.averageCarbonIntensity === data.greenest.averageCarbonIntensity;
      const row = el("div", { class: "zone-row" + (isGreenest ? " best" : "") });
      row.appendChild(el("span", { class: "zip", text: r.zip }));
      if (r.window) {
        row.appendChild(el("span", { class: "zone-detail", text: fmtTime(r.window.startTime) }));
        row.appendChild(el("span", { class: "zone-value", text: `${r.window.averageCarbonIntensity} gCO₂/kWh` }));
        if (isGreenest) {
          row.appendChild(el("span", { class: "badge", text: "Greenest" }));
        }
      } else {
        row.appendChild(el("span", { class: "zone-detail", text: r.error }));
      }
      grid.appendChild(row);
    }
    area.appendChild(grid);
  } catch (err) {
    area.appendChild(el("div", { class: "error-card", text: err.message }));
  }
});

// ── schedules ────────────────────────────────────────────────────────────

function statusBadge(status) {
  return el("span", { class: `badge-status ${status}`, text: status });
}

async function loadSchedules() {
  const tbody = document.querySelector("#scheduleTable tbody");
  let runs;
  try {
    runs = await api("/schedules");
  } catch {
    return;
  }
  tbody.innerHTML = "";
  if (!runs || runs.length === 0) {
    tbody.appendChild(el("tr", { class: "empty-row" }, [el("td", { colspan: "6", text: "No scheduled runs yet." })]));
    return;
  }
  runs.sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt));
  for (const r of runs) {
    const cancelBtn = r.status === "pending"
      ? el("button", { class: "ghost", text: "Cancel", onclick: () => cancelSchedule(r.id) })
      : el("span", { text: "" });
    tbody.appendChild(el("tr", {}, [
      el("td", { text: r.applianceName }),
      el("td", { class: "mono", text: r.zip }),
      el("td", { class: "mono", text: fmtTime(r.startTime) }),
      el("td", { class: "mono", text: r.averageCarbonIntensity }),
      el("td", {}, [statusBadge(r.status)]),
      el("td", {}, [cancelBtn]),
    ]));
  }
}

async function cancelSchedule(id) {
  try {
    await api(`/schedules/${id}`, { method: "DELETE" });
    toast("Schedule cancelled");
    await loadSchedules();
  } catch (err) {
    toast(err.message, true);
  }
}

document.getElementById("scheduleForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const applianceName = document.getElementById("schedAppliance").value;
  const zip = document.getElementById("schedZip").value.trim();
  const webhookUrl = document.getElementById("schedWebhook").value.trim();
  const payload = { applianceName, zip };
  if (webhookUrl) payload.webhookUrl = webhookUrl;
  try {
    await api("/schedules", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    e.target.reset();
    toast("Scheduled");
    await loadSchedules();
  } catch (err) {
    toast(err.message, true);
  }
});

// ── boot ─────────────────────────────────────────────────────────────────

(async function init() {
  await refreshHealth();
  await loadAppliances();
  await loadSchedules();
  setInterval(refreshHealth, 10000);
  setInterval(loadSchedules, 15000);
})();
