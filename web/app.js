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

function fmtClock(iso) {
  try {
    return new Date(iso).toLocaleTimeString(undefined, {
      hour: "numeric", minute: "2-digit", timeZoneName: "short",
    });
  } catch { return iso; }
}

// When the appliance actually finishes. The window's endTime is rounded up to whole hourly
// slots (a 1.5h dishwasher gets a 2h window), so it overstates the finish time.
function finishTime(w) {
  const a = appliances.find((x) => x.name.toLowerCase() === w.applianceName.toLowerCase());
  if (!a) return w.endTime;
  return new Date(new Date(w.startTime).getTime() + a.durationHours * 3600000).toISOString();
}

// Built client-side rather than using the API's recommendation string, which is in UTC —
// so the headline matches the local-time times shown beneath it.
function recommendationText(w) {
  return `Run your ${w.applianceName} at ${fmtClock(w.startTime)} — avg. ` +
    `${Math.round(w.averageCarbonIntensity)} gCO₂/kWh (done by ${fmtClock(finishTime(w))}).`;
}

// Runs fn with the button disabled, so a double-click can't send the same request twice.
async function withBusy(button, fn) {
  if (button) button.disabled = true;
  try {
    await fn();
  } finally {
    if (button) button.disabled = false;
  }
}

function submitButton(form) {
  return form.querySelector('button[type="submit"]');
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
    const action = a.protected
      ? el("span", { class: "protected-label", text: "Built-in", title: "Seeded demo appliances can't be deleted" })
      : el("button", { class: "ghost", text: "Delete", onclick: (e) => deleteAppliance(a, e.currentTarget) });
    tbody.appendChild(el("tr", {}, [
      el("td", { text: a.name }),
      el("td", { class: "mono", text: `${a.durationHours}h` }),
      el("td", {}, [action]),
    ]));
  }
}

async function loadAppliances() {
  appliances = await api("/appliances");
  populateApplianceSelects();
  renderApplianceTable();
}

async function deleteAppliance(a, button) {
  if (!confirm(`Delete "${a.name}"? This can't be undone.`)) return;
  await withBusy(button, async () => {
    try {
      await api(`/appliances/${a.id}`, { method: "DELETE" });
      toast("Appliance deleted");
      await loadAppliances();
    } catch (err) {
      toast(err.message, true);
    }
  });
}

document.getElementById("applianceForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.target;
  const name = document.getElementById("newApplianceName").value.trim();
  const durationHours = parseFloat(document.getElementById("newApplianceDuration").value);
  await withBusy(submitButton(form), async () => {
    try {
      await api("/appliances", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, durationHours }),
      });
      form.reset();
      toast(`Added ${name}`);
      await loadAppliances();
    } catch (err) {
      toast(err.message, true);
    }
  });
});

// ── calculate ────────────────────────────────────────────────────────────

document.getElementById("calcForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const appliance = document.getElementById("calcAppliance").value;
  const zip = document.getElementById("calcZip").value.trim();
  const area = document.getElementById("calcResult");
  area.innerHTML = "";
  await withBusy(submitButton(e.target), async () => {
    try {
      const w = await api(`/calculate?appliance=${encodeURIComponent(appliance)}&zip=${encodeURIComponent(zip)}`);
      area.appendChild(el("div", { class: "result-card" }, [
        el("div", { class: "headline", text: recommendationText(w) }),
        el("div", { class: "metric-row" }, [
          el("div", { class: "metric" }, [el("span", { class: "label", text: "Start" }), el("span", { class: "value", text: fmtTime(w.startTime) })]),
          el("div", { class: "metric" }, [el("span", { class: "label", text: "Done by" }), el("span", { class: "value", text: fmtTime(finishTime(w)) })]),
          el("div", { class: "metric" }, [el("span", { class: "label", text: "Avg gCO₂/kWh" }), el("span", { class: "value accent", text: w.averageCarbonIntensity })]),
          el("div", { class: "metric" }, [el("span", { class: "label", text: "Provider" }), el("span", { class: "value", text: w.provider || "—" })]),
        ]),
      ]));
    } catch (err) {
      area.appendChild(el("div", { class: "error-card", text: err.message }));
    }
  });
});

// ── multi-zone ───────────────────────────────────────────────────────────

