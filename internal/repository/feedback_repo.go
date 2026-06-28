package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/qq900306ss/badminton-court-api/internal/model"
)

func PutFeedback(ctx context.Context, f model.Feedback) error {
	item, err := attributevalue.MarshalMap(f)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("feedback")),
		Item:      item,
	})
	return err
}

// ListFeedback returns all feedback, newest first (capped at limit).
func ListFeedback(ctx context.Context, limit int32) ([]model.Feedback, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("feedback")),
		KeyConditionExpression: aws.String("pk = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: model.FeedbackPK},
		},
		ScanIndexForward: aws.Bool(false), // newest first
		Limit:            aws.Int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var list []model.Feedback
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &list); err != nil {
		return nil, err
	}
	return list, nil
}
