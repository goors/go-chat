# Multiuser Golang chat for console use

- uses Redis in case of scale to track users accross multiple replicas
- supports nc
- support telnet

### How to run gp server?
- go run [chan_chat_server.go](chan_chat_server.go)

### How to run gp client?
- go run [chan_chat_client.go](chan_chat_client.go)

### Command
- /exit -> exist chat
- /online -> list online users

### Build
- go build -o client chan_chat_client.go
- go build -o server chan_chat_server.go

### Run buil
- ./server
- ./client

# TODO
- add admin
- change user role
- kick
- 1 on 1

### Purpose of project
- tech interview