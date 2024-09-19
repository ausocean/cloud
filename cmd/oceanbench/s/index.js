document.addEventListener("sites-loaded", updateNumSites);

function updateNumSites(e) {
  let num = document.getElementById("num-sites");
  num.outerHTML = e.detail.len;
}
