// loadVars loads the variable element.
async function loadVars() {
  let varElement = document.getElementById("var-element");

  // Get the MAC of the device to fetch vars for.
  const params = new URLSearchParams(window.location.search);
  const ma = params.get("ma");

  const url = new URL("/set/devices/vars", window.location.origin);
  if (ma) {
    url.searchParams.set("ma", ma);
  }

  const resp = await fetch(url.toString());
  const html = await resp.text();

  varElement.innerHTML = html;
}

async function updateVars(event, deleteVar) {
  let varElement = document.getElementById("var-element");

  // Get the MAC of the device
  const params = new URLSearchParams(window.location.search);
  const ma = params.get("ma");

  const url = new URL("/set/devices/edit/var", window.location.origin);
  if (ma) {
    url.searchParams.set("ma", ma);
  }

  if (deleteVar) {
    url.searchParams.set("vd", "true");
  }

  // Add variable to update.
  const form = event.target.closest("form");
  const vn = form.querySelector("input[name=vn]");
  const vv = form.querySelectorAll("input[name=vv]");
  let values = [];
  vv.forEach((elem) => {
    // Only use the value if the input is both shown, and checked.
    if (elem.getAttribute("hidden") == "true") {
      return;
    }
    if (!elem.checked) {
      return;
    }
    values.push(elem.value);
  });

  console.log("vn:", vn);
  console.log("vv:", vv);

  url.searchParams.set("vn", vn.value);
  url.searchParams.set("vv", values.join(","));

  // Show a small loading spinner.
  let spinner = document.createElement("div");
  spinner.className = "spinner-border spinner-border-sm text-primary ms-2";
  spinner.role = "status";
  spinner.innerHTML = `<span class="visually-hidden">Loading...</span>`;
  vn.parentElement.appendChild(spinner);

  try {
    const resp = await fetch(url.toString());
    const html = await resp.text();

    varElement.innerHTML = html;

    let elems = document.querySelectorAll(".var-form");
    for (let elem of elems) {
      setValue(elem.id);
    }
  } catch (err) {
    console.error("Error updating variable:", err);
  } finally {
    // Remove spinner.
    spinner.remove();
  }
}
