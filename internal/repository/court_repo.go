package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/qq900306ss/badminton-court-api/internal/model"
)

// PutCourt writes a court with an optimistic lock: the write only commits if the
// stored item's version still matches the one we read (c.Version). On success the
// stored version is bumped to c.Version+1. A brand-new or pre-version (legacy)
// item has no `version` attribute, so the first write is always allowed.
// A version mismatch surfaces as a ConditionalCheckFailedException → IsConflict.
func PutCourt(ctx context.Context, c model.Court) error {
	expected := c.Version
	c.Version = expected + 1 // c is a value copy — caller's struct is untouched
	item, err := attributevalue.MarshalMap(c)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(TableName("courts")),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(version) OR version = :expected"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":expected": &types.AttributeValueMemberN{Value: strconv.Itoa(expected)},
		},
	})
	return err
}

// IsConflict reports whether err is a DynamoDB optimistic-lock conflict
// (another writer committed first), signalling the caller should re-read & retry.
func IsConflict(err error) bool {
	var ccf *types.ConditionalCheckFailedException
	return errors.As(err, &ccf)
}

func GetCourt(ctx context.Context, sessionID, courtID string) (*model.Court, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName("courts")),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
			"court_id":   &types.AttributeValueMemberS{Value: courtID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("court not found")
	}
	var c model.Court
	if err := attributevalue.UnmarshalMap(out.Item, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func GetCourts(ctx context.Context, sessionID string) ([]model.Court, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("courts")),
		KeyConditionExpression: aws.String("session_id = :sid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, err
	}
	var courts []model.Court
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &courts); err != nil {
		return nil, err
	}
	return courts, nil
}

func DeleteCourt(ctx context.Context, sessionID, courtID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(TableName("courts")),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
			"court_id":   &types.AttributeValueMemberS{Value: courtID},
		},
	})
	return err
}
