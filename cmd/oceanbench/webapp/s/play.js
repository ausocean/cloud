/*
AUTHOR
  Alan Noble <alan@ausocean.org>
  Trek Hopton <trek@ausocean.org>
  David Sutton <davidsutton@ausocean.org>

LICENSE
  This file is Copyright (C) 2019-2020 the Australian Ocean Lab (AusOcean)

  It is free software: you can redistribute it and/or modify them
  under the terms of the GNU General Public License as published by the
  Free Software Foundation, either version 3 of the License, or (at your
  option) any later version.

  It is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
  FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
  for more details.

  You should have received a copy of the GNU General Public License in gpl.txt.
  If not, see http://www.gnu.org/licenses.
*/

import Controller from "./player/controller.js";

let controller, viewer;
let isPlaying = false;

// Track input source for audio.
let fromUrl = true;
let audioFile;
let mimeType = "audio/wav";

let bd, chan, rate;

// init initialises the event listeners when the appropriate DOM elements have loaded.
function init() {
  document.addEventListener("DOMContentLoaded", function () {
    load(true);
  });
  document.addEventListener("DOMContentLoaded", function () {
    document.getElementById("loadBtn").addEventListener("click", function () {
      load(false);
    });
    document.getElementById("liveBtn").addEventListener("click", live);
  });
}

document.addEventListener("onload", init());

// load looks at the browser's URL bar and the URL input element to find a URL to load.
async function load(firstLoad) {
  let query = window.location.search.substring(1);
  let params = new URLSearchParams(query);
  let type = params.get("out");
  let url = "";
  switch (type) {
    case "x-motion-jpeg":
      document.querySelector("#mjpeg-tab").classList.add("active");
      url = getURL(firstLoad, query, "m3u", params);
      if (firstLoad) {
        initMJPEG();
      }
      loadMJPEG(url);
      break;
    case "h264":
    case "m3u":
      document.querySelector("#h264-tab").classList.add("active");
      url = getURL(firstLoad, query, "m3u", params);
      if (firstLoad) {
        initH264();
      }
      loadH264(url);
      break;
    case "wav":
    case "pcm":
    case "adpcm":
      fromUrl = true;
      document.querySelector("#audio-tab").classList.add("active");
      url = getURL(firstLoad, query, "media", params);
      if (firstLoad) {
        await initAudio();
      }
      switch (type) {
        case "wav":
        case "pcm":
        case "adpcm":
          await getMedia(url, type);
          break;
        default:
          console.log("cannot play given media type", type);
          return;
      }

      // Once the audio has loaded, start playing immediately.
      if (type != "wav") {
        playAudioFile(audioFile);
      } else {
        playAudioFile(audioFile, bd, chan, rate);
      }
      break;
    default:
      console.warn("missing or invalid media output type in URL");
      break;
  }
}

// getURL will format and return a URL based on the type of media requested. The url input element is updated also.
function getURL(firstLoad, query, type, params) {
  let url = "";
  if (firstLoad) {
    if (query.length <= 0) {
      return url;
    }

    if (type != null) {
      params.set("out", type);
    }
    url = "get?" + decodeURIComponent(params.toString());
    document.getElementById("url").value = url;
    return url;
  }
  url = document.getElementById("url").value;
  if (url == "") {
    document.getElementById("msg").innerHTML = "Enter playlist URL";
    url = textPlaylist();
  }
  return url;
}

// initMJPEG creates the elements needed to play MJPEG and initialises their listeners.
function initMJPEG() {
  document.getElementById("specific").innerHTML = `
    <div class="flex-row">
      <div class="flex-column">
        <input type="file" id="fileInput">
      </div>
    </div>
    <div class="flex-row">
      <div class="flex-column width-100"></div>
      <div class="flex-column">
        <fieldset>
          <label>Frame Rate:</label>
          <input type="text" id="rate" value="25">fps
        </fieldset>
      </div>
    </div>
  `;
  document.getElementById("fileInput").addEventListener("change", play);

  // Slider has range 1 - 10000 for now. Should be determined by clip duration.
  document.getElementById("view").innerHTML = `
    <img src="" id="viewer" style="width:100%;">
    <button id="playPauseBtn" disabled="true" class="btn btn-primary">Play / Pause</button>
    <div class="slidecontainer" style="width=100%;">
      <input type="range" min="1" max="10000" value="0" id="slider" disabled="true" style="width:100%;">
    </div>
  `;
  viewer = document.getElementById("viewer");
}

