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


class Viewer extends EventHandler {
  constructor(controller, fetcher, display, playPauseBtn, slider) {
    super(controller,
      Events.PLAY,
      Events.JUMP_TO,
      Events.LOAD,
      Events.READY,
      Events.FRAME_RATE_CHANGE,
      Events.STOP
    );
    this.updateImage = this.updateImage.bind(this);
    this.fetcher = fetcher;
    this.display = display;
    this.playPauseBtn = playPauseBtn;
    this.slider = slider;
    this.timer = null;
    this.retryPeriod = 0.5; // TODO: make this configurable.
  }

  onPlay() {
    this.timer = setInterval(this.updateImage, 1000 / this.frameRate);
  }

  updateImage() {
    if (!this.frame) {
      this.controller.triggerEvent(Events.HALT);
    } else {
      const blob = new Blob([new Uint8Array(this.frame.data.buffer)], {
        type: 'video/x-motion-jpeg'
      });
      const url = URL.createObjectURL(blob);
      this.display.src = url;
      this.slider.value = this.frame.number;
      this.frame = null;
    }
    this.frame = this.fetcher.read();
    if (!this.frame) {
      setTimeout(() => { this.buffer() }, this.retryPeriod);
    }
  }

  onJumpTo(target) {
    if (this.fetcher.jump(target)) {
      this.slider.value = target;
    }
  }

  onLoad() {
    clearInterval(this.timer);
    this.initControls();
    this.controller.triggerEvent(Events.READY);
  }

  onReady() {
    this.initInfo();
  }

  onFrameRateChange(rate) {
    this.framerate = rate;
  }

  onStop() {
    clearInterval(this.timer);
  }

  buffer() {
    this.frame = this.fetcher.read();
    if (this.frame) {
      this.controller.triggerEvent(Events.READY);
    } else {
      setTimeout(() => { this.buffer() }, this.retryPeriod);
    }
  }

  initControls() {
    this.playPauseBtn.disabled = false;
    this.slider.disabled = false;
  }

  initInfo() {
    this.frameRate = this.fetcher.getFrameRate();
    this.slider.max = this.fetcher.getDuration() * this.frameRate;
  }
}

export default Viewer;