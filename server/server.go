package main

import (
	"context"
	"log"
	"net"

	"github.com/streadway/amqp"
	"google.golang.org/grpc"

	pb "github.com/maulerrr/chatapp/protobuf/pb"
)

const (
	rabbitMQURL    = "amqp://guest:guest@localhost:5672/"
	exchangeName   = "chat_exchange"
	exchangeType   = "direct"
	queueName      = "chat_queue"
	routingKey     = "chat_messages"
	grpcServerPort = ":50051"
)

type server struct {
	pb.UnimplementedChatServer
}

func (s *server) SendMessage(ctx context.Context, msg *pb.Message) (*pb.Empty, error) {
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		exchangeName,
		exchangeType,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare an exchange: %v", err)
	}

	body := "[" + msg.User + "]: " + msg.Content
	err = ch.Publish(
		exchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		},
	)
	if err != nil {
		log.Fatalf("Failed to publish a message: %v", err)
	}

	return &pb.Empty{}, nil
}

func (s *server) ReceiveMessage(empty *pb.Empty, stream pb.Chat_ReceiveMessageServer) error {
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(
		exchangeName,
		exchangeType,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare an exchange: %v", err)
	}

	q, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to declare a queue: %v", err)
	}

	err = ch.QueueBind(
		q.Name,
		routingKey,
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to bind a queue: %v", err)
	}

	msgs, err := ch.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	for msg := range msgs {
		chatMsg := &pb.Message{
			User:    "",
			Content: string(msg.Body),
		}

		err = stream.Send(chatMsg)
		if err != nil {
			log.Fatalf("Failed to send a message to the client: %v", err)
		}
	}

	return nil
}

func main() {
	lis, err := net.Listen("tcp", grpcServerPort)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterChatServer(s, &server{})
	log.Println("gRPC server listening on", grpcServerPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
