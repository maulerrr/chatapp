package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"

	pb "github.com/maulerrr/chatapp/protobuf/pb"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatServiceClient(conn)

	reader := bufio.NewReader(os.Stdin)

	log.Print("Enter your username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	stream, err := client.JoinChannel(context.Background(), &pb.ChannelRequest{Username: username})
	if err != nil {
		log.Fatalf("Error joining channel: %v", err)
	}

	go func() {
		for {
			msg, err := stream.Recv()
			if err != nil {
				log.Fatalf("Error receiving message: %v", err)
			}

			log.Printf("[%s]: %s", msg.Sender, msg.Content)
		}
	}()

	for {
		log.Print("Enter message: ")
		message, _ := reader.ReadString('\n')
		message = strings.TrimSpace(message)

		if message == "" {
			continue
		}

		split := strings.SplitN(message, "#", 3)
		if len(split) == 3 && split[0] == "" {
			recipient := strings.TrimSpace(split[1])
			content := strings.TrimSpace(split[2])

			req := &pb.MessageRequest{
				Sender:    username,
				Recipient: recipient,
				Content:   content,
			}

			_, err := client.SendMessage(context.Background(), req)
			if err != nil {
				log.Fatalf("Error sending message: %v", err)
			}
		} else {
			req := &pb.MessageRequest{
				Sender:  username,
				Content: message,
			}

			_, err := client.SendMessage(context.Background(), req)
			if err != nil {
				log.Fatalf("Error sending message: %v", err)
			}
		}
	}
}
