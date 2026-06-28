package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func extForContentType(ct string) string {
	switch ct {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".jpg"
	}
}

// PresignAvatarUpload returns a short-lived presigned PUT URL for a player to
// upload an avatar directly to S3, plus the public URL to store/display.
func PresignAvatarUpload(ctx context.Context, playerID, contentType string) (uploadURL, publicURL string, err error) {
	bucket := os.Getenv("S3_AVATAR_BUCKET")
	if bucket == "" {
		return "", "", fmt.Errorf("avatar upload not configured")
	}
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "ap-northeast-1"
	}
	key := "avatars/" + playerID + "-" + uuid.New().String() + extForContentType(contentType)

	ps := s3.NewPresignClient(s3Client)
	req, err := ps.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		return "", "", err
	}
	publicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key)
	return req.URL, publicURL, nil
}
