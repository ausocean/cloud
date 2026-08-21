/*
AUTHORS
  Saxon Nelson-Milton <saxon@ausocean.org>

LICENSE
  Copyright (C) 2026 the Australian Ocean Lab (AusOcean)

  This file is part of Ocean TV. Ocean TV is free software: you can
  redistribute it and/or modify it under the terms of the GNU
  General Public License as published by the Free Software
  Foundation, either version 3 of the License, or (at your option)
  any later version.

  Ocean TV is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  in gpl.txt. If not, see <http://www.gnu.org/licenses/>.
*/

package notifier

import (
	"errors"
	"fmt"
	"log"

	"github.com/ausocean/cloud/notify"
)

var N notify.Notifier

var errNoGlobalNotifier = errors.New("global notifier is nil")

// LogAndNotify is intended for use by background processes when an error must be
// indicated, such as within the goLive routine, which monitors and controls
// a broadcast.
func LogAndNotify(notify func(msg string) error, msg string, args ...interface{}) {
	log.Printf(msg, args...)
	err := notify(fmt.Sprintf(msg, args...))
	if err != nil {
		log.Printf("could not send notification: %v", err)
	}
}
