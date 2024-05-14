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

/**
 * @readonly
 * @enum {string}
 */
export const PlaylistContextType = {
  MANIFEST: 'manifest',
  LEVEL: 'level',
  AUDIO_TRACK: 'audioTrack',
  SUBTITLE_TRACK: 'subtitleTrack'
}

/**
 * @enum {string}
 */
export const PlaylistLevelType = {
  MAIN: 'main',
  AUDIO: 'audio',
  SUBTITLE: 'subtitle'
}
