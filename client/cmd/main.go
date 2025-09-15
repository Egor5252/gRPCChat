package main

import (
	"bufio"
	"context"
	"fmt"
	"grpcchatclient/internal/config"
	"grpcchatclient/proto"

	"log"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.MustLoad()
	fmt.Println(cfg)

	conn, err := grpc.NewClient(
		fmt.Sprintf("%v:%v", cfg.ServerIP, cfg.ServerPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Ошибка запуска клиента:", err)
	}
	defer conn.Close()

	client := proto.NewChatServiceClient(conn)
	stream, err := client.Chat(context.Background())
	if err != nil {
		log.Fatalln("Ошибка получения стрима:", err)
	}

	// Отправка системного сообщения (регистрация)
	initMsg := &proto.ChatMessage{
		From:      cfg.ID,
		Type:      "system",
		Timestamp: time.Now().Unix(),
	}

	if err := stream.Send(initMsg); err != nil {
		log.Fatalln("Ошибка инициализации:", err)
	}

	// Получение входящих сообщений
	go receiveLoop(stream)

	// Чтение ввода пользователя и отправка сообщений
	sendLoop(stream, cfg)
}

// ======= Поток получения сообщений =======
func receiveLoop(stream proto.ChatService_ChatClient) {
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Println("Ошибка при получении сообщения:", err)
			break
		}

		fmt.Printf("[%s] %s: %s\n", time.Unix(msg.GetTimestamp(), 0).Format("15:04:05"), msg.GetFrom(), msg.GetContent())
	}
}

// ======= Поток отправки сообщений =======
func sendLoop(stream proto.ChatService_ChatClient, cfg *config.Config) {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		msg := &proto.ChatMessage{
			From:      cfg.ID,
			To:        cfg.To,
			Type:      "text",
			Content:   input,
			Timestamp: time.Now().Unix(),
		}

		if err := stream.Send(msg); err != nil {
			log.Println("Ошибка отправки сообщения:", err)
			break
		}
	}
}
