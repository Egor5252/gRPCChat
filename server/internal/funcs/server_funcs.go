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

	ctx, cancel := context.WithCancel(stream.Context())

	client := &Client{
		ID:       initMsg.GetFrom(),
		SendChan: make(chan *proto.ChatMessage, 10),
		Stream:   stream,
		Cancel:   cancel,
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
				client.Cancel()
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
				client.Cancel()
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
				ctx, cancel := context.WithTimeout(regClient.Stream.Context(), 10*time.Second)
				defer cancel()

				m.Mu.Lock()
				exist := m.Clients[regClient.ID] != nil
				m.Mu.Unlock()

				if exist {
					select {
					case regClient.SendChan <- &proto.ChatMessage{
						From:      "System",
						To:        regClient.ID,
						Type:      "system",
						Content:   fmt.Sprintf("ID %v уже занят", regClient.ID),
						Timestamp: time.Now().Unix(),
					}:
						time.Sleep(100 * time.Millisecond)
						regClient.ID = ""
						regClient.Cancel()

					case <-ctx.Done():
						regClient.Stream.Send(&proto.ChatMessage{
							From:      "System",
							To:        regClient.ID,
							Type:      "error",
							Content:   "Ошибка на стороне сервера при регистрации клиента. Повторите попытку",
							Timestamp: time.Now().Unix(),
						})
						regClient.ID = ""
						regClient.Cancel()
					}
				} else {
					m.Mu.Lock()
					m.Clients[regClient.ID] = regClient
					m.Mu.Unlock()
					select {
					case m.Broadcast <- &proto.ChatMessage{
						From:      "System",
						Type:      "system",
						Content:   fmt.Sprintf("%v присоединился", regClient.ID),
						Timestamp: time.Now().Unix(),
					}:

					case <-ctx.Done():
					}
				}
			}(m, regClient)

		case unregClient := <-m.Unregister:
			go func(m *ClientManager, unregClient *Client) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				m.Mu.Lock()
				client, ok := m.Clients[unregClient.ID]
				m.Mu.Unlock()

				if ok {
					m.Mu.Lock()
					delete(m.Clients, client.ID)
					m.Mu.Unlock()

					select {
					case m.Broadcast <- &proto.ChatMessage{
						From:      "System",
						Type:      "system",
						Content:   fmt.Sprintf("%v покинул чат", unregClient.ID),
						Timestamp: time.Now().Unix(),
					}:

					case <-ctx.Done():
					}
				}
			}(m, unregClient)

		case msg := <-m.Broadcast:
			go func(m *ClientManager, msg *proto.ChatMessage) {
				if to := msg.GetTo(); to == "" {
					m.Mu.Lock()
					clients := make([]*Client, 0, len(m.Clients))
					for _, c := range m.Clients {
						if c.ID != msg.From {
							clients = append(clients, c)
						}
					}
					m.Mu.Unlock()

					for _, client := range clients {
						ctx, cancel := context.WithTimeout(client.Stream.Context(), 10*time.Second)

						select {
						case client.SendChan <- msg:

						case <-ctx.Done():
							//Do somefing
						}

						cancel()
					}
				} else {
					m.Mu.Lock()
					client, ok := m.Clients[to]
					m.Mu.Unlock()
					if ok {
						ctx, cancel := context.WithTimeout(client.Stream.Context(), 10*time.Second)

						select {
						case client.SendChan <- msg:

						case <-ctx.Done():
							//Do somefing
						}

						cancel()
					}
				}
			}(m, msg)
		}
	}
}
