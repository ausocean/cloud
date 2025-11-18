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
  const vv = form.querySelector("input[name=vv]");

  console.log("vn:", vn);
  console.log("vv:", vv);

  url.searchParams.set("vn", vn.value);
  url.searchParams.set("vv", vv.value);

  // Show a small loading spinner.
  let spinner = document.createElement("div");
  spinner.className = "spinner-border spinner-border-sm text-primary ms-2";
  spinner.role = "status";
  spinner.innerHTML = `<span class="visually-hidden">Loading...</span>`;
  vv.parentElement.appendChild(spinner);

  try {
    const resp = await fetch(url.toString());
    const html = await resp.text();

    varElement.innerHTML = html;
  } catch (err) {
    console.error("Error updating variable:", err);
  } finally {
    // Remove spinner.
    spinner.remove();
  }
}
