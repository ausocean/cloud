/*
NAME
  pcm-to-wav.js

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

// pcmToWav takes raw pcm data along with the sample rate, number of channels and bit-depth, 
// and adds a WAV header to it so that it can be read and played by common players.
// Input should be a Uint8Array containing 16 bit PCM samples, output will be a Uint8Array representing the bytes of the wav file.
// WAV spec.: http://soundfile.sapp.org/doc/WaveFormat/
function pcmToWav(data, rate, channels, bitdepth) {
  if (data == undefined || data.length == 0) {
    console.error("no PCM data to convert to WAV")
    return
  }
  let subChunk2ID = [100, 97, 116, 97]; // "data".
  let subChunk2Size = int32ToBytes(data.length);

  let subChunk1ID = [102, 109, 116, 32]; // "fmt ".
  let subChunk1Size = int32ToBytes(16);
  let audioFmt = int16ToBytes(1); // 1 = PCM.
  let numChannels = int16ToBytes(channels);
  let sampleRate = int32ToBytes(rate);
  let byteRate = int32ToBytes(rate * channels * bitdepth / 8);
  let blockAlign = int16ToBytes(channels * bitdepth / 8);
  let bitsPerSample = int16ToBytes(bitdepth)

  let chunkID = [82, 73, 70, 70]; // "RIFF".
  let chunkSize = int32ToBytes(36 + data.length);
  let format = [87, 65, 86, 69]; // "WAVE".

  let result = new Uint8Array((data.length * 2) + 44);
  let off = 0;

  result.set(chunkID, off);
  off += 4;
  result.set(chunkSize, off);
  off += 4;
  result.set(format, off);
  off += 4;
  result.set(subChunk1ID, off);
  off += 4;
  result.set(subChunk1Size, off);
  off += 4;
  result.set(audioFmt, off);
  off += 2;
  result.set(numChannels, off);
  off += 2;
  result.set(sampleRate, off);
  off += 4;
  result.set(byteRate, off);
  off += 4;
  result.set(blockAlign, off);
  off += 2;
  result.set(bitsPerSample, off);
  off += 2;
  result.set(subChunk2ID, off);
  off += 4;
  result.set(subChunk2Size, off);
  off += 4;

  result.set(data, off);

  return result;
}

// int32ToBytes takes a number assumed to be an int 32 and converts it to an array containing bytes (Little Endian).
function int32ToBytes(num) {
  let b = new Uint8Array(4);
  b[0] = (num & 0x000000ff);
  b[1] = (num & 0x0000ff00) >> 8;
  b[2] = (num & 0x00ff0000) >> 16;
  b[3] = (num & 0xff000000) >> 24;
  return b;
}

// int16ToBytes takes a number assumed to be an int 16 and converts it to an array containing bytes (Little Endian).
function int16ToBytes(num) {
  let b = new Uint8Array(2);
  b[0] = (num & 0x00ff);
  b[1] = (num & 0xff00) >> 8;
  return b;
}