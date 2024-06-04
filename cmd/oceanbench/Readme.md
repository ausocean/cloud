# Readme

Ocean Bench, part of Ausocean's [Cloud Blue](https://www.cloudblue.org),
is a cloud service for analyzing ocean data.

Ocean Bench is written in Go and runs on Google App Engine
Standard Edition (part of Google Cloud.)

## Installation and Usage

Before you begin, make sure you have git, go and npm installed. If not, you 
can follow the official guides:

* [git website](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)
* [go website](https://go.dev/doc/install)
* [npm website](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm)

1.  Clone the repository:
    ```bash
    git clone https://github.com/ausocean/cloud.git
2.  Change to the project directory:
    ```bash
    cd cmd/oceanbench
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
    ./vidgrind --standalone

# See Also

* [Ocean Bench service](https://bench.cloudblue.org)
* [AusOcean](https://www.ausocean.org)

