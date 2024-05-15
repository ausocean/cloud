/*
DESCRIPTION
  VidGrind text file handling.

AUTHORS
  Scott Barnard <scott@ausocean.org>
  Trek Hopton <trek@ausocean.org>

LICENSE
  Copyright (C) 2020 the Australian Ocean Lab (AusOcean)

  This file is part of VidGrind. VidGrind is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  VidGrind is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with NetReceiver in gpl.txt.  If not, see
  <http://www.gnu.org/licenses/>.
*/

// Global variables.
let logs = [];
const page = [];
let n = 0;

async function initLogs(params) {
  if (!params.id || !params.lv) {
    console.log("Failed to show logs");
    return;
  }

  // Fetch data.
  let resp;
  if (params.st && params.ft) {
    resp = await fetch(`/get?id=${params.id}&ts=${params.st}-${params.ft}`);
  } else {
    resp = await fetch(`/get?id=${params.id}`);
  }

  // Process data.
  const arr = (await resp.text()).split("\n");
  logs = arr
    .map(function (l) {
      try {
        return JSON.parse(l);
      } catch (e) {
        console.log("failed to parse JSON: ", e);
        return null;
      }
    })
    .filter((l) => compareLevels(l, params.lv));

  // 100 logs per page.
  for (let i = 0; i < logs.length / 100; i++) {
    page.push(logs.slice(i * 100, (i + 1) * 100));
  }

  // Render page.
  showLogs();
}

function nextPage() {
  if (n < page.length - 1) {
    n++;
  }
  showLogs();
}

function prevPage() {
  if (n > 0) {
    n--;
  }
  showLogs();
}

function showLogs() {
  // Show page numbers and result numbers.
  document.querySelector("#page-num").innerHTML =
    `Page ${n + 1} of ${page.length} pages`;
  document.querySelector("#result-num").innerHTML = logs.length;

  // Show table.
  const tbody = document.querySelector("tbody");
  tbody.innerHTML = "";
  if (!page[n]) {
    return;
  }
  for (const log of page[n]) {
    let extraInfo = "";
    for (const key in log) {
      if (["level", "time", "caller", "message"].includes(key)) {
        continue;
      }
      extraInfo += `${key}: ${log[key]}, `;
    }

    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td class="pad-5 center">${log.level}</td>
      <td class="pad-5">${log.time}</td>
      <td class="pad-5">${log.caller}</td>
      <td class="pad-5">${log.message}</td>
      <td class="pad-5 scrollable">${extraInfo}</td>
    `;
    tr.classList += `level-${log.level}`;
    tbody.append(tr);
  }
}

function compareLevels(l, lv) {
  if (l == null) return false;

  const fatal = l.level === "fatal";
  const error = l.level === "error" || fatal;
  const warning = l.level === "warn" || error;
  const info = l.level === "info" || warning;

  switch (lv) {
    case "fatal":
      return fatal;
    case "error":
      return error;
    case "warning":
      return warning;
    case "info":
      return info;
    default: // All levels.
      return true;
  }
}
