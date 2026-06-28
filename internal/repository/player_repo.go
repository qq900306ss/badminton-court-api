package repository

import (
	"context"
	"errors"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/qq900306ss/badminton-court-api/internal/model"
)

func PutPlayer(ctx context.Context, p model.Player) error {
	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("players")),
		Item:      item,
	})
	return err
}

func GetPlayer(ctx context.Context, playerID string) (*model.Player, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName("players")),
		Key:       map[string]types.AttributeValue{"player_id": &types.AttributeValueMemberS{Value: playerID}},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	var p model.Player
	if err := attributevalue.UnmarshalMap(out.Item, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetPlayerByProvider finds an account by "<provider>#<sub>" via the provider-index
// GSI. Returns (nil, nil) when no account exists yet (→ caller creates one).
func GetPlayerByProvider(ctx context.Context, providerKey string) (*model.Player, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:                 aws.String(TableName("players")),
		IndexName:                 aws.String("provider-index"),
		KeyConditionExpression:    aws.String("provider_key = :k"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":k": &types.AttributeValueMemberS{Value: providerKey}},
		Limit:                     aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	var p model.Player
	if err := attributevalue.UnmarshalMap(out.Items[0], &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ensurePlayersTable creates the players table WITH its provider-index GSI in a
// single CreateTable (so first login works immediately — no create-then-update
// race). Idempotent + best-effort, called from Init.
func ensurePlayersTable(ctx context.Context) {
	_, err := client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(TableName("players")),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("player_id"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("provider_key"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("player_id"), KeyType: types.KeyTypeHash},
		},
		GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
			{
				IndexName:  aws.String("provider-index"),
				KeySchema:  []types.KeySchemaElement{{AttributeName: aws.String("provider_key"), KeyType: types.KeyTypeHash}},
				Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
			},
		},
	})
	if err != nil {
		var inUse *types.ResourceInUseException
		if !errors.As(err, &inUse) {
			log.Printf("ensurePlayersTable: %v", err)
		}
	}
}
