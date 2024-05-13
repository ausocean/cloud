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

import EventEmitter from '../eventemitter3/index.js';

/**
 * Simple adapter sub-class of Nodejs-like EventEmitter.
 */
export class Observer extends EventEmitter {
  /**
   * We simply want to pass along the event-name itself
   * in every call to a handler, which is the purpose of our `trigger` method
   * extending the standard API.
   */
  trigger(event, ...data) {
    this.emit(event, event, ...data);
  }
}
