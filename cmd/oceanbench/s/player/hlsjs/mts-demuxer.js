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

// MTSDemuxer demultiplexes an MPEG-TS stream into its individual streams.
// While it is possible that the MPEG-TS stream may contain many streams, 
// this demuxer will result in at most one stream of each type ie. video, audio, id3 metadata.  
class MTSDemuxer {
  constructor() {
    this.init();
  }

  // init initialises MTSDemuxer's state. It can be used to reset an MTSDemuxer instance. 
  init() {
    this.pmtParsed = false;
    this._pmtId = -1;
  }

  // createTrack creates and returns a track model.
  /**
   * @param {string} type 'audio' | 'video' | 'id3' | 'text'
   * @return {object} MTSDemuxer's internal track model.
   */
  static createTrack(type) {
    return {
      type,
      pid: -1,
      data: [] // This will contain Uint8Arrays representing each PES packet's payload for this track.
    };
  }

  // _syncOffset scans the first 'maxScanWindow' bytes and returns an offset to the beginning of the first three MTS packets, 
  // or -1 if three are not found.
  // A TS fragment should contain at least 3 TS packets, a PAT, a PMT, and one PID, each starting with 0x47.
  static _syncOffset(data) {
    const maxScanWindow = 1000; // 1000 is a reasonable number of bytes to search for the first MTS packets.
    const scanWindow = Math.min(maxScanWindow, data.length - 3 * 188);
    let i = 0;
    while (i < scanWindow) {
      if (data[i] === 0x47 && data[i + 188] === 0x47 && data[i + 2 * 188] === 0x47) {
        return i;
      } else {
        i++;
      }
    }
    return -1;
  }

  demux(data) {
    let start, len = data.length, pusi, pid, afc, offset, pes,
      unknownPIDs = false;
    let pmtParsed = this.pmtParsed,
      videoTrack = MTSDemuxer.createTrack('video'),
      audioTrack = MTSDemuxer.createTrack('audio'),
      id3Track = MTSDemuxer.createTrack('id3'),
      videoId,
      audioId,
      id3Id,
      pmtId = this._pmtId,
      videoData = this.videoPesData,
      audioData = this.audioPesData,
      id3Data = this.id3PesData,
      parsePAT = this._parsePAT,
      parsePMT = this._parsePMT,
      parsePES = this._parsePES;

    const syncOffset = MTSDemuxer._syncOffset(data);
    if (syncOffset == -1) {
      console.warn('no TS fragment found in data');
      return null;
    }

    // Don't parse last TS packet if incomplete.
    len -= (len + syncOffset) % 188;

    // Loop through TS packets.
    for (start = syncOffset; start < len; start += 188) {
      if (data[start] === 0x47) {
        pusi = !!(data[start + 1] & 0x40);
        // pid is a 13-bit field starting at the last bit of TS[1].
        pid = ((data[start + 1] & 0x1f) << 8) + data[start + 2];
        afc = (data[start + 3] & 0x30) >> 4;
        // If an adaption field is present, its length is specified by the fifth byte of the TS packet header.
        if (afc > 1) {
          offset = start + 5 + data[start + 4];
          // Continue if there is only adaptation field.
          if (offset === (start + 188)) {
            continue;
          }
        } else {
          offset = start + 4;
        }
        switch (pid) {
          case videoId:
            if (pusi) {
              if (videoData && (pes = parsePES(videoData)) && pes.pts !== undefined) {
                videoTrack.data.push(pes.data);
                // TODO: here pes contains data, pts, dts and len. Are all these needed?
              }
              videoData = { data: [], size: 0 };
            }
            if (videoData) {
              videoData.data.push(data.subarray(offset, start + 188));
              videoData.size += start + 188 - offset;
            }
            break;
          case audioId:
            if (pusi) {
              if (audioData && (pes = parsePES(audioData)) && pes.pts !== undefined) {
                audioTrack.data.push(pes.data);
              }
              audioData = { data: [], size: 0 };
            }
            if (audioData) {
              audioData.data.push(data.subarray(offset, start + 188));
              audioData.size += start + 188 - offset;
            }
            break;
          case id3Id:
            if (pusi) {
              if (id3Data && (pes = parsePES(id3Data)) && pes.pts !== undefined) {
                id3Track.data.push(pes.data);
              }
              id3Data = { data: [], size: 0 };
            }
            if (id3Data) {
              id3Data.data.push(data.subarray(offset, start + 188));
              id3Data.size += start + 188 - offset;
            }
            break;
          case 0:
            if (pusi) {
              offset += data[offset] + 1;
            }

            pmtId = this._pmtId = parsePAT(data, offset);
            break;
          case pmtId:
            if (pusi) {
              offset += data[offset] + 1;
            }

            let parsedPIDs = parsePMT(data, offset);

            // Only update track id if track PID found while parsing PMT.
            // This is to avoid resetting the PID to -1 in case track PID transiently disappears from the stream,
            // this could happen in case of transient missing audio samples for example.
            videoId = parsedPIDs.video;
            if (videoId > 0) {
              videoTrack.pid = videoId;
            }
            audioId = parsedPIDs.audio;
            if (audioId > 0) {
              audioTrack.pid = audioId;
            }
            id3Id = parsedPIDs.id3;
            if (id3Id > 0) {
              id3Track.pid = id3Id;
            }

            if (unknownPIDs && !pmtParsed) {
              // Reparse from beginning.
              unknownPIDs = false;
              // We set it to -188, the += 188 in the for loop will reset start to 0.
              start = syncOffset - 188;
            }
            pmtParsed = this.pmtParsed = true;
            break;
          default:
            unknownPIDs = true;
            break;
        }
      } else {
        console.error('TS packet did not start with 0x47');
      }
    }

    // Try to parse last PES packets.
    if (videoData && (pes = parsePES(videoData)) && pes.pts !== undefined) {
      videoTrack.data.push(pes.data);
      this.videoPesData = null;
    } else {
      // Either pesPkts null or PES truncated, keep it for next frag parsing.
      this.videoPesData = videoData;
    }

    if (audioData && (pes = parsePES(audioData)) && pes.pts !== undefined) {
      audioTrack.data.push(pes.data);
      this.audioPesData = null;
    } else {
      // Either pesPkts null or PES truncated, keep it for next frag parsing.
      this.audioPesData = audioData;
    }

    if (id3Data && (pes = parsePES(id3Data)) && pes.pts !== undefined) {
      id3Track.data.push(pes.data);
      this.id3PesData = null;
    } else {
      // Either pesPkts null or PES truncated, keep it for next frag parsing.
      this.id3PesData = id3Data;
    }

    return videoTrack;
  }

