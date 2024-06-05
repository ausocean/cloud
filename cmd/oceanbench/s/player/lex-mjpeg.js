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

// MJPEGLexer lexes a byte array containing MJPEG into individual JPEGs.
class MJPEGLexer {
    constructor() {
        // off keeps track of the byte offset to start looking for the next MJPEG frame.
        this.off = 0;
        // frameNum track of the current frame number.
        this.frameNum = 0;
    }

    append(data) {
        this.src = new Uint8Array(data);
    }

    // read returns the next single frame along with its number in the sequence.
    read() {
        // Check if the src can contain at least the start and end flags (4B).
        if (this.off + 4 > this.src.length) {
            return null;
        }
        // Iterate through bytes until the start flag is found.
        while (this.src[this.off] != 0xff || this.src[this.off + 1] != 0xd8) {
            this.off++;
            if (this.off + 4 > this.src.length) {
                return null;
            }
        }
        // Start after the start flag and loop until the end flag is found.
        let end = this.off + 2;
        while (true) {
            if (end + 2 > this.src.length) {
                return null;
            }
            if (this.src[end] == 0xff && this.src[end + 1] == 0xd9) {
                break;
            }
            end++;
        }
        // Copy the frame's bytes to a new array to return.
        // Note: optimally this would return a reference but since we are in a worker thread, 
        // the main thread doesn't have access to the ArrayBuffer that we are working with.
        let frame = this.src.slice(this.off, end + 2);
        this.off = end + 2
        return { data: frame, number: this.frameNum++ };
    }
}

export default MJPEGLexer