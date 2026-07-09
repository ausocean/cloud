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

package hardware

import (
	"log"

	"github.com/ausocean/cloud/cmd/oceantv/broadcast"
	"github.com/ausocean/cloud/cmd/oceantv/event"
	"github.com/ausocean/cloud/datastore"
)

type Context struct {
	store datastore.Store
	Cfg   *broadcast.Config
	Bus   event.EventBus

	// When nil, defaults to log.Println. Useful to plug in test implementation.
	logOutput func(v ...any)
}

func (ctx *Context) Log(msg string, args ...interface{}) {
	// If context has nil log output, use standard logger log.Println.
	if ctx.logOutput == nil {
		ctx.logOutput = log.Println
	}
	broadcast.LogForBroadcast(ctx.Cfg, ctx.logOutput, msg, args...)
}

type Manager interface {
	Voltage(ctx *Context) (float64, error)
	AlarmVoltage(ctx *Context) (float64, error)
	IsUp(ctx *Context, mac string) (bool, error)
	Start(ctx *Context)
	Shutdown(ctx *Context)
	Stop(ctx *Context)
	PublishEventIfStatus(ctx *Context, e event.Event, status bool, mac int64, store datastore.Store, log func(format string, args ...interface{}), publish func(e event.Event))
	Error(ctx *Context) (error, error)
}
