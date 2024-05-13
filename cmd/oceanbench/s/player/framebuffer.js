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

import MTSDemuxer from './hlsjs/mts-demuxer.js';

// FrameBuffer allows an array of subarrays (MJPEG frames) to be read one at a time.
class FrameBuffer {
  constructor() {
    this.segments = [];
    this.off = { segment: 0, frame: 0 };
    this.frameNum = 0;
    this.started = false;
    this.demuxer = new MTSDemuxer();
    this.append = this.append.bind(this);
  }

  // read returns the next single frame.
  read() {
    // If there are no segments, return nothing.
    if (this.segments.length <= 0) {
      return null;
    }
    // Don't increment if we're at the very beginning.
    if (!this.started) {
      this.started = true;
      return { data: this.segments[this.off.segment][this.off.frame], number: this.frameNum };
    }
    if (!this.incrementOff()) {
      return null;
    }
    return { data: this.segments[this.off.segment][this.off.frame], number: this.frameNum };
  }

  // append takes a blob of MTS data and demuxes it, 
  // then adds the array of all the demuxed frames to this.segments as one segment.
  append(data) {
    let demuxed = this.demuxer.demux(new Uint8Array(data));
    if (!demuxed) {
      return;
    }
    this.segments.push(demuxed.data);
  }

  // incrementOff sets this.off to the index of the next frame in the 2D array this.segments.
  incrementOff() {
    if (!this.segments || !this.segments[this.off.segment]) {
      return false;
    }
    if (this.off.frame + 1 >= this.segments[this.off.segment].length) {
      if (this.off.segment + 1 >= this.segments.length) {
        return false;
      } else {
        this.off.segment++;
        this.off.frame = 0;
        this.frameNum++;
        return true;
      }
    } else {
      this.off.frame++;
      this.frameNum++;
      return true;
    }
  }

  // jump changes this.off to the target frame.
  jump(target) {
    let frames = 0;
    for (let i = 0; i < this.segments.length; i++) {
      if (frames + this.segments[i].length > target) {
        this.off = { segment: i, frame: target - frames }
        this.frameNum = target;
        return true;
      }
      frames += this.segments[i].length;
    }
    return false;
  }
}

export default FrameBuffer