var advancedOpts;
var adv = false;

document.addEventListener('DOMContentLoaded', function() {
  document.getElementById('time-zone').value = getTimezone();
  const startTimestamp = document.getElementById('start-timestamp').value;
  const endTimestamp = document.getElementById('end-timestamp').value;
  const sensorList = JSON.parse(document.getElementById('sensor-list').dataset.sensorList);
  const sendMsg = document.getElementById('send-msg').dataset.sendMsg === 'true';

  if (startTimestamp) sync('start-time', 'start-timestamp', 'time-zone', false);
  if (endTimestamp) sync('end-time', 'end-timestamp', 'time-zone', false);

  if (sensorList){
      sensorList.forEach(sensor => {
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
    let cpair = c
      .trim()
      .split("=", 2);
    if (cpair[0] == "advanced") {
      adv = cpair[1] == "on" ? true : false;
    }
  }

  advancedOpts = document.getElementsByClassName("advanced");
  for (opt of advancedOpts) {
    adv ? document.getElementById("adv-options-toggle").checked = true : opt.style.display = "none";
  }
});

function handleSiteChange(event) {
    // Make a request to change site.
    let xhr = new XMLHttpRequest();
    xhr.open("GET", "/api/set/site/"+event.detail["newSite"]);
    xhr.onreadystatechange = ()=> {
      if (xhr.readyState == XMLHttpRequest.DONE && xhr.responseText == "OK") {
        location.assign("/admin/broadcast"); // This will empty the form.
      }
    }
    xhr.send();
}

function checkAll(form) {
  const sensorList = JSON.parse(document.getElementById('sensor-list').dataset.sensorList);
  sensorList.forEach(sensor => {
    form.querySelector(`input[id='${sensor.Name}']`).checked = true;
  });
  form.submit();
}

function uncheckAll(form) {
  const sensorList = JSON.parse(document.getElementById('sensor-list').dataset.sensorList);
  sensorList.forEach(sensor => {
    form.querySelector(`input[id='${sensor.Name}']`).checked = false;
  });
  form.submit();
}

function buttonClick(button) {
  button.form.querySelector("input[name='action']").value = button.value;
  button.form.submit();
}

function submitSelect(select) {
  select.form.querySelector("input[name='action']").value = "broadcast-select";
  select.form.submit();
}

function toggleAdvanced(checked) {
  for (opt of advancedOpts) {
    checked ? opt.style.removeProperty("display") : opt.style.display = "none";
  }

  document.cookie = (checked ? "advanced=on;" : "advanced=off;") + " path=/admin/broadcast";
}
