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

  For hls.js Copyright notice and license, see LICENSE file.
*/

/*
 * Stream Controller
*/

import Event from '../events.js';
import EventHandler from '../hls-event-handler.js';

class StreamController extends EventHandler {
  constructor(hls) {
    super(hls,
      Event.LEVEL_LOADED,
      Event.FRAG_LOADED);
    this.hls = hls;
    this.config = hls.config;
    this.audioCodecSwap = false;
    this.stallReported = false;
    this.gapController = null;
    this.currentFragIdx = 0;
    this.lastSN = 0;
    this.fragments = [];
  }

  _fetchPayloadOrEos(levelDetails) {
    // Keep track of any new frags and load them.
    for (let i = 0; i < levelDetails.fragments.length; i++) {
      let frag = levelDetails.fragments[i];
      if (frag.sn > this.lastSN || this.lastSN == 0) {
        console.log("adding fragment: " + frag.sn);
        this.fragments.push(frag);
        this.lastSN = frag.sn;
      }
    }
    this._loadFragment();
  }

  _loadFragment() {
    let fragLen = this.fragments.length;
    if (this.currentFragIdx >= fragLen) {
      return;
    }
    this.hls.trigger(Event.FRAG_LOADING, { frag: this.fragments[this.currentFragIdx++] });
  }

  onLevelLoaded(data) {
    const newDetails = data.details;
    const newLevelId = data.level;
    const levelDetails = data.details;
    const duration = newDetails.totalduration;
    let sliding = 0;

    console.log(`level ${newLevelId} loaded [${newDetails.startSN},${newDetails.endSN}],duration:${duration}`);

    // override level info
    this.levelLastLoaded = newLevelId;
    this.hls.trigger(Event.LEVEL_UPDATED, { details: newDetails, level: newLevelId });

    if (this.startFragRequested === false) {
      // compute start position if set to -1. use it straight away if value is defined
      if (this.startPosition === -1 || this.lastCurrentTime === -1) {
        // first, check if start time offset has been set in playlist, if yes, use this value
        let startTimeOffset = newDetails.startTimeOffset;
        if (Number.isFinite(startTimeOffset)) {
          if (startTimeOffset < 0) {
            console.log(`negative start time offset ${startTimeOffset}, count from end of last fragment`);
            startTimeOffset = sliding + duration + startTimeOffset;
          }
          console.log(`start time offset found in playlist, adjust startPosition to ${startTimeOffset}`);
          this.startPosition = startTimeOffset;
        } else {
          // if live playlist, set start position to be fragment N-this.config.liveSyncDurationCount (usually 3)
          if (newDetails.live) {
            console.log("handling of this case is not implemented");
          } else {
            this.startPosition = 0;
          }
        }
        this.lastCurrentTime = this.startPosition;
      }
      this.nextLoadPosition = this.startPosition;
    }
    this._fetchPayloadOrEos(levelDetails);
  }

  onFragLoaded(data) {
    this._loadFragment();

  }
}
export default StreamController;
