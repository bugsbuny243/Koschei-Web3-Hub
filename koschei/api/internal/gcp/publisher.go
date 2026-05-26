package gcp

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"
)

type PublisherClient struct {
	service     *androidpublisher.Service
	packageName string
}

func NewPublisherClient(ctx context.Context) (*PublisherClient, error) {
	credsJSON := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS_JSON")
	if credsJSON == "" {
		return nil, errors.New("GOOGLE_APPLICATION_CREDENTIALS_JSON is empty")
	}
	packageName := os.Getenv("ANDROID_PLAY_PACKAGE_NAME")
	if packageName == "" {
		return nil, errors.New("ANDROID_PLAY_PACKAGE_NAME is empty")
	}

	cfg, err := google.JWTConfigFromJSON([]byte(credsJSON), androidpublisher.AndroidpublisherScope)
	if err != nil {
		return nil, fmt.Errorf("parse google credentials: %w", err)
	}

	svc, err := androidpublisher.NewService(ctx, option.WithHTTPClient(cfg.Client(ctx)))
	if err != nil {
		return nil, fmt.Errorf("init android publisher service: %w", err)
	}

	return &PublisherClient{service: svc, packageName: packageName}, nil
}

func (c *PublisherClient) UploadBundleToDraft(bundlePath string) (int64, error) {
	if c == nil || c.service == nil {
		return 0, errors.New("publisher client is not initialized")
	}

	edit, err := c.service.Edits.Insert(c.packageName, &androidpublisher.AppEdit{}).Do()
	if err != nil {
		return 0, fmt.Errorf("create edit session: %w", err)
	}

	file, err := os.Open(bundlePath)
	if err != nil {
		return 0, fmt.Errorf("open bundle: %w", err)
	}
	defer file.Close()

	bundle, err := c.service.Edits.Bundles.Upload(c.packageName, edit.Id).Media(file).Do()
	if err != nil {
		return 0, fmt.Errorf("upload bundle: %w", err)
	}

	if _, err := c.service.Edits.Commit(c.packageName, edit.Id).Do(); err != nil {
		return 0, fmt.Errorf("commit edit session: %w", err)
	}

	return bundle.VersionCode, nil
}
