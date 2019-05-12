package main

import (
	"context"
	"log"

	"github.com/HotCodeGroup/warscript-utils/models"

	"google.golang.org/grpc"
)

func main() {
	grcpConn, err := grpc.Dial(
		"127.0.0.1:8094",
		grpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("cant connect to grpc")
	}
	defer grcpConn.Close()

	notifyManager := models.NewNotifyClient(grcpConn)
	_, err = notifyManager.SendNotify(
		context.Background(),
		&models.Message{
			Type: "match",
			User: 1,
			Game: "pong",
			Body: []byte(`{"message":"hello"}`),
		},
	)

	if err != nil {
		log.Fatalf("send err: %v", err)
	}
}
