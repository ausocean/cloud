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
