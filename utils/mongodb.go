package utils

import (
	"context"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ConnectMongo(ctx context.Context) (*mongo.Database, error) {
	mongoUri := "mongodb://localhost:27017"
	if os.Getenv("MONGO_URI") != "" {
		mongoUri = os.Getenv("MONGO_URI")
	}

	mongoDB := "flowee"
	if os.Getenv("MONGO_DB") != "" {
		mongoDB = os.Getenv("MONGO_DB")
	}

	opts := options.Client().ApplyURI(mongoUri)
	client, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}

	return client.Database(mongoDB), nil
}