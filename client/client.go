package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc"

	pb "github.com/maulerrr/chatapp/protobuf/pb"
)

const (
	grpcServerAddr = "localhost:50051"
)

var printLock sync.Mutex

func receiveMessages(stream pb.Chat_ReceiveMessageClient, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		chatMsg, err := stream.Recv()
		if err != nil {
			log.Fatalf("Failed to receive a message from the server: %v", err)
		}

		printLock.Lock()
		fmt.Printf("%s\n", chatMsg.User+": "+chatMsg.Content)
		printLock.Unlock()
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: go run ./client/client.go <username>")
	}

	username := os.Args[1]

	conn, err := grpc.Dial(grpcServerAddr, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewChatClient(conn)

	sendWg := sync.WaitGroup{}
	receiveWg := sync.WaitGroup{}

	sendWg.Add(1)
	receiveWg.Add(1)

	stream, err := client.ReceiveMessage(context.Background(), &pb.Empty{})
	if err != nil {
		log.Fatalf("Failed to receive messages from the server: %v", err)
	}

	go receiveMessages(stream, &receiveWg)

	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("Welcome, %s!\n", username)

	for {
		//fmt.Print(username + ": ")
		userInput, err := reader.ReadString('\n')
		if err != nil {
			log.Fatalf("Failed to read user input: %v", err)
		}
		userInput = strings.TrimSuffix(userInput, "\n") // remove the newline character

		if userInput == "/exit" {
			break
		}

		sendWg.Add(1)

		if userInput == "" {
			log.Println("Cannot send empty messages..")
		} else {
			go func() {
				defer sendWg.Done()

				_, err := client.SendMessage(context.Background(), &pb.Message{
					User:    username,
					Content: userInput,
				})
				if err != nil {
					log.Fatalf("Failed to send a message to the server: %v", err)
				}
			}()
		}

	}

	sendWg.Wait()

	err = stream.CloseSend()
	if err != nil {
		log.Fatalf("Failed to close the stream: %v", err)
	}

	receiveWg.Wait()
}
