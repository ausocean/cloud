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

// States are the possible states that the player model can be in.
const States = {
  INIT: 'INIT',
  PLAYBACK: 'PLAYBACK',
  PAUSED: 'PAUSED',
  WAIT: 'WAIT'
};

// Model is the player model which hold the player's state. It responds to different events and controls the timing of frames.
class Model extends EventHandler {
  constructor(controller, viewer, fetcher) {
    super(controller,
      Events.PLAY_PAUSE,
      Events.LOAD,
      Events.READY,
      Events.HALT
    );
    this.viewer = viewer;
    this.fetcher = fetcher;
  }

  onLoad() {
    this.changeState(States.INIT);
  }

  onPlayPause() {
    switch (this.state) {
      case States.PLAYBACK:
      case States.WAIT:
        this.changeState(States.PAUSED);
        break;
      case States.PAUSED:
        this.changeState(States.PLAYBACK);
        break;
      default:
        break;
    }
  }

  onReady() {
    switch (this.state) {
      case States.INIT:
        this.changeState(States.PLAYBACK);
        break;
      case States.WAIT:
        if (this.playing) {
          this.changeState(States.PLAYBACK);
        } else {
          this.changeState(States.PAUSED);
        }
        break;
      default:
        break;
    }
  }

  onHalt() {
    switch (this.state) {
      case States.PLAYBACK:
      case States.PAUSED:
        this.changeState(States.WAIT);
        break;
      default:
        break;
    }
  }

  changeState(state) {
    this.state = state;
    switch (state) {
      case States.INIT:
        break;
      case States.PLAYBACK:
        this.controller.triggerEvent(Events.PLAY);
        this.playing = true;
        break;
      case States.WAIT:
        this.controller.triggerEvent(Events.STOP);
        break;
      case States.PAUSED:
        this.controller.triggerEvent(Events.STOP);
        this.playing = false;
        break;
      default:
        console.error("impossible state change");
        break;
    }
  }
}

export default Model;