// loadMJPEG creates an instance of a player that can play MJPEG and gives it a URL to load.
function loadMJPEG(url) {
  getController().loadSource(url);
}

// getController returns the player controller or creates one if needed.
// This function should only be used if initMJPEG has already been called.
function getController() {
  if (!controller) {
    controller = new Controller(
      viewer,
      document.querySelector("#playPauseBtn"),
      document.querySelector("#slider"),
    );
  }
  return controller;
}

// play will process and play the target file chosen with the file input element.
function play() {
  let c = getController();
  let rate = document.getElementById("rate");
  if (rate.value > 0) {
    c.setFrameRate(rate.value);
  }
  const input = event.target.files[0];
  c.loadSource(input);
}

// initH264 creates the elements needed to play H264 and initialises their listeners.
function initH264() {
  document.getElementById("view").innerHTML =
    `<video controls id="video" class="responsive"></video>`;
}

// loadH264 creates a player that can play H264 and gives it a URL to load.
function loadH264(url) {
  var video = document.getElementById("video");

  if (Hls.isSupported()) {
    console.log("play: browser can play HLS.");
    var hls = new Hls({
      autoStartLoad: true,
      debug: true,
      startFragPrefetch: true,
    });
    hls.loadSource(url);
    hls.attachMedia(video);

    hls.on(Hls.Events.ERROR, function (event, data) {
      if (data.fatal) {
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            console.log(
              "play: fatal network error encountered, trying to recover.",
            );
            hls.startLoad();
            break;
          case Hls.ErrorTypes.MEDIA_ERROR:
            console.log(
              "play: fatal media error encountered, trying to recover.",
            );
            hls.recoverMediaError();
            break;
          default:
            console.log("play: fatal error, cannot recover.");
            hls.destroy();
            break;
        }
      } else {
        console.log("play: " + data.type + "error, " + data.details);
      }
    });

    hls.on(Hls.Events.MANIFEST_PARSED, function () {
      let playPromise = video.play();
      if (playPromise !== undefined) {
        playPromise
          .then((_) => {
            console.log("play: autoplay started.");
            document.getElementById("msg").innerHTML = "";
          })
          .catch((error) => {
            console.log("play: autoplay was prevented.");
            document.getElementById("msg").innerHTML =
              "Autoplay prevented. Hit play button to start.";
            // Remove the message once playing starts.
            video.addEventListener("playing", function (e) {
              document.getElementById("msg").innerHTML = "";
            });
          });
      }
    });
  } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
    console.log("play: browser can handle vnd.apple.mpegurl.");
    video.src = url;
    video.addEventListener("loadedmetadata", function () {
      video.play();
    });
  } else {
    console.log("play: HLS not supported by browser.");
  }
}

// live loads a live stream URL.
function live() {
  var id = document.getElementById("id").value;
  if (id == "") {
    document.getElementById("msg").innerHTML = "Enter media ID";
    return;
  }
  var fd = document.getElementById("fd").value;
  document.getElementById("url").value =
    "get?id=" + id + "&fd=" + fd + "&out=live";
  load();
}

// textPlaylist checks the text area for a m3u playlist and loads it.
function textPlaylist() {
  var loc = window.location.protocol + "//" + window.location.host;
  var text = document.getElementById("text");
  if (text == null || text.value == "") {
    return "";
  }
  console.log("play: using playlist text.");
  var lines = text.value.split("\n");
  for (i = 0; i < lines.length; i++) {
    if (lines[i].substring(0, 3) == "get") {
      lines[i] = loc + "/" + lines[i];
    }
    lines[i] += "\n";
  }
  url = URL.createObjectURL(
    new Blob(lines, { type: "application/vnd.apple.mpegurl" }),
  );
  return url;
}

