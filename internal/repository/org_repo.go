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

func PutOrg(ctx context.Context, o model.Org) error {
	item, err := attributevalue.MarshalMap(o)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("orgs")),
		Item:      item,
	})
	return err
}

func GetOrgByEmail(ctx context.Context, email string) (*model.Org, error) {
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(TableName("orgs")),
		FilterExpression: aws.String("google_email = :e"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":e": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Items) == 0 {
		return nil, nil
	}
	var o model.Org
	if err := attributevalue.UnmarshalMap(out.Items[0], &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func GetOrg(ctx context.Context, orgID string) (*model.Org, error) {
	out, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(TableName("orgs")),
		Key: map[string]types.AttributeValue{
			"org_id": &types.AttributeValueMemberS{Value: orgID},
		},
	})
	if err != nil {
		return nil, err
	}
	if out.Item == nil {
		return nil, fmt.Errorf("org not found")
	}
	var o model.Org
	if err := attributevalue.UnmarshalMap(out.Item, &o); err != nil {
		return nil, err
	}
	return &o, nil
}

func ListOrgs(ctx context.Context) ([]model.Org, error) {
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(TableName("orgs")),
	})
	if err != nil {
		return nil, err
	}
	var orgs []model.Org
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &orgs); err != nil {
		return nil, err
	}
	return orgs, nil
}

func DeleteOrg(ctx context.Context, orgID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(TableName("orgs")),
		Key: map[string]types.AttributeValue{
			"org_id": &types.AttributeValueMemberS{Value: orgID},
		},
	})
	return err
}

func PutOrgMember(ctx context.Context, m model.OrgMember) error {
	item, err := attributevalue.MarshalMap(m)
	if err != nil {
		return err
	}
	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(TableName("org-members")),
		Item:      item,
	})
	return err
}

func GetOrgMembers(ctx context.Context, orgID string) ([]model.OrgMember, error) {
	out, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(TableName("org-members")),
		KeyConditionExpression: aws.String("org_id = :oid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":oid": &types.AttributeValueMemberS{Value: orgID},
		},
	})
	if err != nil {
		return nil, err
	}
	var members []model.OrgMember
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func DeleteOrgMember(ctx context.Context, orgID, memberID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(TableName("org-members")),
		Key: map[string]types.AttributeValue{
			"org_id":    &types.AttributeValueMemberS{Value: orgID},
			"member_id": &types.AttributeValueMemberS{Value: memberID},
		},
	})
	return err
}
