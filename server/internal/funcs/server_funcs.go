package funcs

import (
	"context"
	"fmt"
	"grpcchatserver/proto"
	"time"

	"google.golang.org/grpc"
)

func (s *ChatServer) Chat(stream grpc.BidiStreamingServer[proto.ChatMessage, proto.ChatMessage]) error {
	initMsg, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("ошибка отправки сообщения инциализации: %v", err)
	}
	if initMsg.GetType() != "system" {
		return fmt.Errorf("неверная нинциализация")
	}

	ctx, cansel := context.WithCancel(stream.Context())

	client := &Client{
		ID:       initMsg.GetFrom(),
		SendChan: make(chan *proto.ChatMessage),
		Stream:   stream,
		Cansel:   cansel,
	}

	go s.sendToClientLoop(ctx, client)
	go s.recvFromClientLoop(ctx, client)
	time.Sleep(100 * time.Millisecond)

	s.Manager.Register <- client

	<-ctx.Done()
	s.Manager.Unregister <- client

	return nil
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client, 100),
		Unregister: make(chan *Client, 100),
		Broadcast:  make(chan *proto.ChatMessage, 1000),
	}
}

func (s *ChatServer) sendToClientLoop(ctx context.Context, client *Client) {
	for {
		select {
		case msg := <-client.SendChan:
			if err := client.Stream.Send(msg); err != nil {
				client.Cansel()
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (s *ChatServer) recvFromClientLoop(ctx context.Context, client *Client) {
	for {
		select {
		default:
			msg, err := client.Stream.Recv()
			if err != nil {
				client.Cansel()
				return
			}
			s.Manager.Broadcast <- msg

		case <-ctx.Done():
			return
		}
	}
}

func (m *ClientManager) Run() {
	for {
		select {
		case regClient := <-m.Register:
			go func(m *ClientManager, regClient *Client) {
				m.Mu.Lock()
				if m.Clients[regClient.ID] != nil {
					regClient.SendChan <- &proto.ChatMessage{
						From:      "System",
						To:        regClient.ID,
						Type:      "system",
						Content:   fmt.Sprintf("ID %v уже занят", regClient.ID),
						Timestamp: time.Now().Unix(),
					}
					// time.Sleep(100 * time.Millisecond)
					regClient.ID = ""
					regClient.Cansel()
				} else {
					m.Clients[regClient.ID] = regClient
					m.Broadcast <- &proto.ChatMessage{
						From:      "System",
						Type:      "system",
						Content:   fmt.Sprintf("%v присоединился", regClient.ID),
						Timestamp: time.Now().Unix(),
					}
					// time.Sleep(100 * time.Millisecond)
				}
				m.Mu.Unlock()
			}(m, regClient)

		case unregClient := <-m.Unregister:
			go func(m *ClientManager, unregClient *Client) {
				m.Mu.Lock()
				client, ok := m.Clients[unregClient.ID]
				if ok {
					delete(m.Clients, client.ID)
					m.Broadcast <- &proto.ChatMessage{
						From:      "System",
						Type:      "system",
						Content:   fmt.Sprintf("%v покинул чат", unregClient.ID),
						Timestamp: time.Now().Unix(),
					}
					time.Sleep(100 * time.Millisecond)
				}
				m.Mu.Unlock()
			}(m, unregClient)

		case msg := <-m.Broadcast:
			go func(m *ClientManager, msg *proto.ChatMessage) {
				m.Mu.Lock()
				if to := msg.GetTo(); to == "" {
					for _, client := range m.Clients {
						if client.ID != msg.From {
							client.SendChan <- msg
						}
					}
				} else {
					client, ok := m.Clients[to]
					if ok {
						client.SendChan <- msg
					}
				}
				m.Mu.Unlock()
			}(m, msg)
		}
	}
}
