document.addEventListener('DOMContentLoaded', function() {
  document.getElementById('time-zone').value = getTimezone();
  const startTimeUnix = document.getElementById('start-time-unix').value;
  const endTimeUnix = document.getElementById('end-time-unix').value;
  const sensorList = JSON.parse(document.getElementById('sensor-list').dataset.sensorList);
  const sendMsg = document.getElementById('send-msg').dataset.sendMsg === 'true';

  if (startTimeUnix) sync('start-time', 'start-time-unix', 'time-zone', false);
  if (endTimeUnix) sync('end-time', 'end-time-unix', 'time-zone', false);

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
