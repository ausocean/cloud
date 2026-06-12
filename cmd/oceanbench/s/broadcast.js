var advancedOpts;
var adv = false;

let camSelect, controllerSelect;
let prevCamOn,
  prevCamShutdown,
  prevCamOff,
  prevControllerOn,
  prevControllerOff,
  prevURL;

document.addEventListener("DOMContentLoaded", function() {
  document.getElementById("time-zone").value = getTimezone();
  const startTimestamp = document.getElementById("start-timestamp").value;
  const endTimestamp = document.getElementById("end-timestamp").value;
  const sensorList = JSON.parse(
    document.getElementById("sensor-list").dataset.sensorList,
  );
  const sendMsg =
    document.getElementById("send-msg").dataset.sendMsg === "true";

  if (startTimestamp)
    syncDateTime("start-time", "start-timestamp", "time-zone", false);
  if (endTimestamp)
    syncDateTime("end-time", "end-timestamp", "time-zone", false);

  if (sensorList) {
    sensorList.forEach((sensor) => {
      if (sensor.SendMsg) {
        document.getElementById(sensor.Name).checked = true;
      }
    });
  }

  if (sendMsg) {
    document.getElementById("report-sensor").checked = true;
  }

  document
    .getElementById("header")
    .addEventListener("site-change", handleSiteChange);

  // Read the cookie for the advanced settings.
  let cookies = document.cookie.split(";", 5);
  for (c of cookies) {
    let cpair = c.trim().split("=", 2);
    if (cpair[0] == "advanced") {
      adv = cpair[1] == "on" ? true : false;
    }
  }

  advancedOpts = document.getElementsByClassName("advanced");
  for (opt of advancedOpts) {
    adv
      ? (document.getElementById("adv-options-toggle").checked = true)
      : (opt.style.display = "none");
  }

  camSelect = document.getElementById("camera-select");
  controllerSelect = document.getElementById("controller-select");
});

function generateActions(e) {
  const onActs = document.getElementById("on-actions");
  const shutdownActs = document.getElementById("shutdown-actions");
  const offActs = document.getElementById("off-actions");
  const rtmpVar = document.getElementById("rtmp-var");

  const controller = getSelectedValue(controllerSelect);
  const cam = getSelectedValue(camSelect);

  const controllerSelected = controller !== "Select";
  const camSelected = cam !== "Select";

  // If nothing is selected, just clear everything
  if (!controllerSelected && !camSelected) {
    console.log("Nothing selected. Clearing all action fields.");
    onActs.value = "";
    shutdownActs.value = "";
    offActs.value = "";
    rtmpVar.value = "";
    return;
  }

  let onActions = [];
  let shutdownActions = [];
  let offActions = [];

  if (controllerSelected) {
    const controllerBase = macToID(controller);
    onActions.push(`${controllerBase}.Power2=true`);
    offActions.push(`${controllerBase}.Power2=false`);
    console.log(
      `Generated controller actions for ${controller} → ${controllerBase}`,
    );
  }

  if (camSelected) {
    const camBase = macToID(cam);
    onActions.push(`${camBase}.mode=Normal`);
    shutdownActions.push(`${camBase}.mode=Shutdown`);
    offActions.push(`${camBase}.mode=Paused`);
    rtmpVar.value = `${camBase}.RTMPURL`;
    console.log(`Generated camera actions for ${cam} → ${camBase}`);
  } else {
    rtmpVar.value = "";
  }

  // Join all values with commas and update the fields
  onActs.value = onActions.join(",");
  shutdownActs.value = shutdownActions.join(",");
  offActs.value = offActions.join(",");
}

function getSelectedValue(selectElement) {
  return selectElement.options[selectElement.selectedIndex].value;
}

function macToID(mac) {
  return mac.toLowerCase().replaceAll(":", "");
}

function handleSiteChange(event) {
  let siteKey = event.detail["newSite"].split(":")[0];

  // Make a request to change site.
  let xhr = new XMLHttpRequest();
  xhr.open("GET", "/api/set/site/" + event.detail["newSite"]);
  xhr.onreadystatechange = () => {
    if (xhr.readyState == XMLHttpRequest.DONE && xhr.responseText == "OK") {
      sessionStorage.setItem("site", siteKey);
      location.assign("/admin/broadcast?site=" + siteKey); // This will empty the form.
    }
  };
  xhr.send();
}

function checkAll(form) {
  const sensorList = JSON.parse(
    document.getElementById("sensor-list").dataset.sensorList,
  );
  sensorList.forEach((sensor) => {
    form.querySelector(`input[id='${sensor.Name}']`).checked = true;
  });
  form.submit();
}

function uncheckAll(form) {
  const sensorList = JSON.parse(
    document.getElementById("sensor-list").dataset.sensorList,
  );
  sensorList.forEach((sensor) => {
    form.querySelector(`input[id='${sensor.Name}']`).checked = false;
  });
  form.submit();
}

function buttonClick(button) {
  button.form.querySelector("input[name='action']").value = button.value;
  button.form.submit();
}

