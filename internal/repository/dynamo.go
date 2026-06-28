package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var (
	client   *dynamodb.Client
	s3Client *s3.Client
	prefix   string
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
	s3Client = s3.NewFromConfig(cfg)
	// best-effort: never crash the function if a table can't be created.
	// Errors are logged so they surface in CloudWatch and via /health.
	ensureTables(ctx)
	ensureSessionStatusGSI(ctx)
	ensurePlayersTable(ctx)
	ensureActionLogTTL(ctx)
	return nil
}

// ensureActionLogTTL turns on DynamoDB TTL for the action-logs table so old
// rows (expires_at < now) are auto-deleted (~90 day retention). Idempotent.
func ensureActionLogTTL(ctx context.Context) {
	tbl := TableName("action-logs")
	desc, err := client.DescribeTimeToLive(ctx, &dynamodb.DescribeTimeToLiveInput{TableName: aws.String(tbl)})
	if err == nil && desc.TimeToLiveDescription != nil {
		st := desc.TimeToLiveDescription.TimeToLiveStatus
		if st == types.TimeToLiveStatusEnabled || st == types.TimeToLiveStatusEnabling {
			return // already on
		}
	}
	_, err = client.UpdateTimeToLive(ctx, &dynamodb.UpdateTimeToLiveInput{
		TableName: aws.String(tbl),
		TimeToLiveSpecification: &types.TimeToLiveSpecification{
			Enabled:       aws.Bool(true),
			AttributeName: aws.String("expires_at"),
		},
	})
	if err != nil {
		log.Printf("ensureActionLogTTL: %v", err)
	}
}

// ensureSessionStatusGSI adds a status GSI to the sessions table (idempotent,
// best-effort) so the lobby can Query open sessions instead of full-table Scan.
func ensureSessionStatusGSI(ctx context.Context) {
	tbl := TableName("sessions")
	desc, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: aws.String(tbl)})
	if err != nil {
		return
	}
	for _, gsi := range desc.Table.GlobalSecondaryIndexes {
		if gsi.IndexName != nil && *gsi.IndexName == "status-index" {
			return // already present
		}
	}
	_, err = client.UpdateTable(ctx, &dynamodb.UpdateTableInput{
		TableName: aws.String(tbl),
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("status"), AttributeType: types.ScalarAttributeTypeS},
		},
		GlobalSecondaryIndexUpdates: []types.GlobalSecondaryIndexUpdate{
			{Create: &types.CreateGlobalSecondaryIndexAction{
				IndexName: aws.String("status-index"),
				KeySchema: []types.KeySchemaElement{
					{AttributeName: aws.String("status"), KeyType: types.KeyTypeHash},
				},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			}},
		},
	})
	if err != nil {
		log.Printf("ensureSessionStatusGSI: %v", err)
	}
}

func TableName(name string) string {
	return prefix + "-" + name
}

func Client() *dynamodb.Client {
	return client
}

// lastTableError records the most recent table-setup failure for /health.
var lastTableError string

func ensureTables(ctx context.Context) {
	tables := []struct {
		name string
		pk   string
		sk   string
	}{
		{"orgs", "org_id", ""},
		{"sessions", "session_id", ""},
		{"session-players", "session_id", "player_id"},
		{"courts", "session_id", "court_id"},
		{"session-history", "org_id", "closed_at_session"},
		{"game-logs", "session_id", "ended_at_id"},
		{"push-subscriptions", "player_id", ""},
		{"action-logs", "session_id", "ts_id"},
	}

	for _, t := range tables {
		if err := createTableIfNotExists(ctx, t.name, t.pk, t.sk); err != nil {
			lastTableError = err.Error()
			log.Printf("ensureTables: %v", err)
		}
	}
}

// LastTableError exposes the most recent table-setup failure (for /health).
func LastTableError() string {
	return lastTableError
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
		// ignore "already exists" — use stdlib errors.As, which correctly
		// unwraps the SDK's *smithy.OperationError chain.
		var resErr *types.ResourceInUseException
		if !errors.As(err, &resErr) {
			return fmt.Errorf("create table %s: %w", tableName, err)
		}
	}
	return nil
}
