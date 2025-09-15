package main

import (
	"fmt"
	"grpcchatserver/internal/config"
	"grpcchatserver/internal/funcs"
	"grpcchatserver/proto"
	"net"

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

	proto.RegisterChatServiceServer(s, &funcs.ChatServer{Manager: manager})
	fmt.Println("Сервер запущен")
	if err := s.Serve(lis); err != nil {
		fmt.Printf("Ошибка запуска сервера: %v\n", err)
	}
}