function toggleAdvanced(checked) {
  for (opt of advancedOpts) {
    checked
      ? opt.style.removeProperty("display")
      : (opt.style.display = "none");
  }

  document.cookie =
    (checked ? "advanced=on;" : "advanced=off;") + " path=/admin/broadcast";
}

// Maps UUIDs to broadcast names to prevent unnecessary requests.
var nameCache = new Map()

// Caches the given broadcast UUID and name.
function cacheBroadcastName(uuid, name) {
  if (uuid && uuid.startsWith("Broadcast.")) {
    uuid = uuid.substring("Broadcast.".length);
  }
  nameCache.set(uuid, name)
}

async function handleReset(event) {
  // Prevent form submission.
  event.preventDefault();

  // Get the current value from the broadcast selector.
  let selectedID = document.getElementById("broadcast-select").value

  // Update broadcast list.
  updateBroadcastsList(selectedID)

  // Reload config.
  handleBroadcastSelect(selectedID);
}

// Fetches broadcast IDs and names (if not cached) and reconstructs
// the broadcast select list.
async function updateBroadcastsList(selectedID) {
  // Get the broadcast IDs to update the broadcast list.
  let ids = await fetchBroadcastIDs()

  // Clear existing options from broadcast selector.
  let selector = document.getElementById("broadcast-select")
  selector.innerHTML = ""

  // Add default new broadcast option.
  let opt = new Option("-- New Broadcast --", "")
  selector.appendChild(opt)

  // Add broadcasts to select input.
  for (id of ids) {
    let uuid = id.replace("Broadcast.", "")
    let name = nameCache.get(uuid)

    // If the name doesn't exist in the cache, we need to request it.
    // This should only happen if the broadcast is newly created on another device or tab.
    if (name == undefined) {
      console.log("fetching broadcast with unseen UUID")
      let cfg = await fetchBroadcast(uuid);
      name = cfg.Name;
    }

    // Create option element.
    opt = new Option(name, id)

    // Mark as selected if it was already selected.
    console.log(`selected: ${selectedID}, curr: ${id}`)
    opt.selected = uuid == selectedID

    // Append it to the select input.
    selector.appendChild(opt)
  }
}

// handleBroadcastSelect triggers fetching and form population
async function handleBroadcastSelect(uuid) {
  // HTML <option> elements in the select dropdown have values prefixed with "Broadcast."
  // (e.g., "Broadcast.1234-5678"). We strip this before making the API request
  // to ensure we're querying against just the raw UUID.
  if (uuid && uuid.startsWith("Broadcast.")) {
    uuid = uuid.substring("Broadcast.".length);
  }

  const loadingOverlay = document.getElementById("loading-overlay");
  if (loadingOverlay) loadingOverlay.classList.remove("d-none");

  const ytContainer = document.getElementById("youtube-preview-container");
  const ytFrame = document.getElementById("youtube-preview-frame");
  if (ytContainer) {
    ytContainer.classList.add("d-none");
    ytContainer.classList.remove("d-flex");
  }
  if (ytFrame) ytFrame.src = "";

  if (!uuid) {
    if (loadingOverlay) loadingOverlay.classList.add("d-none");
    document.querySelector("form").reset();
    return;
  }

  const data = await fetchBroadcast(uuid);

  if (data) {
    populateForm(data);
    cacheBroadcastName(uuid, data.Name)
    updateBroadcastsList(uuid)
  } else {
    alert("Failed to load broadcast data.");
  }

  if (loadingOverlay) loadingOverlay.classList.add("d-none");
}

