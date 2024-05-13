# Readme

Ocean Bench, a part of AusOcean's cloud services, is a web app for
working with ocean data.

It is written in Go and runs on Google App Engine Standard Edition
(part of Google Cloud.)

## Installation and Usage

Before you begin, make sure you have git, go and npm installed. If not, you 
can follow the official guides:

* [git website](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)
* [go website](https://go.dev/doc/install)
* [npm website](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm)

1.  Clone the ausocean/cloud repository:
    ```bash
    git clone https://github.com/ausocean/cloud.git
2.  Change to the project directory:
    ```bash
    cd cloud/cmd/oceanbench
3.  Install node dependencies from package.json:
    ```bash
    npm install
4.  Compile typescript:
    ```bash
    npm run build
5.  Compile Go:
    ```bash
    go build
6.  Run a local instance:
    ```bash
    ./oceanbench --standalone

# See Also

* [Ocean Bench](https://bench.ausocean.org)
* [AusOcean](https://www.ausocean.org)

# License

Copyright (C) 2019-2024 the Australian Ocean Lab (AusOcean).

It is free software: you can redistribute it and/or modify them
under the terms of the GNU General Public License as published by the
Free Software Foundation, either version 3 of the License, or (at your
option) any later version.

It is distributed in the hope that it will be useful, but WITHOUT
ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
for more details.

You should have received a copy of the GNU General Public License
along with NetReceiver in gpl.txt. If not, see
<http://www.gnu.org/licenses>.