package main

import (
	"context"
	"google.golang.org/grpc/codes"
	"log"
	"net"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/streadway/amqp"
	"google.golang.org/grpc"

	pb "github.com/maulerrr/chatapp/protobuf/pb"
)

type chatServer struct {
	pb.UnimplementedChatServiceServer
}

type channel struct {
	name     string
	messages []pb.MessageResponse
	sync.Mutex
}

var channels map[string]*channel
var conn *amqp.Connection
var ch *amqp.Channel

func (s *chatServer) JoinChannel(req *pb.ChannelRequest, stream pb.ChatService_JoinChannelServer) error {
	channelName := req.Username

	if _, ok := channels[channelName]; !ok {
		channels[channelName] = &channel{
			name: channelName,
		}
	}

	currentChannel := channels[channelName]

	for _, msg := range currentChannel.messages {
		if err := stream.Send(&msg); err != nil {
			return err
		}
	}

	ch.QueueBind(channelName, channelName, "chat-exchange", false, nil)

	msgs, err := ch.Consume(
		channelName,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatal(err)
	}

	for msg := range msgs {
		var message pb.MessageResponse
		err := proto.Unmarshal(msg.Body, &message)
		if err != nil {
			log.Fatal(err)
		}

		if err := stream.Send(&message); err != nil {
			return err
		}
	}

	return nil
}

func (s *chatServer) SendMessage(ctx context.Context, req *pb.MessageRequest) (*pb.MessageResponse, error) {
	sender := req.Sender
	recipient := req.Recipient
	content := req.Content

	message := &pb.MessageResponse{
		Sender:  sender,
		Content: content,
	}

	if recipient != "" {
		if _, ok := channels[recipient]; ok {
			ch.Publish(
				"chat-exchange",
				recipient,
				false,
				false,
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte(proto.MarshalTextString(message)),
				},
			)
		} else {
			return nil, grpc.Errorf(codes.NotFound, "Recipient not found")
		}
	} else {
		for _, channel := range channels {
			channel.Lock()
			channel.messages = append(channel.messages, *message)
			channel.Unlock()

			ch.Publish(
				"chat-exchange",
				channel.name,
				false,
				false,
				amqp.Publishing{
					ContentType: "text/plain",
					Body:        []byte(proto.MarshalTextString(message)),
				},
			)
		}
	}

	return message, nil
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	amqpConn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	conn = amqpConn

	amqpChannel, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a RabbitMQ channel: %v", err)
	}
	ch = amqpChannel

	ch.ExchangeDeclare(
		"chat-exchange",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)

	defer func(conn *amqp.Connection) {
		err := conn.Close()
		if err != nil {
			log.Println("Error in connection..")
		}
	}(conn)
	defer func(ch *amqp.Channel) {
		err := ch.Close()
		if err != nil {
			log.Println("Error in channel..")
		}
	}(ch)

	channels = make(map[string]*channel)

	s := grpc.NewServer()
	pb.RegisterChatServiceServer(s, &chatServer{})

	log.Println("Chat server started")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