function populateForm(data) {
  // Simple inputs
  const mapping = {
    "broadcast-name": data.Name,
    "broadcast-uuid": data.UUID,
    account: data.Account,
    description: data.Description,
    "stream-name": data.StreamName,
    "rtmp-key-var": data.RTMPVar,
    "rtmp-key": data.RTMPKey,
    "vidforward-host": data.VidforwardHost,
    "battery-voltage-pin": data.BatteryVoltagePin,
    "required-streaming-voltage": data.RequiredStreamingVoltage,
    "voltage-recovery-timeout": data.VoltageRecoveryTimeout,
    "openfish-capturesource": data.OpenFishCaptureSource,
    "notify-suppress-rules": data.NotifySuppressRules,
    "on-actions": data.OnActions,
    "shutdown-actions": data.ShutdownActions,
    "off-actions": data.OffActions,
    "start-timestamp": data.StartTimestamp,
    "end-timestamp": data.EndTimestamp,
  };

  for (const [name, val] of Object.entries(mapping)) {
    const el = document.querySelector(
      `input[name="${name}"], textarea[name="${name}"]`,
    );
    if (el) el.value = val !== undefined && val !== null ? val : "";
  }

  // Checkboxes
  const checks = {
    enabled: data.Enabled,
    "in-failure": data.InFailure,
    "use-vidforward": data.UsingVidforward,
    "check-health": data.CheckingHealth,
    "register-openfish": data.RegisterOpenFish,
  };

  for (const [name, val] of Object.entries(checks)) {
    const el = document.querySelector(`input[name="${name}"][type="checkbox"]`);
    if (el) el.checked = !!val;
  }

  // Radio buttons
  if (data.LivePrivacy) {
    const el = document.querySelector(
      `input[name="live-privacy"][value="${data.LivePrivacy}"]`,
    );
    if (el) el.checked = true;
  }
  if (data.PostLivePrivacy) {
    const el = document.querySelector(
      `input[name="post-live-privacy"][value="${data.PostLivePrivacy}"]`,
    );
    if (el) el.checked = true;
  }

  // Selects
  const camSelect = document.getElementById("camera-select");
  if (
    camSelect &&
    data.CameraMac !== undefined &&
    data.CameraMac !== null &&
    data.CameraMac !== 0
  ) {
    camSelect.value = formatMac(data.CameraMac);
  }

  const controllerSelect = document.getElementById("controller-select");
  if (
    controllerSelect &&
    data.ControllerMAC !== undefined &&
    data.ControllerMAC !== null &&
    data.ControllerMAC !== 0
  ) {
    controllerSelect.value = formatMac(data.ControllerMAC);
  }

  // Hidden/Readonly
  const bEl = document.querySelector(`input[name="broadcast-id"]`);
  if (bEl) bEl.value = data.BID || "";
  const aEl = document.querySelector(`input[name="active"]`);
  if (aEl) aEl.value = data.Active || false;

  const stateEl = document.getElementById("broadcast-state");
  if (stateEl) stateEl.value = data.BroadcastState || "";
  const hStateEl = document.getElementById("hardware-state");
  if (hStateEl) hStateEl.value = data.HardwareState || "";
  const hdEl = document.getElementById("hardware-state-data");
  if (hdEl) {
    if (data.HardwareStateData) {
      try {
        const decoded = atob(data.HardwareStateData);
        hdEl.value = decoded;
      } catch (e) {
        hdEl.value = data.HardwareStateData;
      }
    } else {
      hdEl.value = "";
    }
  }

  // Synchronize time inputs
  if (data.StartTimestamp) {
    if (typeof syncDateTime === "function")
      syncDateTime("start-time", "start-timestamp", "time-zone", false);
  }
  if (data.EndTimestamp) {
    if (typeof syncDateTime === "function")
      syncDateTime("end-time", "end-timestamp", "time-zone", false);
  }

  // Check sensor config dynamically
  if (data.SensorList && Array.isArray(data.SensorList)) {
    // Uncheck all first
    document
      .querySelectorAll(`input[type="checkbox"].advanced`)
      .forEach((el) => {
        data.SensorList.forEach((sensor) => {
          if (el.id === sensor.Name) el.checked = false;
        });
      });
    // Check included
    data.SensorList.forEach((sensor) => {
      if (sensor.SendMsg) {
        const el = document.getElementById(sensor.Name);
        if (el) el.checked = true;
      }
    });
  }

  const reportSens = document.getElementById("report-sensor");
  if (reportSens) {
    reportSens.checked = !!data.SendMsg;
  }

  // Youtube Preview
  if (data.BID) {
    const c = document.getElementById("youtube-preview-container");
    const frame = document.getElementById("youtube-preview-frame");
    if (c && frame) {
      frame.src = `https://www.youtube.com/embed/${data.BID}`;
      c.classList.remove("d-none");
      c.classList.add("d-flex");
    }
  }
}

// fetchBroadcast fetches the broadcast configuration asynchronously via the API.
// This is exposed to allow custom functionality or components to fetch things like the NotifySuppressRules.
async function fetchBroadcast(uuid) {
  if (!uuid) return null;
  try {
    const urlParams = new URLSearchParams(window.location.search);
    const site = urlParams.get("site");
    const url = site
      ? `/api/v1/broadcasts/${uuid}?site=${site}`
      : `/api/v1/broadcasts/${uuid}`;

    const res = await fetch(url);
    if (!res.ok) {
      console.error(`Failed to fetch broadcast config: ${res.statusText}`);
      return null;
    }
    return await res.json();
  } catch (err) {
    console.error(`Error fetching broadcast config:`, err);
    return null;
  }
}

async function fetchBroadcastIDs() {
  const urlParams = new URLSearchParams(window.location.search);
  const site = urlParams.get('site');
  const url = site ? `/api/v1/broadcasts?site=${site}` : `/api/v1/broadcasts`;

  try {
    const resp = await fetch(url);
    if (!resp.ok) {
      throw new Error(resp.statusText);
    }
    const data = await resp.json();
    return data.map((v) => { return "Broadcast." + v.split(".")[2] })
  } catch (err) {
    console.error(`Error fetching broadcast IDs: ${err}`)
  }
}

function formatMac(macInt) {
  if (!macInt) return "";
  let hex = macInt.toString(16).toUpperCase();
  hex = hex.padStart(12, "0");
  let result = [];
  for (let i = 0; i < 12; i += 2) {
    result.push(hex.substring(i, i + 2));
  }
  return result.join(":");
}
