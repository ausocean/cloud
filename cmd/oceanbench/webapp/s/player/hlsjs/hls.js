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

import URLToolkit from '../url-toolkit/url-toolkit.js';
import HlsEvents from './events.js';
import PlaylistLoader from './loader/playlist-loader.js';
import FragmentLoader from './loader/fragment-loader.js';
import StreamController from './controller/stream-controller.js';
import LevelController from './controller/level-controller.js';
import { hlsDefaultConfig } from './config.js';
import { Observer } from './observer.js';

class Hls extends Observer {
  constructor() {
    super();
    this.pLoader = new PlaylistLoader(this);
    this.streamController = new StreamController(this);
    this.levelController = new LevelController(this);
    this.fragmentLoader = new FragmentLoader(this);

    this.config = hlsDefaultConfig;
  }

  // url is the source URL. Can be relative or absolute.
  loadSource(url) {
    this.levelController.startLoad();
    url = URLToolkit.buildAbsoluteURL(window.location.href, url, { alwaysNormalize: true });
    this.trigger(HlsEvents.MANIFEST_LOADING, { url: url });
  }
}

export default Hls