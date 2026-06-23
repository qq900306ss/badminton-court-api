package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/qq900306ss/badminton-court-api/internal/model"
)

func PutSession(ctx context.Context, s model.Session) error {
	item, err := attributevalue.MarshalMap(s)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("sessions")),
		Item:      item,
	})
	return err
}

func GetSession(ctx context.Context, sessionID string) (*model.Session, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName("sessions")),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("session not found")
	}
	var s model.Session
	if err := attributevalue.UnmarshalMap(out.Item, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateSession(ctx context.Context, s model.Session) error {
	return PutSession(ctx, s)
}

// ListOpenSessions returns all sessions that haven't been closed (for the lobby).
// Uses the status GSI (Query) when available; falls back to Scan otherwise.
func ListOpenSessions(ctx context.Context) ([]model.Session, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("sessions")),
		IndexName:              aws.String("status-index"),
		KeyConditionExpression: aws.String("#s = :open"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":open": &types.AttributeValueMemberS{Value: "open"},
		},
	})
	if err != nil {
		return scanOpenSessions(ctx) // GSI not ready yet → fall back
	}
	var sessions []model.Session
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func scanOpenSessions(ctx context.Context) ([]model.Session, error) {
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(TableName("sessions")),
		FilterExpression: aws.String("#s = :open"),
		ExpressionAttributeNames: map[string]string{
			"#s": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":open": &types.AttributeValueMemberS{Value: "open"},
		},
	})
	if err != nil {
		return nil, err
	}
	var sessions []model.Session
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// ListAllSessions returns every session in the system (superadmin view).
func ListAllSessions(ctx context.Context) ([]model.Session, error) {
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(TableName("sessions")),
	})
	if err != nil {
		return nil, err
	}
	var sessions []model.Session
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

// ListSessionsByOrg returns every session created by an org (for "my sessions").
func ListSessionsByOrg(ctx context.Context, orgID string) ([]model.Session, error) {
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(TableName("sessions")),
		FilterExpression: aws.String("org_id = :oid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":oid": &types.AttributeValueMemberS{Value: orgID},
		},
	})
	if err != nil {
		return nil, err
	}
	var sessions []model.Session
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func PutSessionPlayer(ctx context.Context, p model.SessionPlayer) error {
	item, err := attributevalue.MarshalMap(p)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("session-players")),
		Item:      item,
	})
	return err
}

func GetSessionPlayers(ctx context.Context, sessionID string) ([]model.SessionPlayer, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("session-players")),
		KeyConditionExpression: aws.String("session_id = :sid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, err
	}
	var players []model.SessionPlayer
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &players); err != nil {
		return nil, err
	}
	return players, nil
}

func PutGameLog(ctx context.Context, g model.GameLog) error {
	item, err := attributevalue.MarshalMap(g)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("game-logs")),
		Item:      item,
	})
	return err
}

func ListGameLogs(ctx context.Context, sessionID string) ([]model.GameLog, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("game-logs")),
		KeyConditionExpression: aws.String("session_id = :sid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sid": &types.AttributeValueMemberS{Value: sessionID},
		},
		ScanIndexForward: aws.Bool(false), // newest first
	})
	if err != nil {
		return nil, err
	}
	var logs []model.GameLog
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func DeleteSessionPlayer(ctx context.Context, sessionID, playerID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(TableName("session-players")),
		Key: map[string]types.AttributeValue{
			"session_id": &types.AttributeValueMemberS{Value: sessionID},
			"player_id":  &types.AttributeValueMemberS{Value: playerID},
		},
	})
	return err
}
