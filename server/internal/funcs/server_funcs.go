package funcs

import (
	"context"
	"fmt"
	"grpcchatserver/proto"
	"log"
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
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *proto.ChatMessage),
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
		msg, err := client.Stream.Recv()
		if err != nil {
			client.Cansel()
			return
		}

		s.Manager.Broadcast <- msg
	}
}

func (m *ClientManager) Run() {
	for {
		select {
		case regClient := <-m.Register:
			m.mu.Lock()
			if m.Clients[regClient.ID] != nil {
				regClient.SendChan <- &proto.ChatMessage{
					From:      "System",
					To:        regClient.ID,
					Type:      "system",
					Content:   fmt.Sprintf("ID %v уже занят", regClient.ID),
					Timestamp: time.Now().Unix(),
				}
				time.Sleep(100 * time.Millisecond)
				regClient.Cansel()
			} else {
				m.Clients[regClient.ID] = regClient
				log.Printf("Клиент %v подкдючился", regClient.ID)
			}
			m.mu.Unlock()

		case unregClient := <-m.Unregister:
			m.mu.Lock()
			client, ok := m.Clients[unregClient.ID]
			if ok {
				delete(m.Clients, client.ID)
			}
			m.mu.Unlock()

		case msg := <-m.Broadcast:
			if to := msg.GetTo(); to == "" {
				for _, client := range m.Clients {
					client.SendChan <- msg
				}
			} else {
				client, ok := m.Clients[to]
				if ok {
					client.SendChan <- msg
				}
			}

		}
	}

}
