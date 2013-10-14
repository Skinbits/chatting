TARGET=chat_server
OBJS=src/chat_server.go

TARGET:
	GOPATH=${shell pwd} go build $(OBJS)

clean:
	@rm $(TARGET)

