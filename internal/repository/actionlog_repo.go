package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/qq900306ss/badminton-court-api/internal/model"
)

func PutActionLog(ctx context.Context, a model.ActionLog) error {
	item, err := attributevalue.MarshalMap(a)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("action-logs")),
		Item:      item,
	})
	return err
}

// ListActionLogs returns a session's action log, newest first (capped at limit).
func ListActionLogs(ctx context.Context, sessionID string, limit int32) ([]model.ActionLog, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("action-logs")),
		KeyConditionExpression: aws.String("session_id = :sid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid": &types.AttributeValueMemberS{Value: sessionID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var logs []model.ActionLog
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}
