/*
DESCRIPTION
  This file contains a simple file server which serves the audio player page.
  This is because it is currently its own site, disconnected from vidgrind. The intent is to
  integrate it with vidgrind so we will eventually no longer need this file.

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

package main

import (
	"flag"
	"log"
	"net/http"
)

var (
	listen = flag.String("listen", ":8080", "listen address")
	dir    = flag.String("dir", ".", "directory to serve")
)

func main() {
	flag.Parse()
	log.Printf("listening on %q...", *listen)
	log.Fatal(http.ListenAndServe(*listen, http.FileServer(http.Dir(*dir))))
}