document.getElementById("multiForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const appliance = document.getElementById("multiAppliance").value;
  const zips = document.getElementById("multiZips").value.trim();
  const area = document.getElementById("multiResult");
  area.innerHTML = "";
  await withBusy(submitButton(e.target), async () => {
    try {
      const data = await api(`/calculate/multi?appliance=${encodeURIComponent(appliance)}&zips=${encodeURIComponent(zips)}`);
      const grid = el("div", { class: "zone-grid" });
      let badged = false; // on a tie, badge only the first greenest zip
      for (const r of data.results) {
        const isGreenest = !badged && data.greenest && r.window &&
          r.window.averageCarbonIntensity === data.greenest.averageCarbonIntensity;
        if (isGreenest) badged = true;
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
});

// ── schedules ────────────────────────────────────────────────────────────

function statusBadge(status) {
  return el("span", { class: `badge-status ${status}`, text: status });
}

// What happened to a run, so a failure always says why.
function runDetails(r) {
  if (r.status === "failed") {
    return `Failed after ${r.attemptCount} attempt${r.attemptCount === 1 ? "" : "s"}: ${r.lastError || "unknown error"}`;
  }
  if (r.status === "pending" && r.nextAttemptAt) {
    return `Retrying at ${fmtClock(r.nextAttemptAt)} (attempt ${r.attemptCount + 1}). Last error: ${r.lastError || "unknown"}`;
  }
  if (r.status === "fired" && r.firedAt) {
    return `Fired ${fmtTime(r.firedAt)}` + (r.attemptCount > 1 ? ` on attempt ${r.attemptCount}` : "");
  }
  if (r.status === "pending") return "Waiting for its start time";
  return "";
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
    tbody.appendChild(el("tr", { class: "empty-row" }, [el("td", { colspan: "7", text: "No scheduled runs yet." })]));
    return;
  }
  runs.sort((a, b) => new Date(b.createdAt) - new Date(a.createdAt));
  for (const r of runs) {
    const cancelBtn = r.status === "pending"
      ? el("button", { class: "ghost", text: "Cancel", onclick: (e) => cancelSchedule(r, e.currentTarget) })
      : el("span", { text: "" });
    tbody.appendChild(el("tr", {}, [
      el("td", { text: r.applianceName }),
      el("td", { class: "mono", text: r.zip }),
      el("td", { class: "mono", text: fmtTime(r.startTime) }),
      el("td", { class: "mono", text: r.averageCarbonIntensity }),
      el("td", {}, [statusBadge(r.status)]),
      el("td", { class: "details", text: runDetails(r) }),
      el("td", {}, [cancelBtn]),
    ]));
  }
}

async function cancelSchedule(r, button) {
  if (!confirm(`Cancel the ${r.applianceName} run at ${fmtTime(r.startTime)}?`)) return;
  await withBusy(button, async () => {
    try {
      await api(`/schedules/${r.id}`, { method: "DELETE" });
      toast("Schedule cancelled");
      await loadSchedules();
    } catch (err) {
      toast(err.message, true);
    }
  });
}

document.getElementById("scheduleForm").addEventListener("submit", async (e) => {
  e.preventDefault();
  const form = e.target;
  const applianceName = document.getElementById("schedAppliance").value;
  const zip = document.getElementById("schedZip").value.trim();
  const webhookUrl = document.getElementById("schedWebhook").value.trim();
  const payload = { applianceName, zip };
  if (webhookUrl) payload.webhookUrl = webhookUrl;
  await withBusy(submitButton(form), async () => {
    try {
      await api("/schedules", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      form.reset();
      toast("Scheduled");
      await loadSchedules();
    } catch (err) {
      toast(err.message, true);
    }
  });
});

// ── boot ─────────────────────────────────────────────────────────────────

function loadAppliancesQuietly() {
  return loadAppliances().catch((err) => toast(`Couldn't load appliances: ${err.message}`, true));
}

// Each load fails independently, and the refresh timers start regardless, so one failed
// request at startup can't leave the page frozen. Polling pauses while the tab is hidden.
(function init() {
  refreshHealth();
  loadAppliancesQuietly();
  loadSchedules();

  setInterval(() => { if (!document.hidden) refreshHealth(); }, 10000);
  setInterval(() => { if (!document.hidden) loadSchedules(); }, 15000);
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) return;
    refreshHealth();
    loadSchedules();
    if (appliances.length === 0) loadAppliancesQuietly();
  });
})();
