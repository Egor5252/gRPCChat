package serverfuncs

import (
	"context"
	"fmt"
	"grpcchat/proto"
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
		Rooms:    initMsg.GetRooms(),
		SendChan: make(chan *proto.ChatMessage, 100),
		Stream:   stream,
		Cansel:   cansel,
	}
	s.Manager.Register <- client

	go s.sendLoop(ctx, client)    // Отправка сообщений клиенту
	go s.receiveLoop(ctx, client) // Получение сообщений от клиента

	select {
	case <-stream.Context().Done():
		s.Manager.Unregister <- client
		return nil

	case <-ctx.Done():
		return nil
	}
}

func (s *ChatServer) sendLoop(ctx context.Context, client *Client) {
	for {
		select {
		case msg := <-client.SendChan:
			err := client.Stream.Send(msg)
			if err != nil {
				if err.Error() != "rpc error: code = Unavailable desc = transport is closing" {
					log.Printf("Ошибка отправки сообщения клиенту %v: %v", client.ID, err)
				}
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

func (s *ChatServer) receiveLoop(ctx context.Context, client *Client) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			msg, err := client.Stream.Recv()
			if err != nil {
				// Контекст уже отменён? Просто выйдем без лишнего шума
				if ctx.Err() != nil {
					return
				}

				if err.Error() != "rpc error: code = Canceled desc = context canceled" {
					log.Printf("Ошибка получения сообщения от клиента %v: %v", client.ID, err)
				}
				return
			}

			s.Manager.Broadcast <- msg
		}
	}
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		Clients:    make(map[string]*Client),
		Rooms:      make(map[string]map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *proto.ChatMessage),
	}
}

func (m *ClientManager) Run() {
	for {
		select {
		case client := <-m.Register:
			m.mu.Lock()
			if m.Clients[client.ID] != nil {
				client.SendChan <- &proto.ChatMessage{
					From:      "System",
					Type:      "system",
					Content:   fmt.Sprintf("ID %v уже занят", client.ID),
					Timestamp: time.Now().Unix(),
				}
				client.Cansel()
			} else {
				m.Clients[client.ID] = client
				for _, room := range client.Rooms {
					if m.Rooms[room] == nil {
						m.Rooms[room] = make(map[string]*Client)
					}
					m.Rooms[room][client.ID] = client
					go func(room, ID string) {
						time.Sleep(100 * time.Millisecond)
						m.Broadcast <- &proto.ChatMessage{
							From:      "System",
							To:        room,
							Type:      "system",
							Content:   fmt.Sprintf("Пользователь %v присоединился", ID),
							Timestamp: time.Now().Unix(),
						}
					}(room, client.ID)
				}
				log.Printf("Клиент %s присоединился", client.ID)
			}
			m.mu.Unlock()

		case client := <-m.Unregister:
			m.mu.Lock()
			delete(m.Clients, client.ID)
			for _, room := range client.Rooms {
				clientsRoom, ok := m.Rooms[room]
				if ok {
					delete(clientsRoom, client.ID)
					if len(clientsRoom) == 0 {
						delete(m.Rooms, room)
					}
				}
			}
			close(client.SendChan)
			m.mu.Unlock()
			log.Printf("Клиент %s отсоединился", client.ID)

		case msg := <-m.Broadcast:
			m.mu.RLock()
			for _, client := range m.Rooms[msg.GetTo()] {
				if client.ID != msg.GetFrom() {
					client.SendChan <- msg
				}
			}
			m.mu.RUnlock()
		}
	}
}