  _parsePAT(data, offset) {
    // Skip the PSI header and parse the first PMT entry.
    return (data[offset + 10] & 0x1F) << 8 | data[offset + 11];
    // console.log('PMT PID:'  + this._pmtId);
  }

  _parsePMT(data, offset) {
    let programInfoLength, pid, result = { audio: -1, video: -1, id3: -1 },
      sectionLength = (data[offset + 1] & 0x0f) << 8 | data[offset + 2],
      tableEnd = offset + 3 + sectionLength - 4;
    // To determine where the table is, we have to figure out how
    // long the program info descriptors are.
    programInfoLength = (data[offset + 10] & 0x0f) << 8 | data[offset + 11];
    // Advance the offset to the first entry in the mapping table.
    offset += 12 + programInfoLength;
    while (offset < tableEnd) {
      pid = (data[offset + 1] & 0x1F) << 8 | data[offset + 2];
      switch (data[offset]) {
        case 0x1c: // MJPEG
        case 0xdb: // SAMPLE-AES AVC.
        case 0x1b: // ITU-T Rec. H.264 and ISO/IEC 14496-10 (lower bit-rate video).
          if (result.video === -1) {
            result.video = pid;
          }
          break;
        case 0xcf: // SAMPLE-AES AAC.
        case 0x0f: // ISO/IEC 13818-7 ADTS AAC (MPEG-2 lower bit-rate audio).
        case 0xd2: // ADPCM audio.
        case 0x03: // ISO/IEC 11172-3 (MPEG-1 audio).
        case 0x24:
        // console.warn('HEVC stream type found, not supported for now');
        case 0x04: // or ISO/IEC 13818-3 (MPEG-2 halved sample rate audio).
          if (result.audio === -1) {
            result.audio = pid;
          }
          break;
        case 0x15: // Packetized metadata (ID3)
          // console.log('ID3 PID:'  + pid);
          if (result.id3 === -1) {
            result.id3 = pid;
          }
          break;
        default:
          // console.log('unknown stream type:' + data[offset]);
          break;
      }
      // Move to the next table entry, skip past the elementary stream descriptors, if present.
      offset += ((data[offset + 3] & 0x0F) << 8 | data[offset + 4]) + 5;
    }
    return result;
  }

