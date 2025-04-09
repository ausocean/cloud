var advancedOpts;
var adv = false;

let camSelect, controllerSelect;
let prevCamOn, prevCamShutdown, prevCamOff, prevControllerOn, prevControllerOff, prevURL;

document.addEventListener("DOMContentLoaded", function () {
  document.getElementById("time-zone").value = getTimezone();
  const startTimestamp = document.getElementById("start-timestamp").value;
  const endTimestamp = document.getElementById("end-timestamp").value;
  const sensorList = JSON.parse(document.getElementById("sensor-list").dataset.sensorList);
  const sendMsg = document.getElementById("send-msg").dataset.sendMsg === "true";

  if (startTimestamp) sync("start-time", "start-timestamp", "time-zone", false);
  if (endTimestamp) sync("end-time", "end-timestamp", "time-zone", false);

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

  document.getElementById("header").addEventListener("site-change", handleSiteChange);

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
    adv ? (document.getElementById("adv-options-toggle").checked = true) : (opt.style.display = "none");
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
    console.log(`Generated controller actions for ${controller} → ${controllerBase}`);
  }

  if (camSelected) {
    const camBase = macToID(cam);
    onActions.push(`${camBase}.mode=normal`);
    shutdownActions.push(`${camBase}.mode=shutdown`);
    offActions.push(`${camBase}.mode=paused`);
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
  // Make a request to change site.
  let xhr = new XMLHttpRequest();
  xhr.open("GET", "/api/set/site/" + event.detail["newSite"]);
  xhr.onreadystatechange = () => {
    if (xhr.readyState == XMLHttpRequest.DONE && xhr.responseText == "OK") {
      location.assign("/admin/broadcast"); // This will empty the form.
    }
  };
  xhr.send();
}

function checkAll(form) {
  const sensorList = JSON.parse(document.getElementById("sensor-list").dataset.sensorList);
  sensorList.forEach((sensor) => {
    form.querySelector(`input[id='${sensor.Name}']`).checked = true;
  });
  form.submit();
}

function uncheckAll(form) {
  const sensorList = JSON.parse(document.getElementById("sensor-list").dataset.sensorList);
  sensorList.forEach((sensor) => {
    form.querySelector(`input[id='${sensor.Name}']`).checked = false;
  });
  form.submit();
}

function buttonClick(button) {
  button.form.querySelector("input[name='action']").value = button.value;
  button.form.submit();
}

function submitSelect(select) {
  if (!select) {
    select = document.getElementById("broadcast-select");
  }
  select.form.querySelector("input[name='action']").value = "broadcast-select";
  select.form.submit();
}

function toggleAdvanced(checked) {
  for (opt of advancedOpts) {
    checked ? opt.style.removeProperty("display") : (opt.style.display = "none");
  }

  document.cookie = (checked ? "advanced=on;" : "advanced=off;") + " path=/admin/broadcast";
}