// getMedia will request media from a URL and load the player once the response has been received.
async function getMedia(url) {
  return new Promise((resolve, reject) => {
    var xhr = new XMLHttpRequest();
    xhr.open("GET", url);
    xhr.responseType = "arraybuffer";
    xhr.onreadystatechange = function () {
      // check if request's ready state is 4 meaning DONE.
      if (xhr.readyState == 4) {
        let type = xhr.getResponseHeader("content-type").split("/")[1];
        console.log(
          "response mime type",
          xhr.getResponseHeader("content-type"),
        );
        bd = document.getElementById("bdinput").value;
        chan = document.getElementById("chaninput").value;
        rate = document.getElementById("rateinput").value;
        console.log("bitdepth: ", bd, "channels: ", chan, "rate: ", rate);
        switch (type) {
          case "text":
            console.log(
              "received text response: ",
              new TextDecoder("utf-8").decode(xhr.response),
            );
            break;
          case "json":
            console.log(
              "received JSON response: ",
              new TextDecoder("utf-8").decode(xhr.response),
            );
            break;
          case "wav":
            console.log("loading wav");
            resolve((audioFile = xhr.response));
            break;
          case "pcm":
            mimeType = "audio/pcm";
            console.log("loading pcm");
            resolve((audioFile = new Uint8Array(xhr.response)), rate, chan, bd);
            break;
          case "adpcm":
            mimeType = "audio/adpcm";
            console.log("loading adpcm");
            resolve(
              (audioFile = pcmToWav(
                new Uint8Array(
                  decodeADPCM(new Uint8Array(xhr.response)).buffer,
                ),
                rate,
                chan,
                bd,
              )),
            );
            break;
          default:
            console.log("cannot play media, unexpected response type", type);
            return;
        }
      }
    };
    xhr.send();
    // Refresh form check on load.
    xhr.onload = () => {
      isReady();
    };
  });
}

// initAudio creates the elements needed to play audio and initialises their listeners.
async function initAudio() {
  return new Promise((resolve, reject) => {
    document.getElementById("liverow").hidden = true;
    var xhr = new XMLHttpRequest();
    xhr.open("GET", "/s/player/audio-player.html");
    xhr.onreadystatechange = function () {
      if (xhr.readyState == 4) {
        document.getElementById("specific").innerHTML = xhr.responseText;
        document
          .querySelector("#filter-dropdown")
          .addEventListener("change", filterSelect);
        document
          .querySelector("#continue-btn")
          .addEventListener("click", applyFilter);
        document.querySelector("#fileinput").addEventListener("change", () => {
          fromUrl = false;
          applyFilter();
        });
        document
          .querySelector("#fc-lower-input")
          .addEventListener("keyup", isReady);
        document
          .querySelector("#fc-upper-input")
          .addEventListener("keyup", isReady);
        document
          .querySelector("#amp-factor-input")
          .addEventListener("keyup", isReady);
        document
          .querySelector("#loadBtn")
          .addEventListener("click", function () {
            load(false);
          });
        [].forEach.call(
          document.querySelectorAll("[data-action]"),
          function (el) {
            el.addEventListener("click", function (e) {
              let action = e.currentTarget.dataset.action;
              if (action in GLOBAL_ACTIONS) {
                e.preventDefault();
                GLOBAL_ACTIONS[action](e);
              }
            });
          },
        );
        resolve();
      }
    };
    xhr.send();
  });
}

// playAudioFile will process and play the chosen target file.
function playAudioFile(audio, bd, chan, rate) {
  if (!(bd && chan && rate)) {
    bd = document.getElementById("bdinput").value;
    chan = document.getElementById("chaninput").value;
    rate = document.getElementById("rateinput").value;

    loadWAV(pcmToWav(new Uint8Array(audio), rate, chan, bd));
    return;
  }

  loadWAV(pcmToWav(new Uint8Array(audio), rate, chan, bd));
}

// decodeADPCM decodes a Uint8array of ADPCM data and returns PCM data as a Uint16Array.
function decodeADPCM(b) {
  console.log("decodeADPCM received ", b.length, " bytes of ADPCM data");
  let dec = new Decoder();
  // Decode adpcm to pcm.
  let result = dec.decode(b);
  console.log(
    "decodeADPCM resulted in ",
    result.length * 2,
    " bytes of PCM data",
  );
  return result;
}

