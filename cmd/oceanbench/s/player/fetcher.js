/*
AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  This file is Copyright (C) 2020 the Australian Ocean Lab (AusOcean)

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

import Events from './events.js';
import EventHandler from './hlsjs/event-handler.js';
import FrameBuffer from './framebuffer.js';
import MJPEGLexer from './lex-mjpeg.js';
import VGPlayer from './hlsjs/hls.js';

// Fetcher is in charge of loading media data from a source. 
class Fetcher extends EventHandler {
  constructor(controller) {
    super(controller,
      Events.LOAD,
      Events.FRAME_RATE_CHANGE
    );
    this.onFragLoaded = this.onFragLoaded.bind(this);
    this.onLevelLoaded = this.onLevelLoaded.bind(this);
    this.framerate = 25; //TODO: read this from MTS metadata.
  }

  onLoad(source) {
    if (typeof source.name == 'string') {
      const reader = new FileReader();
      reader.onload = event => {
        switch (source.name.split('.')[1]) {
          case "mjpeg":
          case "mjpg":
          case "x-motion-jpeg":
            this.frameSrc = new MJPEGLexer();
            this.frameSrc.append(event.target.result);
            break;
          case "ts":
          case "mts":
          case "mpegts":
            this.frameSrc = new FrameBuffer();
            this.frameSrc.append(event.target.result);
            break;
          default:
            console.error("unknown file format");
            break;
        }
      };
      reader.onerror = error => {
        console.error('could not read file: ' + error)
      };
      reader.readAsArrayBuffer(source);
    } else {
      this.frameSrc = new FrameBuffer();
      let hls = new VGPlayer();
      hls.loadSource(source, this.frameSrc.append);
      hls.on(Hls.Events.FRAG_LOADED, this.onFragLoaded);
      hls.on(Hls.Events.LEVEL_LOADED, this.onLevelLoaded);
    }
  }

  onFrameRateChange(rate) {
    this.framerate = rate;
  }

  onFragLoaded(event, data) {
    this.frameSrc.append(data.payload);
  }

  onLevelLoaded(event, data) {
    this.levelDetails = data.details;
  }

  read() {
    if (!this.frameSrc) {
      return null;
    }
    let frame = this.frameSrc.read();
    return frame;
  }

  jump(target) {
    return this.frameSrc.jump(target);
  }

  getFrameRate() {
    return this.framerate;
  }

  getDuration() {
    if (this.levelDetails) {
      return this.levelDetails.totalduration;
    }
    return null;
  }
}

export default Fetcher;