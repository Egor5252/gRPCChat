package main

import (
	"fmt"
	serverconfig "grpcchat/internal/server_config"
	serverfuncs "grpcchat/internal/server_funcs"
	"grpcchat/proto"
	"net"

	"google.golang.org/grpc"
)

func main() {
	cfg := serverconfig.MustLoad()
	fmt.Println(cfg)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%v", cfg.ServerPort))
	if err != nil {
		fmt.Printf("Ошибка прослушивания: %v\n", err)
		return
	}
	s := grpc.NewServer()

	manager := serverfuncs.NewClientManager()
	go manager.Run()

	proto.RegisterChatServiceServer(s, &serverfuncs.ChatServer{Manager: manager})
	fmt.Println("Сервер запущен")
	if err := s.Serve(lis); err != nil {
		fmt.Printf("Ошибка запуска сервера: %v\n", err)
	}
}