// loadWAV creates a BLOB for the WAV so it can be loaded in the HTML5 audio player by URL.
function loadWAV(b) {
  const blob = new Blob([b], {
    type: "audio/wav",
  });
  const url = URL.createObjectURL(blob);
  initAndLoadSpectrogram(url);
}

var wavesurfer;

// Init and load waveform and spectrogram.
function initAndLoadSpectrogram(url) {
  if (isPlaying) {
    togglePlayPause();
  }
  let options = {
    container: "#waveform",
    waveColor: "black",
    progressColor: "navy",
    loaderColor: "blue",
    cursorColor: "navy",
    plugins: [
      WaveSurfer.spectrogram.create({
        container: "#wave-spectrogram",
        labels: true,
        frequencyMax: 24000,
      }),
    ],
  };

  // If query contains "scroll", use higher resolution scrollable view.
  if (location.search.match("scroll")) {
    options.minPxPerSec = 100;
    options.scrollParent = true;
  }

  // If query contains "hot", use heat map colour scheme.
  if (location.search.match("hot")) {
    WaveSurfer.util
      .fetchFile({ url: "s/hot-colormap.json", responseType: "json" })
      .on("success", (colorMap) => {
        options.plugins[0].params.colorMap = colorMap;
      });
  }

  // Only create a new wavesurfer if it has not already been created.
  if (wavesurfer == null) {
    wavesurfer = WaveSurfer.create(options);
  }

  // Progress bar.
  (function () {
    let progressDiv = document.querySelector("#progress-bar");

    let showProgress = function (percent) {
      progressDiv.hidden = "false";
      progressDiv.value = percent;
    };

    let hideProgress = function () {
      progressDiv.hidden = "true";
    };

    wavesurfer.on("loading", showProgress);
    wavesurfer.on("ready", hideProgress);
    wavesurfer.on("destroy", hideProgress);
    wavesurfer.on("error", hideProgress);
  })();

  wavesurfer.load(url);
}

var GLOBAL_ACTIONS = {
  // eslint-disable-line
  play: function () {
    wavesurfer.playPause();
    togglePlayPause();
  },
  back: function () {
    wavesurfer.skipBackward();
  },
  forth: function () {
    wavesurfer.skipForward();
  },
  "toggle-mute": function () {
    wavesurfer.toggleMute();
  },
};

// Bind actions to buttons and keypresses
document.addEventListener("DOMContentLoaded", function () {
  document.addEventListener("keydown", function (e) {
    let map = {
      32: "play", // space
      37: "back", // left
      39: "forth", // right
    };
    let action = map[e.keyCode];
    if (action in GLOBAL_ACTIONS) {
      if (
        document == e.target ||
        document.body == e.target ||
        e.target.attributes["data-action"]
      ) {
        e.preventDefault();
      }
      GLOBAL_ACTIONS[action](e);
    }
  });
});

// fileCheck runs to see whether a file has been uploaded and whether it is a valid type.
function fileCheck() {
  // Get file type.
  let input = document.getElementById("fileinput");
  let fileType = "";
  if (input.value) {
    fileType = input.files[0].name.split(".").slice(-1);
  }
  // Check if valid type (includes checking for upload overall).
  const validFileTypes = ["pcm", "raw", "wav", "adpcm"];
  return validFileTypes.includes(fileType[0]);
}

