package repository

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	client *dynamodb.Client
	prefix string
)

func Init(ctx context.Context) error {
	prefix = os.Getenv("TABLE_PREFIX")
	if prefix == "" {
		prefix = "badminton"
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}
	client = dynamodb.NewFromConfig(cfg)
	return ensureTables(ctx)
}

func TableName(name string) string {
	return prefix + "-" + name
}

func Client() *dynamodb.Client {
	return client
}

func ensureTables(ctx context.Context) error {
	tables := []struct {
		name string
		pk   string
		sk   string
	}{
		{"orgs", "org_id", ""},
		{"org-members", "org_id", "member_id"},
		{"sessions", "session_id", ""},
		{"session-players", "session_id", "player_id"},
		{"courts", "session_id", "court_id"},
		{"session-history", "org_id", "closed_at_session"},
	}

	for _, t := range tables {
		if err := createTableIfNotExists(ctx, t.name, t.pk, t.sk); err != nil {
			return err
		}
	}
	return nil
}

func createTableIfNotExists(ctx context.Context, name, pk, sk string) error {
	tableName := TableName(name)

	keySchema := []types.KeySchemaElement{
		{AttributeName: aws.String(pk), KeyType: types.KeyTypeHash},
	}
	attrDefs := []types.AttributeDefinition{
		{AttributeName: aws.String(pk), AttributeType: types.ScalarAttributeTypeS},
	}
	if sk != "" {
		keySchema = append(keySchema, types.KeySchemaElement{
			AttributeName: aws.String(sk), KeyType: types.KeyTypeRange,
		})
		attrDefs = append(attrDefs, types.AttributeDefinition{
			AttributeName: aws.String(sk), AttributeType: types.ScalarAttributeTypeS,
		})
	}

	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:            aws.String(tableName),
		KeySchema:            keySchema,
		AttributeDefinitions: attrDefs,
		BillingMode:          types.BillingModePayPerRequest,
	})
	if err != nil {
		// ignore "already exists" error
		var resErr *types.ResourceInUseException
		if !errorAs(err, &resErr) {
			return fmt.Errorf("create table %s: %w", tableName, err)
		}
	}
	return nil
}

func errorAs(err error, target interface{}) bool {
	type causer interface{ As(interface{}) bool }
	if c, ok := err.(causer); ok {
		return c.As(target)
	}
	return false
}
