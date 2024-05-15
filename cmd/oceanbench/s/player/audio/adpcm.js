/*
NAME
  adpcm.js

AUTHOR
  Trek Hopton <trek@ausocean.org>

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

/*
	Original IMA/DVI ADPCM specification: (http://www.cs.columbia.edu/~hgs/audio/dvi/IMA_ADPCM.pdf).
	Reference algorithms for ADPCM compression and decompression are in part 6.
*/


class Decoder {
    constructor() {
        this.est = 0;   // Estimation of sample based on quantised ADPCM nibble.
        this.idx = 0;   // Index to step used for estimation.
        this.step = 0;
    }

    // Table of index changes (see spec).
    static get indexTable() {
        return [
            -1, -1, -1, -1, 2, 4, 6, 8,
            -1, -1, -1, -1, 2, 4, 6, 8
        ];
    }

    // Quantize step size table (see spec).
    static get stepTable() {
        return [
            7, 8, 9, 10, 11, 12, 13, 14,
            16, 17, 19, 21, 23, 25, 28, 31,
            34, 37, 41, 45, 50, 55, 60, 66,
            73, 80, 88, 97, 107, 118, 130, 143,
            157, 173, 190, 209, 230, 253, 279, 307,
            337, 371, 408, 449, 494, 544, 598, 658,
            724, 796, 876, 963, 1060, 1166, 1282, 1411,
            1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024,
            3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484,
            7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899,
            15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794,
            32767
        ];
    }

    static get byteDepth() { return 2; }    // We are working with 16-bit samples.
    static get headSize() { return 8; }     // Number of bytes in the header of ADPCM.
    static get chunkLenSize() { return 4; } // Length in bytes of the chunk length field in header.
    static get compFact() { return 4; }     // In general ADPCM compresses by a factor of 4.

    // decodeSample takes 4 bits which represents a single ADPCM nibble, and returns a 16 bit decoded PCM sample.
    decodeSample(nibble) {
        let diff = 0;
        if ((nibble & 4) != 0) {
            diff += this.step;
        }
        if ((nibble & 2) != 0) {
            diff += this.step >> 1;
        }
        if ((nibble & 1) != 0) {
            diff += this.step >> 2;
        }
        diff += this.step >> 3;

        if ((nibble & 8) != 0) {
            diff = -diff;
        }
        this.est += diff;
        this.idx += Decoder.indexTable[nibble];

        if (this.idx < 0) {
            this.idx = 0;
        } else if (this.idx > Decoder.stepTable.length - 1) {
            this.idx = Decoder.stepTable.length - 1;
        }

        this.step = Decoder.stepTable[this.idx];

        return this.est;
    }

    // decode takes an array of bytes of arbitrary length representing adpcm and decodes it into pcm.
    decode(b) {
        let result = new Uint16Array(Decoder.decBytes(b)/Decoder.byteDepth);
        let resultOff = 0;
        // Iterate over each chunk and decode it.
        let chunkLen;
        for (let off = 0; off + Decoder.headSize <= b.length; off += chunkLen) {
            // Read length of chunk and check if whole chunk exists.
            chunkLen = Decoder.bytesToInt32(b.slice(off, off + Decoder.chunkLenSize))
            if (off + chunkLen > b.length) {
                break;
            }

            // Initialize Decoder.
            this.est = Decoder.bytesToInt16(b.slice(off + Decoder.chunkLenSize, off + Decoder.chunkLenSize + Decoder.byteDepth));
            this.idx = b[off + Decoder.chunkLenSize + Decoder.byteDepth];
            this.step = Decoder.stepTable[this.idx];

            result[resultOff] = Decoder.bytesToInt16(b.slice(off + Decoder.chunkLenSize, off + Decoder.chunkLenSize + Decoder.byteDepth));
            resultOff++;

            for (let i = off + Decoder.headSize; i < off + chunkLen - b[off + Decoder.chunkLenSize + 3]; i++) {
                let twoNibs = b[i];
                let nib2 = twoNibs >> 4;
                let nib1 = (nib2 << 4) ^ twoNibs;

                let sample1 = this.decodeSample(nib1);
                result[resultOff] = sample1;
                resultOff++;
                
                let sample2 = this.decodeSample(nib2);
                result[resultOff] = sample2;
                resultOff++;
            }
            if (b[off + Decoder.chunkLenSize + 3] == 1) {
                let padNib = b[off + chunkLen - 1];
                let sample = this.decodeSample(padNib);
                result[resultOff] = sample;
                resultOff++;
            }
        }
        return result;
    }

    // bytesToInt16 takes an array of bytes (assumed to be values between 0 and 255), interprates them as little endian and converts it to an int16.
    static bytesToInt16(b) {
        return (b[0] | (b[1] << 8));
    }

    // bytesToInt32 takes an array of bytes (assumed to be values between 0 and 255), interprates them as little endian and converts it to an int32.
    static bytesToInt32(b) {
        return (b[0] |
            (b[1] << 8) |
            (b[2] << 16) |
            (b[3] << 24));
    }

    // decBytes takes a parameter that is assumed to be a byte array containing one or more adpcm chunks.
    // It reads the chunk lengths from the chunk headers to calculate and return the number of pcm bytes that are expected to be decoded from the adpcm.
    static decBytes(b){
        let chunkLen;
        let n = 0;
        for (let off = 0; off + Decoder.headSize <= b.length; off += chunkLen) {
            chunkLen = Decoder.bytesToInt32(b.slice(off, off + Decoder.chunkLenSize))
            if (off + chunkLen > b.length) {
                break;
            }

            // Account for uncompressed sample in header.
            n += Decoder.byteDepth;
            // Account for PCM bytes that will result from decoding ADPCM.
            n += (chunkLen - Decoder.headSize)*Decoder.compFact;
            // Account for padding.
            if(b[off+Decoder.chunkLenSize+3] == 0x01){
                n -= Decoder.byteDepth;
            }
        }
        return n;
    }
}