  _parsePES(stream) {
    let i = 0, frag, pesFlags, pesPrefix, pesLen, pesHdrLen, pesData, pesPts, pesDts, payloadStartOffset, data = stream.data;
    // Safety check.
    if (!stream || stream.size === 0) {
      return null;
    }

    // We might need up to 19 bytes to read PES header.
    // If first chunk of data is less than 19 bytes, let's merge it with following ones until we get 19 bytes.
    // Usually only one merge is needed (and this is rare ...).
    while (data[0].length < 19 && data.length > 1) {
      let newData = new Uint8Array(data[0].length + data[1].length);
      newData.set(data[0]);
      newData.set(data[1], data[0].length);
      data[0] = newData;
      data.splice(1, 1);
    }
    // Retrieve PTS/DTS from first fragment.
    frag = data[0];
    pesPrefix = (frag[0] << 16) + (frag[1] << 8) + frag[2];
    if (pesPrefix === 1) {
      pesLen = (frag[4] << 8) + frag[5];
      // If PES parsed length is not zero and greater than total received length, stop parsing. PES might be truncated.
      // Minus 6 : PES header size.
      if (pesLen && pesLen > stream.size - 6) {
        return null;
      }

      pesFlags = frag[7];
      if (pesFlags & 0xC0) {
        // PES header described here : http://dvd.sourceforge.net/dvdinfo/pes-hdr.html
        // As PTS / DTS is 33 bit we cannot use bitwise operator in JS,
        // as Bitwise operators treat their operands as a sequence of 32 bits.
        pesPts = (frag[9] & 0x0E) * 536870912 +// 1 << 29
          (frag[10] & 0xFF) * 4194304 +// 1 << 22
          (frag[11] & 0xFE) * 16384 +// 1 << 14
          (frag[12] & 0xFF) * 128 +// 1 << 7
          (frag[13] & 0xFE) / 2;
        // Check if greater than 2^32 -1.
        if (pesPts > 4294967295) {
          // Decrement 2^33.
          pesPts -= 8589934592;
        }
        if (pesFlags & 0x40) {
          pesDts = (frag[14] & 0x0E) * 536870912 +// 1 << 29
            (frag[15] & 0xFF) * 4194304 +// 1 << 22
            (frag[16] & 0xFE) * 16384 +// 1 << 14
            (frag[17] & 0xFF) * 128 +// 1 << 7
            (frag[18] & 0xFE) / 2;
          // Check if greater than 2^32 -1.
          if (pesDts > 4294967295) {
            // Decrement 2^33.
            pesDts -= 8589934592;
          }
          if (pesPts - pesDts > 60 * 90000) {
            // console.warn(`${Math.round((pesPts - pesDts) / 90000)}s delta between PTS and DTS, align them`);
            pesPts = pesDts;
          }
        } else {
          pesDts = pesPts;
        }
      }
      pesHdrLen = frag[8];
      // 9 bytes : 6 bytes for PES header + 3 bytes for PES extension.
      payloadStartOffset = pesHdrLen + 9;

      stream.size -= payloadStartOffset;
      // Reassemble PES packet.
      pesData = new Uint8Array(stream.size);
      for (let j = 0, dataLen = data.length; j < dataLen; j++) {
        frag = data[j];
        let len = frag.byteLength;
        if (payloadStartOffset) {
          if (payloadStartOffset > len) {
            // Trim full frag if PES header bigger than frag.
            payloadStartOffset -= len;
            continue;
          } else {
            // Trim partial frag if PES header smaller than frag.
            frag = frag.subarray(payloadStartOffset);
            len -= payloadStartOffset;
            payloadStartOffset = 0;
          }
        }
        pesData.set(frag, i);
        i += len;
      }
      if (pesLen) {
        // Payload size : remove PES header + PES extension.
        pesLen -= pesHdrLen + 3;
      }
      return { data: pesData, pts: pesPts, dts: pesDts, len: pesLen };
    } else {
      return null;
    }
  }
}

export default MTSDemuxer