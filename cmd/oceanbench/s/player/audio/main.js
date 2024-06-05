/*
NAME
  main.js

AUTHOR
  Trek Hopton <trek@ausocean.org>
  Alan Noble <alan@ausocean.org>

LICENSE
  This file is Copyright (C) 2018 the Australian Ocean Lab (AusOcean)

  It is free software: you can redistribute it and/or modify them
  under the terms of the GNU General Public License as published by the
  Free Software Foundation, either version 3 of the License, or (at your
  option) any later version.

  It is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
  FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
  for more details.

  You should have received a copy of the GNU General Public License in gpl.txt.
  If not, see [GNU licenses](http://www.gnu.org/licenses).
*/

// playFile will process and play the chosen target file.
function playFile() {
  const input = document.querySelector('#fileinput');
  if (input.files.length == 0) {
    console.log("no file chosen")
    return
  }
  const file = input.files[0];
  const reader = new FileReader();

  let bd = document.getElementById("bdinput").value;
  let chan = document.getElementById("chaninput").value;
  let rate = document.getElementById("rateinput").value;

  extension = file.name.split('.').pop();
  extension = extension.toLowerCase();
  switch (extension) {
    case "adpcm":
      reader.onload = e => { playADPCM(new Uint8Array(e.target.result), rate, chan, bd) };
      break;
    case "pcm":
    case "raw":
      reader.onload = e => { playPCM(new Uint8Array(e.target.result), rate, chan, bd) };
      break;
    case "wav":
      reader.onload = e => { playWAV(new Uint8Array(e.target.result)) };
      break;
    default:
      console.log("error, unsupported format");
  }
  reader.onerror = error => reject(error);
  reader.readAsArrayBuffer(file);
}

function playADPCM(b, rate, channels, bitdepth) {
  let dec = new Decoder();
  // Decode adpcm to pcm.
  let decoded = dec.decode(b);
  playPCM(new Uint8Array(decoded.buffer), rate, channels, bitdepth);
}

function playPCM(b, rate, channels, bitdepth) {
  let wav = pcmToWav(new Uint8Array(b), rate, channels, bitdepth);
  playWAV(wav);
}

function playWAV(b) {
  // Play wav data in player.
  const blob = new Blob([b], {
    type: 'audio/wav'
  });
  const url = URL.createObjectURL(blob);
  const audio = document.getElementById('audio');
  const source = document.getElementById('source');
  source.src = url;
  audio.load();
  audio.play();
}

// load looks at the browser's URL bar and the URL input element to find a URL to load.
function load(firstLoad) {
  let query = window.location.search.substring(1);
  let params = new URLSearchParams(query);
  let type = params.get("out");
  // TODO: load and play media retreived from URL.
}

// init is what runs when the body of the document has loaded.
function init() {
  document.addEventListener('DOMContentLoaded', function () { load(true) });
  document.addEventListener('DOMContentLoaded', function () {
    document.querySelector('#fileinput').addEventListener('change', playFile);
    document.querySelector('#loadBtn').addEventListener('click', function () { load(false) });
  }
  );
}

init();