// filterSelect is used to select the type of filter to be used, and update the form appropriately.
function filterSelect() {
  // Set filter value.
  let filterType = document.getElementById("filter-dropdown").value;
  console.log("filter is of type: ", filterType);

  // Get containers for different form sections.
  let fcUpperContainer = document.querySelector("#fc-upper-input-container");
  let fcLowerContainer = document.querySelector("#fc-lower-container");
  let ampFactorContainer = document.querySelector(
    "#amp-factor-input-container",
  );

  // Get form fields and populate with default values if empty.
  var fields = document.getElementsByClassName("parameter-input");
  fields[0].value = fields[0].value || 5000; // fcLower.
  fields[1].value = fields[1].value || 10000; // fcUpper.
  fields[2].value = fields[2].value || 2; // ampFactor.

  // Show correct input fields for chosen filter type.
  const displayValues = {
    Lowpass: ["block", "none", "none"],
    Highpass: ["none", "block", "none"],
    Bandpass: ["block", "block", "none"],
    Bandstop: ["block", "block", "none"],
    Amplifier: ["none", "none", "block"],
  };
  const [fcUpperDisplay, fcLowerDisplay, ampFactorDisplay] = displayValues[
    filterType
  ] || ["none", "none", "none"];
  fcUpperContainer.style.display = fcUpperDisplay;
  fcLowerContainer.style.display = fcLowerDisplay;
  ampFactorContainer.style.display = ampFactorDisplay;
}

// isReady runs when changes are made to the form to determine whether the continue button should be available.
function isReady() {
  // If audio is from the URL, don't check form.
  if (fromUrl) {
    document.getElementById("continue-btn").removeAttribute("disabled");
    return;
  }

  // Cancel if uploaded file is of an invalid type.
  if (!fileCheck()) {
    alert("Invalid file type, please try again");
    return;
  }

  // Ensure that all relevant fields are filled.
  let hidableInputs = document.getElementsByClassName("hidable-form");
  for (let item of hidableInputs) {
    let input = item.getElementsByClassName("parameter-input")[0];
    if (item.style.display == "none") {
    } else if (parseFloat(input.value) < 22100 && parseFloat(input.value) > 0) {
    } else {
      document.getElementById("continue-btn").setAttribute("disabled", "true");
      return;
    }
  }

  // Enable apply filter button.
  let applyBtn = document.getElementById("continue-btn");
  if (fileCheck) {
    applyBtn.removeAttribute("disabled");
  }
}

// applyFilter gets the inputted data from the form or from the search result, and makes a HTTP POST request,
// which is read by filterHandler.
function applyFilter() {
  // Cancel if file is invalid.
  if (!fromUrl && !fileCheck()) {
    alert("Invalid audio file (Wrong type or missing).");
    return;
  }

  if (document.getElementById("filter-dropdown").value == "none") {
    playAudioFile(audioFile);
  }

  // Load the form element.
  const formElement = document.querySelector("#player-form");

  // Make HTTP POST request.
  const request = new XMLHttpRequest();
  request.open("POST", "/play/audiorequest");
  request.responseType = "arraybuffer";

  request.onload = () => {
    console.log("audio request sent");
    if (request.status == 200) {
      let fileType = fromUrl
        ? "wav"
        : String(
            document
              .getElementById("fileinput")
              .files[0].name.split(".")
              .slice(-1),
          ).toLowerCase();
      mimeType = "audio/pcm";
      if (fileType == "wav") {
        const bd = request.getResponseHeader("bit-depth");
        const chan = request.getResponseHeader("channels");
        const rate = request.getResponseHeader("sample-rate");
        playAudioFile(request.response, bd, chan, rate);
      } else {
        playAudioFile(request.response);
      }
      // Update audioFile so new filters are applied to this instead.
      audioFile = request.response;
    } else if (request.status == 400) {
      let alertMSG =
        "An error occurred: " +
        request.getResponseHeader("msg") +
        "\nPlease try again.";
      alert(alertMSG);
    }
  };

  let form = new FormData(formElement);
  // If the audio is from the URL, update the request with this audio.
  if (fromUrl) {
    form.set(
      "audio-file",
      new Blob([audioFile], { type: mimeType }),
      mimeType.replace("/", "."),
    );
  }
  request.send(form);
}

// togglePlayPause toggles the play button to a pause and vice versa based on the current state of isPlaying.
function togglePlayPause() {
  let btn = document.getElementById("play-pause");
  isPlaying = !isPlaying;
  if (isPlaying) {
    btn.innerHTML = `<i class="gg-play-pause"></i>&ensp;Pause`;
  } else {
    btn.innerHTML = `<i class="gg-play-button"></i>Play`;
  }
}
