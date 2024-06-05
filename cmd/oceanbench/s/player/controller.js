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

import EventEmitter from './eventemitter3/index.js';
import Events from './events.js';
import Fetcher from './fetcher.js';
import Viewer from './viewer.js';
import Model from './model.js';

class Controller extends EventEmitter {
  constructor(display, playPauseBtn, slider) {
    super();
    this.playPausePressed = this.playPausePressed.bind(this);
    this.jumpRequested = this.jumpRequested.bind(this);
    this.fetcher = new Fetcher(this);
    this.viewer = new Viewer(this, this.fetcher, display, playPauseBtn, slider);
    this.model = new Model(this, this.viewer, this.fetcher);
    playPauseBtn.addEventListener('click', this.playPausePressed);
    slider.oninput = () => {
      this.jumpRequested(slider.value);
    }
  }

  playPausePressed() {
    this.triggerEvent(Events.PLAY_PAUSE);
  }

  loadSource(source) {
    this.triggerEvent(Events.LOAD, source);
  }

  jumpRequested(target) {
    this.triggerEvent(Events.JUMP_TO, target);
  }

  setFrameRate(rate) {
    this.triggerEvent(Events.FRAME_RATE_CHANGE, rate);
  }

  triggerEvent(e, ...data) {
    setTimeout(() => { this.emit(e, e, ...data) }, 0);
  }
}

export default Controller;