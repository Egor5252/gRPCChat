package main

import (
	"fmt"
	"grpcchatserver/internal/config"
	"grpcchatserver/internal/funcs"
	"grpcchatserver/proto"
	"net"
	"time"

	"google.golang.org/grpc"
)

func main() {
	cfg := config.MustLoad()
	fmt.Println(cfg)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", cfg.ServerPort))
	if err != nil {
		fmt.Printf("Ошибка прослушивания: %v\n", err)
		return
	}
	s := grpc.NewServer()

	manager := funcs.NewClientManager()
	go manager.Run()

	go func(manager *funcs.ClientManager) {
		for {
			fmt.Print("\n\n\n")
			manager.Mu.Lock()
			activeClients := manager.Clients
			for _, client := range activeClients {
				fmt.Printf("%v\n", client.ID)
			}
			manager.Mu.Unlock()
			time.Sleep(1 * time.Second)
		}
	}(manager)

	proto.RegisterChatServiceServer(s, &funcs.ChatServer{Manager: manager})
	fmt.Println("Сервер запущен")
	if err := s.Serve(lis); err != nil {
		fmt.Printf("Ошибка запуска сервера: %v\n", err)
	}
}
