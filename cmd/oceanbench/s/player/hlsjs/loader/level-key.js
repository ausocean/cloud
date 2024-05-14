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

import URLToolkit from '../../url-toolkit/url-toolkit.js';

export default class LevelKey {
  constructor(baseURI, relativeURI) {
    this._uri = null;

    this.baseuri;
    this.reluri;
    this.method = null;
    this.key = null;
    this.iv = null;

    this.baseuri = baseURI;
    this.reluri = relativeURI;
  }

  get uri() {
    if (!this._uri && this.reluri) {
      this._uri = URLToolkit.buildAbsoluteURL(this.baseuri, this.reluri, { alwaysNormalize: true });
    }

    return this._uri;
  }
}
