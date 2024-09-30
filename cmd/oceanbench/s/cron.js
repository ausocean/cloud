window.addEventListener("load", init());

async function init() {
  // Get all variables for the site.
  let devicesPromise = fetch("/api/get/devices/site");
  let varsPromise = await fetch("/api/get/vars/site");

  // Get select elements.
  let varSelects = document.getElementsByName("cv");

  let [devicesRaw, varsRaw] = await Promise.all([devicesPromise, varsPromise]);

  const vars = await varsRaw.json();
  const devs = await devicesRaw.json();

  vars.forEach((v) => {
    let mac = v.Name.split(".")[0];
    let device = devs.find((d) => {
      return d.Mac === parseInt(mac, 16);
    });

    // Add the variables to each dropdown.
    varSelects.forEach((element) => {
      if (v.Name == element.value) {
        // If the values match, this means this is the currently selected var.
        // Edit the inner text rather creating a new option with the same value.
        element.firstElementChild.innerText =
          device.Name + "." + v.Name.split(".")[1];
        return;
      }
      let opt = document.createElement("option");
      opt.value = v.Name;
      opt.innerText = device.Name + "." + v.Name.split(".")[1];
      element.appendChild(opt);
    });
  });
}

function updateCron(elem) {
  elem.form.submit();
}

function addCron() {
  document.getElementById("_newcron").submit();
}

function deleteCron(id) {
  window.location = "/set/crons/edit?ci=" + id + "&task=Delete";
}
