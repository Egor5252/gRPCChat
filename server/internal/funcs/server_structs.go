package funcs

import (
	"context"
	"grpcchatserver/proto"
	"sync"

	"google.golang.org/grpc"
)

type ChatServer struct {
	proto.UnimplementedChatServiceServer
	Manager *ClientManager
}

type Client struct {
	ID       string
	Rooms    []string
	SendChan chan *proto.ChatMessage
	Stream   grpc.BidiStreamingServer[proto.ChatMessage, proto.ChatMessage]
	Cansel   context.CancelFunc
}

type ClientManager struct {
	Clients    map[string]*Client
	Rooms      map[string]map[string]*Client
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *proto.ChatMessage
	mu         sync.RWMutex
}
