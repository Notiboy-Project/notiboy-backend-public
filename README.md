
Notiboy API

# notiboy-backend
Powers NotiBoy Notification Service


## Prerequisites

- Run a Cassandra database instance
  - `docker run --rm -d --name cassandra --hostname cassandra -p 9042:9042 cassandra`
  - If absent, schemas will be automatically created at app start up

## Build

    git clone https://github.com/Notiboy-Project/notiboy-backend-public.git
    make build

## Install

  
    cd notiboy-backend-public
    go build -o ~ ./...
    ./notiboy
