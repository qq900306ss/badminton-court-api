package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/qq900306ss/badminton-court-api/internal/model"
)

func PutPushSub(ctx context.Context, s model.PushSub) error {
	item, err := attributevalue.MarshalMap(s)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("push-subscriptions")),
		Item:      item,
	})
	return err
}

func GetPushSub(ctx context.Context, playerID string) (*model.PushSub, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName("push-subscriptions")),
		Key: map[string]types.AttributeValue{
			"player_id": &types.AttributeValueMemberS{Value: playerID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, nil
	}
	var s model.PushSub
	if err := attributevalue.UnmarshalMap(out.Item, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func DeletePushSub(ctx context.Context, playerID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(TableName("push-subscriptions")),
		Key: map[string]types.AttributeValue{
			"player_id": &types.AttributeValueMemberS{Value: playerID},
		},
	})
	return err
}
